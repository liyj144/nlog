package push_agent

import (
        "fmt"
        "io"
        "os"
        "log"
        "net"
        "unsafe"
        "time"
        "sync"
        "errors"
        "sync/atomic"
        "encoding/binary"
        "../statuspb_gogo"
        "../pushpb_gogo"
)

//TODO add time.After for token check

var CompileTime string

const WHEEL_COUNT = 128

//debug flag for accept logic
const debugPushServer = true

const MAX_MSG_BODY_LEN = 40960

const DOWNSTREAM_PORT = 8999

var errDownstreamFull = errors.New("downstream channel is full")

//loop type use in read/write looop
const LOOP_DOWNSTREAM = 1
const LOOP_STATUS = 2   //write loop for notify, read loop for force offline

type tcpKeepAliveListener struct {
        *net.TCPListener
}


func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
        tc, err := ln.AcceptTCP()
        if err != nil {
                return
        }
        //fmt.Printf("%+#v\n",tc.RemoteAddr().(*net.TCPAddr).IP[12:])
        tc.SetKeepAlive(true)
        tc.SetKeepAlivePeriod(1 * time.Minute)
        tc.SetReadBuffer(128*1024)
        return tc, nil
}

type rolling_struct struct {
byte_slice []byte

time       int64
}

type frontend_conns struct {
        s                       *Server
        frontend_server_id      uint32
        sendback_chan           chan pushpb_gogo.PushRequestToDownstream
        rwm                     sync.RWMutex
        conns                   []net.Conn
}

func (fc *frontend_conns) writeloopForFrontend() {
        var pb_obj pushpb_gogo.PushRequestToDownstream
        var conn_len int
        var n int
        var err error
        var counter uint
        var conn net.Conn
        var msg_len int
        buf := make([]byte,1024*1024)
        for {
                pb_obj = <- fc.sendback_chan
                fc.rwm.RLock()
                conn_len = len(fc.conns)
                if conn_len == 0 {
                        fc.rwm.RUnlock()
                        fc.s.logf("no alive conn for server id %d",fc.frontend_server_id)
                        continue
                }

                conn = fc.conns[counter % uint(conn_len)]
                conn.(*net.TCPConn).SetWriteDeadline(time.Now().Add(1500*time.Millisecond))
                msg_len,err = pb_obj.MarshalTo(buf[4:])
                if err != nil {
                        fc.s.logf("marshal err in sendback %s",err)
                        continue
                }
                msg_len += 4
                binary.BigEndian.PutUint32(buf[:4],uint32(msg_len))
                n,err = conn.Write(buf[:msg_len])
                fc.rwm.RUnlock()

                if err != nil {
                        fc.s.logf("write error in sendback %s, %d",err,fc.frontend_server_id)
                }
                if n != msg_len {
                        fc.s.logf("shortwrite in sendback %d, %d",n,msg_len)
                }

                counter++
        }
}

type Server struct {
        alive_conn     uint32
        alive_push_conn     uint32
        counter        uint32
        t              Transport
        ClientListenAddr             string 
        FrontendListenAddr           string 
        ErrorLog       *log.Logger    //log.Logger has internal locking to protect concurrent access
        mu             sync.RWMutex
        m              map[uint32]*conn
        frontends     []*frontend_conns


        lazy_now       int64


        //slice_wheel    [128]*rolling_struct
        slice_wheel    [WHEEL_COUNT]unsafe.Pointer
        wheel_counter  uint32

        balance_coun  uint32

        push_fail     uint32
        push_succ     uint32

}

//setup flog facility
func (s *Server) SetupFlog() {

        logFile, err := os.OpenFile("./push_server.log", os.O_WRONLY | os.O_CREATE | os.O_APPEND, 0644)
        if err != nil {
                panic("SetupFlog: error")
        }
        s.ErrorLog = log.New(logFile,"[push_agent]: ",log.LstdFlags)
        logFile, err = os.OpenFile("./push_transport.log", os.O_WRONLY | os.O_CREATE | os.O_APPEND, 0644)
        if err != nil {
                panic("SetupFlog: error")
        }
        s.t.ErrorLog = log.New(logFile,"[transport]: ",log.LstdFlags)
}

//setup nlog facility
func (s *Server) SetupNlog(localaddr string, remoteaddr string) {

        local , err := net.ResolveUDPAddr("udp4",localaddr)
        if err != nil {
                panic("SetupNlog: resolve  localaddr error")
        }

        _ , err = net.ResolveUDPAddr("udp4",remoteaddr)
        if err != nil {
                panic("SetupNlog: resolve  remoteaddr error")
        }

        d := &net.Dialer{LocalAddr: local}

        conn,err := d.Dial("udp",remoteaddr)
        if err!= nil {
                panic("SetupNlog: dial error")
        }

        s.ErrorLog = log.New(conn,"[push_server]: ",log.LstdFlags)

        d.LocalAddr.(*net.UDPAddr).Port += 1
        conn,err = d.Dial("udp",remoteaddr)
        if err!= nil {
                panic("SetupNlog: dial error")
        }
        s.t.ErrorLog = log.New(conn,"[transport]: ",log.LstdFlags)
}


func (s *Server) logf(format string, args ...interface{}) {
	s.ErrorLog.Printf(format, args...)
}

func (s *Server) RegisterStatus(ip_port string) (err error){
        s.t.registerServer(s)
        return s.t.registerStatus(ip_port)
}

func (srv *Server) ListenAndServe() error {
        //TODO: comment out in production env
        go func(){for {fmt.Printf("%d,%d,%d,%d,%d\n",srv.counter,srv.alive_conn,atomic.LoadUint32(&srv.balance_coun),srv.push_succ,srv.push_fail);time.Sleep(20*time.Second)}}()

        // register error log to stderr
        if srv.ErrorLog == nil {
                srv.ErrorLog = log.New(os.Stderr, "[push_server]: ", log.LstdFlags)
        }


        if srv.ClientListenAddr == "" {
                panic("pleace set listen addr and port for client conn")
        }
        srv.logf("listening: %s for client connections",srv.ClientListenAddr)

        if srv.FrontendListenAddr == "" {
                //addr = ":8999"    //Default port 8999
                panic("pleace set frontend addr and port for client conn")
        }
        srv.logf("listening: %s for client connections",srv.FrontendListenAddr)

        if srv.m == nil {
                srv.m = make(map[uint32]*conn,128)
        }

        frontend_ln, err := net.Listen("tcp", srv.FrontendListenAddr)

        if err != nil {
                panic(err)
        }
        
        cli_ln, err := net.Listen("tcp", srv.ClientListenAddr)

        if err != nil {
                panic(err)
        }

        go srv.AcceptFrontend(tcpKeepAliveListener{frontend_ln.(*net.TCPListener)})

        return srv.AcceptClient(tcpKeepAliveListener{cli_ln.(*net.TCPListener)})
}

func (srv *Server) AcceptFrontend(l net.Listener) error {
        var tempDelay time.Duration // how long to sleep on accept failure
        alive := true


        for alive {
                rw, e := l.Accept()

                if debugPushServer {
                        srv.logf("Accept frontend from %s",rw.RemoteAddr().String())
                }

                if e != nil {
                        if ne, ok := e.(net.Error); ok && ne.Temporary() {
                                if tempDelay == 0 {
                                        tempDelay = 5 * time.Millisecond
                                } else {
                                        tempDelay *= 2
                                }
                                if max := 1 * time.Second; tempDelay > max {
                                        tempDelay = max
                                }
                                srv.logf("push_server: Accept error: %v; retrying in %v", e, tempDelay)
                                time.Sleep(tempDelay)
                                continue
                        }
                        return e
                }
                tempDelay = 0

                //add r/w logic
                go srv.readAndRegisterFrontend(rw)
        }
        l.Close()
        return nil

}

func (srv *Server) readAndRegisterFrontend(c net.Conn) {
        //tune write buffer
        c.(*net.TCPConn).SetWriteBuffer(128*1024)

        server_id_buf := make([]byte,4)
        var err error
        var sid uint32
        var found bool
        var found_conn bool

        //register mapping between server_id and net_id
        c.(*net.TCPConn).SetReadDeadline(time.Now().Add(5500*time.Millisecond))
        _ , err = io.ReadFull(c,server_id_buf)
        if err == nil {
                sid = binary.BigEndian.Uint32(server_id_buf)
                srv.logf("decode sid %d from %s",sid,c.RemoteAddr().String())

        } else {
                srv.logf("read server id error" + err.Error())
                c.Close()
                return

        }
        c.(*net.TCPConn).SetReadDeadline(noDeadline)

        //register
        srv.mu.Lock()
        for _,e := range srv.frontends {
                if e.frontend_server_id == sid {
                        found = true
                        e.conns = append(e.conns,c)
                        srv.logf("append item for sid %d item %s",sid,c.RemoteAddr().String())
                        break

                }
        }

        if !found {
                //add item

                srv.logf("add item for sid item %d, %s",sid,c.RemoteAddr().String())

                temp_fc := &frontend_conns {
                s:srv,
                sendback_chan:make(chan pushpb_gogo.PushRequestToDownstream,100),
                frontend_server_id:sid,}

                temp_fc.conns = append(temp_fc.conns,c)

                srv.frontends = append(srv.frontends,temp_fc)
                
                go temp_fc.writeloopForFrontend()
        }
        srv.mu.Unlock()

        //read until eof

        for err == nil{
                _ , err = io.ReadFull(c,server_id_buf)
                srv.logf("readloop: read garbage from downstream:%v,%s",server_id_buf,err)
        }


        found = false
        srv.mu.Lock()
        for _,e := range srv.frontends {
                if e.frontend_server_id == sid {
                        found = true
                        for i,tc := range e.conns {
                                if tc == c {
                                        found_conn = true
                                        srv.logf("closing conn del item %s",c.RemoteAddr().String())
                                        //delete item
                                        e.conns[i], e.conns[len(e.conns)-1], e.conns =
                                        e.conns[len(e.conns)-1], nil, e.conns[:len(e.conns)-1]
                                        break
                                }
                        }
                        break

                }
        }
        srv.mu.Unlock()
        c.Close()

        if found_conn == false {
                srv.logf("closing conn not found item %s",c.RemoteAddr().String())
        }
        if found == false {
                srv.logf("closing conn not found sid %d",sid)
        }

}

func (srv *Server) AcceptClient(l net.Listener) error {
        var tempDelay time.Duration // how long to sleep on accept failure
        alive := true


        for alive {
                rw, e := l.Accept()

                if debugPushServer {
                        srv.logf("Accept from %s, counter: %d",rw.RemoteAddr().String(),srv.counter)
                }

                if e != nil {
                        if ne, ok := e.(net.Error); ok && ne.Temporary() {
                                if tempDelay == 0 {
                                        tempDelay = 5 * time.Millisecond
                                } else {
                                        tempDelay *= 2
                                }
                                if max := 1 * time.Second; tempDelay > max {
                                        tempDelay = max
                                }
                                srv.logf("push_server: Accept error: %v; retrying in %v", e, tempDelay)
                                time.Sleep(tempDelay)
                                continue
                        }
                        return e
                }
                tempDelay = 0

                //increment alive connection counter
                atomic.AddUint32(&srv.alive_conn,1)

                conn_id := atomic.AddUint32(&srv.counter,1)

                c := srv.newConn(rw,conn_id)
                go c.serve()
        }
        l.Close()
        return nil
}

func (srv *Server) sendDownstreamRequest(sid uint32,conn_id uint32,body []byte) (err error) {
        var found bool
        srv.mu.RLock()
        for _,e := range srv.frontends {
                if e.frontend_server_id == sid {
                        found = true
                        if len(e.sendback_chan) < cap(e.sendback_chan) - 4 {
                                e.sendback_chan <- pushpb_gogo.PushRequestToDownstream {
                                NetId :   conn_id,
                                Payload:     body,
                                }
                        } else {
                                return errDownstreamFull
                        }
                        break

                }
        }
        srv.mu.RUnlock()
        if !found {
                return fmt.Errorf("not found frontend for sid %d",sid)
        }
        return
}


func (srv *Server) newConn(rwc net.Conn,conn_id uint32) (c *conn) {
        c = new(conn)
        c.remoteAddr = rwc.RemoteAddr().String()
        c.server = srv
        c.conn_id = conn_id
        c.rwc = rwc
        c.now_msec = time.Now().UnixNano()/1e6
        srv.lazy_now = c.now_msec

        //add map entry
        srv.mu.Lock()
        srv.m[conn_id] = c
        srv.mu.Unlock()

        return c
}

type conn struct {
        conn_id    uint32               // connection id
        now_msec   int64
        remoteAddr string               // network address of remote side
        server     *Server              // the Server on which the connection arrived
        rwc        net.Conn             // i/o connection

}



//when receive force offline we did not need to send offline notification to upstream server
// Close the connection.
func (c *conn) close() {
        if debugPushServer {
                c.server.logf("closing: %v, conn_id: %d\n",c.remoteAddr,int(c.conn_id))
        }

        //delete map entry
        c.server.mu.Lock()
        delete(c.server.m,c.conn_id)
        c.server.mu.Unlock()

 
        //decrement counter
        atomic.AddUint32(&c.server.alive_conn, ^uint32(0))

        //c.finalFlush()
        if c.rwc != nil {
                c.rwc.Close()
                c.rwc = nil
        }
}

func (c *conn) serve() {
        defer func() {
                if err := recover(); err != nil {
                        c.server.logf("panic serving %v: %v\n", c.remoteAddr, err)
                }
                c.close()
        }()
        
        var rbuf []byte
        //TODO review max body size
        rbuf = make([]byte,4096*1024)
        msg_len_buf := []byte{0,0,0,0}
        alive := true

        for alive {
                rc,err := io.ReadFull(c.rwc,msg_len_buf)
                if err != nil {
                        c.server.logf("msg_len read error %v,conn_id %d: %v, rc: %d\n", c.remoteAddr, c.conn_id, err, rc)
                        break
                }
                msg_len := binary.BigEndian.Uint32(msg_len_buf)
                rc,err = io.ReadFull(c.rwc,rbuf[:msg_len-4])
                if debugPushServer {
                        c.server.logf("read msg_len: %d, msg_body: %v,rc: %d, from: %v,conn_id: %d\n",
                                        msg_len,
                                        rbuf[:msg_len-4],
                                        rc,
                                        c.remoteAddr,c.conn_id)
                }
                if err != nil {
                        c.server.logf("msg_body length error from %v,conn_id %d: %v %d\n", c.remoteAddr,c.conn_id,err, rc)
                        break
                }

                //process message
                var preq pushpb_gogo.PushRequestFromUpstream

                preq.Reset()
                err = preq.Unmarshal(rbuf[:msg_len-4])
                if err != nil {
                        c.server.logf("unmarshal error %v", err)
                        break
                }

                if debugPushServer {
                        c.server.logf("unmarshal result %s",
                                        preq.String())
                }

                msg_count := atomic.AddUint32(&c.server.wheel_counter,1)

                atomic.AddUint32(&c.server.balance_coun,1)

                tindex := msg_count%WHEEL_COUNT

                        rs := new(rolling_struct)

                        rs.byte_slice = preq.Payload

                        rs.time = time.Now().UnixNano()/1e6
                        //check for edge case
                        trs := (*rolling_struct)(atomic.LoadPointer(&c.server.slice_wheel[tindex])) 
                        if trs != nil {
                                c.server.logf("wheel conflict delta %d ms %d",rs.time-trs.time,tindex)
                        }

                        atomic.StorePointer(&c.server.slice_wheel[tindex],unsafe.Pointer(rs))
                        if debugPushServer {
                                c.server.logf("register wheel %d",
                                        tindex)
                        }

                //send status request and wait for response
                if preq.GetFieldType() == pushpb_gogo.PushRequestFromUpstream_UID {
                        err = c.server.t.sendStatusRequest(preq.GetUserId(),nil,tindex,statuspb_gogo.StatusRequest_UID)
                } else {
                        //use device id here
                        err = c.server.t.sendStatusRequest(0,preq.GetDeviceId(),tindex,statuspb_gogo.StatusRequest_DEVICEID)
                }


                if err != nil {
                        c.server.logf("error in main loop %s ,deregister %d",err,tindex)
                        trs.byte_slice = nil
                        atomic.StorePointer(&c.server.slice_wheel[tindex],nil)                        
                }
        }
}




//check for nil uid in main loop
func (srv *Server) checkStatusResume(resume_conn_id uint32,query_result []*statuspb_gogo.StatusQueryResult) error {
        atomic.AddUint32(&srv.balance_coun, ^uint32(0))


        var rs  *rolling_struct
        var err error

        if query_result != nil {

                atomic.AddUint32(&srv.push_succ, 1)

                for _,s := range query_result {
                        if debugPushServer {
                                srv.logf("query result %s",
                                                s.String())
                        }
        
                        rs = (*rolling_struct)(atomic.LoadPointer(&srv.slice_wheel[resume_conn_id]))
                        if rs == nil {
                                srv.logf("rs is niiiiiiiiiiiiil %d",resume_conn_id)
                        }
                        if rs.byte_slice == nil {
                                srv.logf("rs.bs is niiiiiiiiiiiiil %d",resume_conn_id)
                        }
                        err = srv.sendDownstreamRequest(s.Ip,s.RemoteNetId,rs.byte_slice)


                        if debugPushServer {
                                srv.logf("sendback result %s time %d index %d",
                                                string(rs.byte_slice),rs.time,resume_conn_id)
                        }

                        if err != nil {
                                srv.logf("send downstream error %s",err)
                                break
                        }
                }

                        rs.byte_slice = nil
                        atomic.StorePointer((&srv.slice_wheel[resume_conn_id]),nil)                        
                        if debugPushServer {
                                srv.logf("deregister wheel %d",resume_conn_id)
                        }

        } else {

                atomic.AddUint32(&srv.push_fail, 1)

                if debugPushServer {
                        srv.logf("query result is nil, deregister wheel %d",resume_conn_id)
                }
                rs = (*rolling_struct)(atomic.LoadPointer((*unsafe.Pointer)(&srv.slice_wheel[resume_conn_id])))
                rs.byte_slice = nil
                atomic.StorePointer((*unsafe.Pointer)(&srv.slice_wheel[resume_conn_id]),nil)
        }

        return err

}

