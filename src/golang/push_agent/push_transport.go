package push_agent

import (
        "bufio"
        "io"
        "os"
        "net"
        "fmt"
        "sync/atomic"
        "time"
        "log"
        "encoding/binary"
        "../statuspb_gogo"
        "github.com/gogo/protobuf/proto"
)

var noDeadline = time.Time{}

const debugTransport = true

const cmdUpstreamConnsNum = 2

const MAX_BACKOFF_TIME = 60

var default_dialer = net.Dialer{
                Timeout:   8 * time.Second,
                KeepAlive: 30 * time.Second,
                }

type Transport struct {
        msg_counter     uint64
        s               *Server
        ErrorLog        *log.Logger    //log.Logger has internal locking to protect concurrent access
        status_conns    *cmdConnections

}

func (t *Transport) logf(format string, args ...interface{}) {
        t.ErrorLog.Printf(format, args...)
}

//should call before registerCmd
func (t *Transport) registerServer(s *Server) {
        if t.s == nil {
                t.s = s
        }
}


func (t *Transport) registerStatus(ip_port string) (err error){
        if err != nil{
                panic("error in RegisterCmd: resolve error" + ip_port)
        }

        //setup log facility
        if t.ErrorLog == nil {
                t.ErrorLog = log.New(os.Stderr, "[transport]: ", log.LstdFlags)
        }


        
        t.status_conns = &cmdConnections{
                cmd:           LOOP_STATUS,
                ip_port:       ip_port,
                t:             t,
                status_writech:       make(chan statuspb_gogo.StatusRequest,300),
                }

        for i := 0;i < cmdUpstreamConnsNum;i++{
                dc,err := default_dialer.Dial("tcp",ip_port)
                if err == nil{
                        t.status_conns.conns = append(t.status_conns.conns,t.status_conns.newConn(dc))
                } else {
                        panic(fmt.Sprintf("dial conn error for status, ip_port: %s",ip_port))
                }
        }
        
        return nil
}

// send token check request to upstream
func (t *Transport) sendStatusRequest(uid uint64,
                                        dev_id []byte,
                                        resume_net_id uint32,
                                        field_type statuspb_gogo.StatusRequest_FieldType) (err error){
        
        if len(t.status_conns.status_writech) >= cap(t.status_conns.status_writech) {
                return fmt.Errorf("write channel for status_request is full")
        }

        if field_type == statuspb_gogo.StatusRequest_UID {

                t.status_conns.status_writech <- statuspb_gogo.StatusRequest {
                        ReqType : statuspb_gogo.StatusRequest_GET,
                        FieldType: field_type,
                        Uid  :  proto.Uint64(uid),
                        ResumeNetId  :  proto.Uint32(resume_net_id),
                } 

        } else {

                t.status_conns.status_writech <- statuspb_gogo.StatusRequest {
                        ReqType : statuspb_gogo.StatusRequest_GET,
                        FieldType: field_type,
                        DeviceId:proto.String(string(dev_id)),
                        ResumeNetId  :  proto.Uint32(resume_net_id),
                } 

        }


        return nil;
}



type cmdConnections struct {
        cmd         uint16
        ip_port     string
        t           *Transport
        status_writech     chan statuspb_gogo.StatusRequest   // written by roundTrip; read by writeLoop
        alive_conns uint32
        conns       []*persistConn
}



type noteEOFReader struct {
        r          io.Reader
        alive_conn *uint32
        notify_eof chan struct{}         //channel for readloop no notify write loop, no buffer here
}

func (nr noteEOFReader) Read(p []byte) (n int, err error) {
        n, err = nr.r.Read(p)
        //TODO need review
        //if err == io.EOF {
        if err != nil {
                nr.notify_eof <- struct{}{}
                atomic.AddUint32(nr.alive_conn,^uint32(0))
        }
        return
}




func (cc *cmdConnections) newConn(conn net.Conn) (*persistConn) {

        atomic.AddUint32(&cc.alive_conns,1)

        pconn := &persistConn{
                cmdConns:   cc,
                conn:       conn,
                notify_eof: make(chan struct{}),
                exp_backoff_time: 2,
        }
        pconn.br = bufio.NewReader(noteEOFReader{pconn.conn, &cc.alive_conns,pconn.notify_eof})
        pconn.bw = bufio.NewWriter(pconn.conn)

        pconn.spawnReadLoop()
        pconn.spawnWriteLoop()

        return pconn
}


type persistConn struct {
        cmdConns        *cmdConnections
        conn     net.Conn
        br       *bufio.Reader       // from conn
        bw       *bufio.Writer       // to conn
        notify_eof chan struct{}         //channel for readloop no notify write loop, no buffer here

        exp_backoff_time  uint8
}


func (pc *persistConn) close() {
        pc.cmdConns.t.logf("close %v",pc)
        pc.conn.Close()
}


func (pc *persistConn) writeLoopForStatus() {

        writeCounter := 0
        var err error
        var status_request statuspb_gogo.StatusRequest
        //send sid to other status server uint32(-1)
        sid_buf := []byte{0,0,0,0}
        binary.BigEndian.PutUint32(sid_buf,^uint32(0))
        pc.cmdConns.t.logf("send server id %v",sid_buf)
        _,err = pc.conn.Write(sid_buf)
        if err != nil {
                pc.cmdConns.t.logf("err in send server id" + err.Error())
        }

        pc.conn.(*net.TCPConn).SetNoDelay(true)
        sta_buf := make([]byte,512)
        for {

                //wait for request
                select {
                        case   <- pc.notify_eof:
                                pc.cmdConns.t.logf("get notify eof signal %d",pc.cmdConns.cmd)
                                pc.bw.Flush()
                                return
                        case status_request = <-pc.cmdConns.status_writech:
                }

                writeCounter += 1
                msg_len,err := status_request.MarshalTo(sta_buf[4:])
                if err != nil {
                        pc.cmdConns.t.logf("pb marshal error %v",err)
                        continue
                }

                if debugTransport {
                        pc.cmdConns.t.logf("write request to %d,%s upstream: %s, err: %v\n",pc.cmdConns.cmd,pc.cmdConns.ip_port,status_request.String(),err)
                }

                msg_len += 4
                binary.BigEndian.PutUint32(sta_buf[:4],uint32(msg_len))
                _,err = pc.bw.Write(sta_buf[:msg_len])


                if err == nil && writeCounter >= 0  {
                        err = pc.bw.Flush()
                }
                if err != nil {
                        if cap(pc.cmdConns.status_writech) > len(pc.cmdConns.status_writech) + 10 {
                                pc.cmdConns.status_writech <- status_request
                        }
                        //should not return here
                        //return
                }


        } //end of case LOOP_STATUS

}

func (pc *persistConn) spawnWriteLoop() {
        switch pc.cmdConns.cmd {

                case LOOP_STATUS:
                        go pc.writeLoopForStatus()

        }       //end of switch
}




func (pc *persistConn) readLoopForStatus() {

        alive := true
        var status_query_response statuspb_gogo.StatusQueryResponse
        var msg_len uint32
        //TODO review here,we use
        body_buf    := make([]byte,4096) 
        pc.conn.(*net.TCPConn).SetReadBuffer(64*1024)

        for alive {
                //read request from upstream
                //err := rtr.read(pc.br,body_buf)
                //fmt.Println(rtr)

                _ , err := io.ReadFull(pc.br,body_buf[:4])
                if err == nil {
                        msg_len =  binary.BigEndian.Uint32(body_buf[:4])
                        _ , err = io.ReadFull(pc.br,body_buf[4:msg_len])
                }

                if err == nil  {
                        status_query_response.Reset()
                        err = status_query_response.Unmarshal(body_buf[4:msg_len])
                        if err != nil {
                                pc.cmdConns.t.logf("readloop: unmarshal error:%v",err)
                        } else {
                                        if debugTransport {
                                        pc.cmdConns.t.logf("read request from %d,%s upstream: %s, err: %v\n",
                                                                pc.cmdConns.cmd,
                                                                pc.cmdConns.ip_port,
                                                                status_query_response.String(),err)
                                        }


                                //TODO
                                //send back push message here, use resume
                                //if status_query_response.ResumeNetId == nil {
                                //        pc.cmdConns.t.logf("read nil resume net id from status server")
                                //} else {
                                        //TODO no need to send back result when not found
                                        err = pc.cmdConns.t.s.checkStatusResume(status_query_response.ResumeNetId,
                                                                                status_query_response.Result)

                                        if err != nil {
                                                pc.cmdConns.t.logf("check status resume error %s",err)
                                        }
                                //}
                        }
                } else if err == io.EOF {
                        alive = false
                } else if err == io.ErrUnexpectedEOF {
                        alive = false
                } else {
                        //alive = false
                        //panic(fmt.Sprintf("Unknown error in readloop: %v",err))
                        pc.cmdConns.t.logf("Unknowned read error from %s: %v",pc.cmdConns.ip_port,err)
                }



        }

        pc.close()
        pc.redialConn()

}

func (pc *persistConn) spawnReadLoop() {
        switch pc.cmdConns.cmd {

                case LOOP_STATUS:
                        go pc.readLoopForStatus()

                 //end of case LOOP_STATUS
        } //end of switch
}

func (pc *persistConn) redialConn() (err error) {
        conn,err := default_dialer.Dial("tcp",pc.cmdConns.ip_port)
        for err != nil{
                time.Sleep( time.Duration(pc.exp_backoff_time) * time.Second)
                if pc.exp_backoff_time <= MAX_BACKOFF_TIME {
                        pc.exp_backoff_time *= 2
                }
                pc.cmdConns.t.logf("redial %s  error: %s",pc.cmdConns.ip_port,err)
                conn,err = default_dialer.Dial("tcp",pc.cmdConns.ip_port)
        }
        pc.conn = conn
        pc.br.Reset(noteEOFReader{conn,&pc.cmdConns.alive_conns,pc.notify_eof})
        pc.bw.Reset(conn)
        //re spawn read/write loop
        pc.spawnReadLoop()
        pc.spawnWriteLoop()
        pc.cmdConns.t.logf("respawn %d %s",pc.cmdConns.cmd,pc.cmdConns.ip_port)
        return nil
}
