package tcp_agent

import (
        "fmt"
        "io"
        "os"
        "log"
        "net"
        "time"
        "sync"
        "strconv"
        "runtime"
        "runtime/debug"
        "sync/atomic"
        "encoding/binary"
        "../statuspb_gogo"
        "../upstreampb_gogo"
        "../metadatapb_gogo"
)

var CompileTime string

//TODO add time.After for token check

//debug flag for accept logic
const debugTcpServer = true

const MAX_MSG_BODY_LEN = 1024 * 1024

const MAX_PARSE_ERROR = 3

//loop type use in read/write looop
const LOOP_TOKEN_REST = 0
const LOOP_ECHO = 1
const LOOP_STATUS = 2   //write loop for notify, read loop for force offline
const LOOP_PUSH = 3   //write loop for notify, read loop for force offline

var global4kPool = sync.Pool{
        New: func() interface{} { return make([]byte,4096) },
}


var global8kPool = sync.Pool{
        New: func() interface{} { return make([]byte,4096 * 2) },
}


var global16kPool = sync.Pool{
        New: func() interface{} { return make([]byte,4096 * 4) },
}

type tcpKeepAliveListener struct {
        *net.TCPListener
}

func roundUp(a, b int) int {
        return a - a % b + b
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
        tc, err := ln.AcceptTCP()
        if err != nil {
                return
        }
        tc.SetKeepAlive(true)
        tc.SetKeepAlivePeriod(1 * time.Minute)
        tc.SetKeepAliveCount(2)
        return tc, nil
}

type Server struct {
        //counter
        sendback_coun  uint64
        sendback_fail  uint64
        sendback_write_fail  uint64

        total_msg_ok_cnt  uint64
        total_msg_err_cnt  uint64

        handshake_sucess_count  uint64
        handshake_sucess_count2  uint64
        handshake_fail_count  uint64

        token_fail_count      uint32
        token_timeout_count      uint32
        token_hit_count      uint32
        
        alive_conn     uint32
        alive_push_conn     uint32

        current_general_user uint32
        current_anonymous_user uint32

        size_lt_400b        uint32
        size_lt_900b        uint32
        size_lt_4k          uint32
        size_lt_8k          uint32
        size_lt_16k         uint32
        size_gt_16k         uint32

        sendback_size_lt_400b        uint32
        sendback_size_lt_900b        uint32
        sendback_size_lt_4k          uint32
        sendback_size_lt_8k          uint32
        sendback_size_lt_16k         uint32
        sendback_size_gt_16k         uint32

        counter        uint32

        t              Transport
        ClientListenAddr           string 
        ServerId                   uint32
        ErrorLog       *log.Logger    //log.Logger has internal locking to protect concurrent access
        mu             sync.RWMutex
        m              map[uint32]*conn

        tc             *TokenCache    //token cache object

        lazy_now       int64

        //ip_mu          sync.RWMutex

        startup_time   int64
}

func (s *Server) reportMetadata() {

        conn,err := net.Dial("udp","127.0.0.1:8991")
        if err!= nil {
                panic("report metadata: dial error")
        }

        buf := make([]byte,1024)

        var pb_obj  metadatapb_gogo.MetadataFromFrontend
        var memstat runtime.MemStats
        var gcstat debug.GCStats 
        var count int

        pb_obj.CompileTime = CompileTime

        for {
                <- time.After(60*time.Second)
                count++
                if count % 4 == 0 {
                        pb_obj.GoroutineNum = int32(runtime.NumGoroutine())

                        pb_obj.TcLen = int32(s.tc.Len())

                        pb_obj.StartTime = int64(s.startup_time)

                        runtime.ReadMemStats(&memstat)
                        pb_obj.HAlloc = int64(memstat.HeapAlloc)
                        pb_obj.HSys = int64(memstat.HeapSys)
                        pb_obj.HIdle = int64(memstat.HeapIdle)
                        pb_obj.HInuse = int64(memstat.HeapInuse)
                        pb_obj.HObjects = int64(memstat.HeapObjects)
                        pb_obj.HRelease = int64(memstat.HeapReleased)

                        debug.ReadGCStats(&gcstat)
                        pb_obj.GcLast = gcstat.LastGC.Unix()
                        pb_obj.GcNum = gcstat.NumGC
                        pb_obj.GcLastPause = gcstat.Pause[0].Nanoseconds()
                }

                pb_obj.CurrentTime = int64(s.lazy_now)

                pb_obj.SRollCounter = int64(s.counter)
                pb_obj.SAliveConn = int64(s.alive_conn)
                pb_obj.SCurrentGeneral = int64(s.current_general_user)
                pb_obj.SCurrentAnonymous = int64(s.current_anonymous_user)

                pb_obj.STokenOk = int64(s.token_hit_count)
                pb_obj.STokenFail = int64(s.token_fail_count)
                pb_obj.STokenTimeout = int64(s.token_timeout_count)

                pb_obj.SSbCount = int64(s.sendback_coun)
                pb_obj.SSbFail = int64(s.sendback_fail)
                pb_obj.SSbWriteFail = int64(s.sendback_write_fail)
                
                pb_obj.STmOk = int64(s.total_msg_ok_cnt)
                pb_obj.STmErr = int64(s.total_msg_err_cnt)

                if debugTcpServer {
                //        s.logf("metadata result %s",pb_obj.String())
                }

                n,err := pb_obj.MarshalTo(buf)
                if err == nil {
                        conn.Write(buf[:n])
                } else {
                        s.logf("marshal metadata error %s",err)
                }

        }
}


func (s *Server) SetupFlog() {
        fmt.Println("Setup Flog")

        logFile, err := os.OpenFile("./server.log", os.O_WRONLY | os.O_CREATE | os.O_APPEND, 0644)
        if err != nil {
                panic("SetupFlog: error")
        }
        s.ErrorLog = log.New(logFile,"[tcp_agent]: ",log.LstdFlags|log.Lmicroseconds)
        logFile, err = os.OpenFile("./transport.log", os.O_WRONLY | os.O_CREATE | os.O_APPEND, 0644)
        if err != nil {
                panic("SetupFlog: error")
        }
        s.t.ErrorLog = log.New(logFile,"[transport]: ",log.LstdFlags|log.Lmicroseconds)
}


//setup nlog facility
func (s *Server) SetupNlog(localaddr string, remoteaddr string) {
        fmt.Println("Setup Nlog")

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

        s.ErrorLog = log.New(conn,"[tcp_agent]: ",log.LstdFlags|log.Lmicroseconds)

        d.LocalAddr.(*net.UDPAddr).Port += 1
        conn,err = d.Dial("udp",remoteaddr)
        if err!= nil {
                panic("SetupNlog: dial error")
        }
        s.t.ErrorLog = log.New(conn,"[transport]: ",log.LstdFlags|log.Lmicroseconds)
}


func (s *Server) logf(format string, args ...interface{}) {
	s.ErrorLog.Printf(format, args...)
}

func (s *Server) RegisterPush(ip_port string) (err error){
        s.t.registerServer(s)
        return s.t.registerPush(ip_port)
}


func (s *Server) RegisterStatusMaster(ip_port string) (err error){
        s.t.registerServer(s)
        return s.t.registerStatus(ip_port,false)
}

func (s *Server) RegisterStatusSlave(ip_port string) (err error){
        s.t.registerServer(s)
        return s.t.registerStatus(ip_port,true)
}

func (s *Server) RegisterRestagent(ip_port string) (err error){
        s.t.registerServer(s)
        return s.t.registerRestagent(ip_port)
}

func (s *Server) RegisterCmd(cmd_low uint16,cmd_high uint16,ip_port string) (err error){
        s.t.registerServer(s)
        return s.t.registerCmd(cmd_low,cmd_high,ip_port)
}

func (srv *Server) ListenAndServe() error {
        srv.startup_time = time.Now().Unix()

        //pre caculate
        private_key_1.Precompute()

        go srv.reportMetadata()

        //TODO: comment out in production env
        go func(){for {fmt.Printf("%d,%d,%d,%d,%d\n",srv.counter,srv.alive_conn,srv.sendback_coun,len(srv.m),runtime.NumGoroutine());time.Sleep(20*time.Second)}}()

        // register error log to stderr
        if srv.ErrorLog == nil {
                srv.ErrorLog = log.New(os.Stderr, "[tcp_server]: ", log.LstdFlags|log.Lmicroseconds)
        }

        // initialize tokencache object
        if srv.tc == nil {
                srv.tc = NewTokenCache(srv)
        }

        if srv.ClientListenAddr == "" {
                panic("pleace set listen addr and port for client conn")
        }
        srv.logf("listening: %s for client connections",srv.ClientListenAddr)


        if srv.m == nil {
                srv.m = make(map[uint32]*conn)
        }


        cli_ln, err := net.Listen("tcp", srv.ClientListenAddr)

        if err != nil {
                panic(err)
        }
        return srv.AcceptClient(tcpKeepAliveListener{cli_ln.(*net.TCPListener)})
}


func (srv *Server) AcceptClient(l net.Listener) error {
        var tempDelay time.Duration // how long to sleep on accept failure
        alive := true

        //go func(){time.Sleep(300*time.Second);alive = false}()

        for alive {
                rw, e := l.Accept()

                if debugTcpServer {
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
                                srv.logf("accept: Accept error: %s; retrying in %v", e, tempDelay)
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

// send upstream tokencheck request and wait for result
func (srv *Server) checkToken(conn_id uint32, token string, uid string,uid_int uint64, time_now int64,trr <-chan string) bool {
        //MOCK
        //return true
        //check for cache hit
        var rc string
        if rc,ok := srv.tc.Get(token,time_now);ok {
                if uid_int == rc {
                        atomic.AddUint32(&srv.token_hit_count,1)
                        if debugTcpServer {
                                srv.logf("checkToken: token cache hit for %s,%s",token,uid)
                        }
                        return true
                } else {
                        if debugTcpServer {
                                srv.logf("checkToken: token cache hit but not equal for %s,%s",token,uid)
                        }
                        //no return here
                        //return false
                }               
        }

        if debugTcpServer {
                srv.logf("checkToken: token cache not hit  for %s,%s",token,uid)
        }

        // send upstream
        //TODO check for upstream not found error here
        //MOCK
        //err := srv.t.sendTokenCheckRequest(conn_id,[]byte(token),uid)
        err := srv.t.sendTokenCheckRequest(conn_id,token,uid)
        
        if err != nil {
                srv.logf("checkToken: %s",err)
                return false
        }

        //TODO add time out here
        //get result
        select {
        case rc = <-trr:
        case <- time.After(80*time.Second):
                atomic.AddUint32(&srv.token_timeout_count,1)
                srv.logf("checkToken: timeout for uid %s",uid)
                return false
        }

        if rc == "" {
                if debugTcpServer {
                        srv.logf("checkToken: restagent token check not exist for for %s,%s",token,uid)
                }
                return false
        } else if uid == rc {
                //add or overwrite old result here
                srv.tc.Add(token,uid_int) 
                if debugTcpServer {
                        srv.logf("checkToken: restagent token check success for for %s,%s",token,uid)
                }
                return true

        } else {
                srv.tc.Add(token,uid_int) 
                if debugTcpServer {
                        srv.logf("checkToken: restagent token check not equal for for %s,%s",token,uid)
                }
                return false
        }

}

func (srv *Server) statusResponseResume(conn_id uint32,resptype statuspb_gogo.StatusResponse_Type) error {
        if resptype == statuspb_gogo.StatusResponse_FORCE_OFFLINE {
                
                srv.mu.RLock()
                if c,exist := srv.m[conn_id];exist {
                        //no need to send offline signal to upstream
                        //need review
	        	srv.mu.RUnlock()
                        err := c.forceOffline()
                        if err != nil {
                                c.server.logf("statusResponseResume:  force offline error %v",err)
                        }

                        if debugTcpServer {
                                c.server.logf("statusResponseResume:  %s:%d \n",c.remoteAddr,conn_id)
                        }

                        return nil
                } else {
	        	srv.mu.RUnlock()
                        return fmt.Errorf("entry for %d not exist",conn_id)
                }
        }

        return nil
}

//check for nil uid in main loop
func (srv *Server) checkTokenResume(conn_id uint32,uid string) error {

        srv.mu.RLock()
        if c,exist := srv.m[conn_id];exist {

		//srv.mu.RUnlock()
                if len(c.token_resume_result) >= cap(c.token_resume_result) {
                        //will hang here
                        c.server.logf("checkTokenResume get one result already full %d",conn_id)
                } else {
                        c.token_resume_result <- uid
                }

		srv.mu.RUnlock()

                if debugTcpServer {
                        c.server.logf("checkTokenResume:  %s:%s \n",c.remoteAddr,string(uid))
                }

                return nil
        } else {
		srv.mu.RUnlock()
                return fmt.Errorf("entry for %d not exist",conn_id)
        }
}

//we use first 4 bytes of "body []byte" to encoding msg_len, don't overwrite!
func (srv *Server) sendback(conn_id uint32,msg_id uint32,body []byte,is_push bool) (err error) {

        var local_buf [900]byte
        var temp_msg_len int
        var assembly_buffer []byte
        var written int

        srv.mu.RLock()
        if c,exist := srv.m[conn_id];exist {
                atomic.AddUint64(&srv.sendback_coun,1)
		srv.mu.RUnlock()

                var cm channelMessage

                cm.m_crypt_flag = S_ENC_AES_DYNAMIC
                //push or normal sendback
                if is_push == false {
                        cm.m_req_id = msg_id
                        //INFO
                        c.server.logf("back %d:%d:%d",c.handshake_message.uid,conn_id,msg_id)
                } else {
                        cm.m_req_id = atomic.AddUint32(&c.push_id,2)
                        //INFO
                        c.server.logf("push %s:%d:%d",c.handshake_message.m_anonymous_deviceid,conn_id,msg_id)
                }
                cm.aes_dynamic_key = c.handshake_message.aes_dynamic_key

                if debugTcpServer {
                        if len(body) > 132 {
                        c.server.logf("sendback to %s:%d:%d:%s \n",c.remoteAddr,conn_id,cm.m_req_id,string(body[:132]))
                        } else {
                        c.server.logf("sendback to %s:%d:%d:%s \n",c.remoteAddr,conn_id,cm.m_req_id,string(body))
                        

                        }
                }


                //get buf
                temp_msg_len = 16 + 4 + roundUp(len(body),AES_BLOCK_SIZE)

                if temp_msg_len <= 400 {

                        atomic.AddUint32(&c.server.sendback_size_lt_400b,1)

                        assembly_buffer = local_buf[:]


                } else if temp_msg_len <= 900 {

                        atomic.AddUint32(&c.server.sendback_size_lt_900b,1)

                        assembly_buffer = local_buf[:]

                } else if temp_msg_len <= 4096 {

                        atomic.AddUint32(&c.server.sendback_size_lt_4k,1)
                        assembly_buffer = global4kPool.Get().([]byte)

                } else if temp_msg_len <= 4096 * 2 {

                        atomic.AddUint32(&c.server.sendback_size_lt_8k,1)
                        assembly_buffer = global8kPool.Get().([]byte)

                } else if temp_msg_len <= 4096 * 4 {

                        atomic.AddUint32(&c.server.sendback_size_lt_16k,1)
                        assembly_buffer = global16kPool.Get().([]byte)

                } else {

                        atomic.AddUint32(&c.server.sendback_size_gt_16k,1)
                        assembly_buffer = make([]byte,temp_msg_len)
                }

                //srv.logf("tcp_server sendback len %d",16+4+roundUp(len(body),AES_BLOCK_SIZE))
                msg_len,err := cm.assembly(assembly_buffer,body)
                //srv.logf("tcp_server sendback len %d",msg_len)
                //srv.logf("tcp_server sendback len %d",len(body))

                if err == nil {
                        written,err = c.writeData(assembly_buffer[:msg_len])


                        if err != nil {
                        atomic.AddUint64(&srv.sendback_write_fail,1)

                        srv.logf("sendback error %d;%d",written,msg_len)
                        //check error in calling function
                        //srv.logf("sendback %s err: %s",c.remoteAddr,err)
                        //TODO handle err here

                        }
                }

                //put buf
                if temp_msg_len>900  && temp_msg_len <= 4096 {
                        global4kPool.Put(assembly_buffer)
                } else if temp_msg_len>4096  && temp_msg_len <= 4096 * 2 {
                        global8kPool.Put(assembly_buffer)

                } else if temp_msg_len>4096 * 2  && temp_msg_len <= 4096 * 4 {
                        global16kPool.Put(assembly_buffer)

                }
                assembly_buffer = nil

                return err
        } else {
		srv.mu.RUnlock()
                atomic.AddUint64(&srv.sendback_fail,1)
                return fmt.Errorf("entry for %d not exist",conn_id)
        }
}

func (srv *Server) newConn(rwc net.Conn,conn_id uint32) (c *conn) {
        c = new(conn)
        c.remoteAddr = rwc.RemoteAddr().String()
        c.server = srv
        c.conn_id = conn_id
        c.rwc = rwc
        c.token_resume_result = make(chan string,1)
        c.now_msec = time.Now().UnixNano()/1e6

        //

        atomic.StoreInt64(&srv.lazy_now , c.now_msec)
        //MOCK
        c.handshake_message = newChannelMessage(c.now_msec)
        //c.handshake_message = newChannelMessage(1423966670599)

        //add map entry
        srv.mu.Lock()
        srv.m[conn_id] = c
        srv.mu.Unlock()

        return c
}

type conn struct {
        now_msec   int64                // time stamp in msec
        conn_id    uint32               // connection id
        remoteAddr string               // network address of remote side
        channel_type byte
        server     *Server              // the Server on which the connection arrived
        rwc        net.Conn             // i/o connection

        app_version string

        token_resume_result chan string

        is_passive_offline int32        // no need to send offline signal to status server ,0-init, 1 req sent, 2 force offline
        handshake_message  channelMessage
        push_id    uint32

        write_mu           sync.Mutex //mutex protect concurrent write from force_offline and sendback routine

        err_count  uint8
        msg_count  uint16
        
}

func (c *conn) sendDecodeFail() (error) {

        c.write_mu.Lock()
        var cm channelMessage
        cm.m_code = S_ERROR_CODE_DEC_FAIL
        var assembly_buffer []byte
        assembly_buffer = make([]byte,16)
        msg_len,err := cm.assembly(assembly_buffer,nil)
        _,err = c.rwc.Write(assembly_buffer[:msg_len])
        c.rwc.(*net.TCPConn).CloseWrite()
        c.write_mu.Unlock()
        if debugTcpServer {
                c.server.logf("decode fail force offline: %s, conn_id: %d %v",c.remoteAddr,c.conn_id,assembly_buffer[:msg_len])
        }
        return err
}

func (c *conn) sendUidOrTokenNotValid() (error) {
        var cm channelMessage
        cm.m_req_id = c.handshake_message.m_req_id
        cm.m_type = S_PTOTO_MSG_INVALID_TOKEN
        cm.m_crypt_flag = S_ENC_NOT
        //length must be 33 bytes
        assembly_buffer := make([]byte,33)

        time_str := strconv.FormatInt(atomic.LoadInt64(&c.server.lazy_now),10)
        msg_len,err := cm.assembly(assembly_buffer,[]byte(time_str))
        if err != nil {
                return err
        }
        c.write_mu.Lock()
        _,err = c.rwc.Write(assembly_buffer[:msg_len])
        c.rwc.(*net.TCPConn).CloseWrite()
        c.write_mu.Unlock()
        if debugTcpServer {
                c.server.logf("uid token not valid: %s, conn_id: %d %v",c.remoteAddr,c.conn_id,assembly_buffer[:msg_len])
        }
        return err
}


func (c *conn) sendExpireToken() (error) {
        var cm channelMessage
        cm.m_type = S_PTOTO_MSG_TOKEN_EXPIRE
        cm.m_crypt_flag = S_ENC_NOT
        //length must be 33 bytes
        assembly_buffer := make([]byte,33)

        time_str := strconv.FormatInt(atomic.LoadInt64(&c.server.lazy_now),10)
        msg_len,err := cm.assembly(assembly_buffer,[]byte(time_str))
        if err != nil {
                return err
        }
        c.write_mu.Lock()
        _,err = c.rwc.Write(assembly_buffer[:msg_len])
        c.rwc.(*net.TCPConn).CloseWrite()
        c.write_mu.Unlock()
        if debugTcpServer {
                c.server.logf("token expire: %s, conn_id: %d %v",c.remoteAddr,c.conn_id,assembly_buffer[:msg_len])
        }
        return err
}

func (c *conn) forceOffline() (error) {
        c.write_mu.Lock()
        if c.is_passive_offline == 2 {
                c.write_mu.Unlock()
                return fmt.Errorf("forceOffline: %s already get offline signal write failed!",c.remoteAddr)
        }
        if c.rwc == nil {
                c.write_mu.Unlock()
                return fmt.Errorf("forceOffline: %s %d already close",c.remoteAddr,c.conn_id)
        }
        c.is_passive_offline = 2
        var cm channelMessage
        cm.m_req_id = atomic.AddUint32(&c.push_id,2)
        cm.m_code = S_ERROR_CODE_FORCE_OFFLINE
        var assembly_buffer []byte
        assembly_buffer = make([]byte,16)
        msg_len,err := cm.assembly(assembly_buffer,nil)
        var n int
        c.rwc.(*net.TCPConn).SetWriteDeadline(time.Now().Add(1500*time.Millisecond))
        n,err = c.rwc.Write(assembly_buffer[:msg_len])
        c.rwc.(*net.TCPConn).CloseWrite()
        c.write_mu.Unlock()
        if debugTcpServer {
                c.server.logf("force offline: %s, conn_id: %d dev_id %s %d,%s",c.remoteAddr,c.conn_id,c.handshake_message.m_device_id,n,err)
        }
        return err
}

func (c *conn) writeData(wb []byte) (n int,err error) {
        var written int
        c.write_mu.Lock()
        if c.rwc == nil {
                c.write_mu.Unlock()
                return 0,fmt.Errorf("writeData: conn_id %d %s already offline!",c.conn_id,c.remoteAddr)
        }
        c.rwc.(*net.TCPConn).SetWriteDeadline(time.Now().Add(1500*time.Millisecond))
        for written < len(wb) {
                n,err = c.rwc.Write(wb[written:])
                written += n
                if err != nil {
                        c.write_mu.Unlock()
                        return written,err
                }
        }
        c.write_mu.Unlock()
        return n,err
}

//when receive force offline we did not need to send offline notification to upstream server
// Close the connection.
func (c *conn) close() {

        //delete map entry
        c.server.mu.Lock()
        delete(c.server.m,c.conn_id)
        close(c.token_resume_result)
        c.server.mu.Unlock()

        //notify upstream status server, if we are active off line

        if c.channel_type == GENERAL_CHANNEL {

                atomic.AddUint32(&c.server.current_general_user,^uint32(0))

                c.server.logf("offline %d:%d",c.handshake_message.uid,c.conn_id)

                if atomic.LoadInt32(&c.is_passive_offline) == 1 {

                        if err := c.server.t.sendStatusRequest(c.conn_id,
                                                        c.handshake_message.m_device_id,
                                                        c.handshake_message.uid,
                                                        0,
                                                        c.app_version,
                                                        statuspb_gogo.StatusRequest_OFFLINE,
                                                        statuspb_gogo.StatusRequest_UID);err != nil {
                                c.server.logf("sned general offline signal error")
                        }
                }

        } else {
        //ANONYMOUS
                atomic.AddUint32(&c.server.current_anonymous_user,^uint32(0))

                c.server.logf("offline %s:%d",c.handshake_message.m_device_id,c.conn_id)

                if atomic.LoadInt32(&c.is_passive_offline) == 1 {

                        if err := c.server.t.sendStatusRequest(c.conn_id,
                                                        c.handshake_message.m_device_id,
                                                        //c.handshake_message.uid,
                                                        0,
                                                        0,
                                                        c.app_version,
                                                        statuspb_gogo.StatusRequest_OFFLINE,
                                                        statuspb_gogo.StatusRequest_DEVICEID);err != nil {
                                c.server.logf("sned annoymous offline signal error")
                        }

                }

        }
 

        var err error

        c.write_mu.Lock()
        //c.finalFlush()
        if c.rwc != nil {
                err = c.rwc.Close()
                c.rwc = nil
        } else {
                c.server.logf("close nil socket %d",c.conn_id)
        }
        c.write_mu.Unlock()

        //decrement counter
        atomic.AddUint32(&c.server.alive_conn, ^uint32(0))

        if err != nil {
                c.server.logf("closing error:%s %s, conn_id: %d, ",err,c.remoteAddr,c.conn_id)
        }

        if debugTcpServer {
                c.server.logf("closing: %s, conn_id: %d",c.remoteAddr,c.conn_id)
        }
}

func (c *conn) serve() {
        defer func() {
                if err := recover(); err != nil {
                        c.server.logf("panic serving %s: %s\n", c.remoteAddr, err)
                        c.server.logf("panic serving %s: %s\n", c.remoteAddr, debug.Stack() )
                }
                c.close()
        }()
        
        var body_buf []byte
        alive := false
        var msg_len uint32
        msg_len_buf := [4]byte{0,0,0,0}   // 32bits buffer for msg_len
        var assembly_buffer []byte
        var local_buf [900]byte

        //start handshake here
        //hand shake message is exactly 160 bytes
        _,err := io.ReadFull(c.rwc,msg_len_buf[:])
        msg_len = binary.BigEndian.Uint32(msg_len_buf[:])

        if msg_len != 160 || err != nil {
                c.server.logf("wrong handshake msg length %d, close conn now ,err: %s",msg_len,err)
                return
        }


        body_buf    = local_buf[:msg_len-4]
        _,err = io.ReadFull(c.rwc,body_buf[:msg_len-4])
        if debugTcpServer {
                //c.server.logf("read msg_len: %v \n",msg_len_buf)
                        if msg_len - 4 > 132 {
                c.server.logf("read msg: %v err: %s, from: %s, conn_id %d",body_buf[:132],err,c.remoteAddr,c.conn_id)
                        } else {
                c.server.logf("read msg: %v err: %s, from: %s, conn_id %d",body_buf[:msg_len-4],err,c.remoteAddr,c.conn_id)
                        

                        }
        }

        if err != nil {
                atomic.AddUint64(&c.server.handshake_fail_count,1)
                c.server.logf("msg_len read error in handshake %s,conn_id %d: %s\n", c.remoteAddr, c.conn_id, err)
                return
        }

        err = c.handshake_message.parse(body_buf,msg_len-4,stageHandshake)


        //TODO check error
        if err != nil {
                c.server.logf("parse handshake error %s",err)
                atomic.AddUint64(&c.server.handshake_fail_count,1)
                c.sendDecodeFail()
                //rethrn something here
                //break
                return
        }

        atomic.AddUint64(&c.server.handshake_sucess_count,1)
        if debugTcpServer {
                c.server.logf("aes key: %s",string(c.handshake_message.aes_dynamic_key))
                c.server.logf("m_type : %d",c.handshake_message.m_type)

        }

        if c.handshake_message.m_type == S_PTOTO_MSG_EXCHANGE_KEY {
                c.channel_type = GENERAL_CHANNEL
        } else {
                c.channel_type = ANONYMOUS_CHANNEL
        }

        if c.channel_type == ANONYMOUS_CHANNEL || true ==c.server.checkToken(c.conn_id,
                                        string(c.handshake_message.m_token),
                                        string(c.handshake_message.m_user_id),
                                        c.handshake_message.uid,
                                        c.now_msec,
                                        c.token_resume_result) {


        assembly_buffer = local_buf[:]
        c.handshake_message.m_crypt_flag = S_ENC_AES_STATIC
        msg_len,err = c.handshake_message.assembly(assembly_buffer,[]byte(c.handshake_message.unix_msec))
        _,err = c.writeData(assembly_buffer[:msg_len])

                //TODO need read metadata here
                _,err = io.ReadFull(c.rwc,msg_len_buf[:])
                msg_len = binary.BigEndian.Uint32(msg_len_buf[:])

                if err != nil {
                        c.server.logf("catch %s in read client metadata",err)
                        return
                }

                body_buf    = local_buf[:]

                if msg_len -4 > 900{
                        c.server.logf("metadata buffer overflow")
                        c.sendDecodeFail()
                        return
                }

                _,err = io.ReadFull(c.rwc,body_buf[:msg_len-4])
                if err != nil {

                        c.server.logf("catch %s in read client metadata",err)
                        return
                }

                err = c.handshake_message.parse(body_buf,msg_len - 4,stageMsgloop)

                if err != nil {

                        c.sendDecodeFail()
                        c.server.logf("catch %s in parse client metadata",err)
                        return
                }

                if c.handshake_message.m_type != S_PTOTO_MSG_CLIENT_METADATA {
                        c.server.logf("should read client metadata here")
                        c.sendDecodeFail()
                        return

                }
                //read app and version
                offset := 4 + binary.BigEndian.Uint32(c.handshake_message.payload[:4])
                if int(offset) > len(c.handshake_message.payload) {
                        c.server.logf("metadata overflow ")
                        c.sendDecodeFail()
                        return

                }
                app := string(c.handshake_message.payload[4:offset])

                offset2 := binary.BigEndian.Uint32(c.handshake_message.payload[offset:offset + 4])
                offset += 4
                offset2 += offset

                if int(offset2) > len(c.handshake_message.payload) {
                        c.server.logf("metadata overflow ")
                        c.sendDecodeFail()
                        return


                }

                version := string(c.handshake_message.payload[offset:offset2])

                c.app_version = app + version

        if c.channel_type == GENERAL_CHANNEL {
                
                atomic.AddUint32(&c.server.current_general_user,1)

                //INFO
                c.server.logf("online %s:%d",c.handshake_message.uid,c.conn_id)

                if debugTcpServer {
                        c.server.logf("general app: %s, version: %s, from: %s",app,version,c.remoteAddr)

                }

                

                if err := c.server.t.sendStatusRequest(c.conn_id,
                                                c.handshake_message.m_device_id,
                                                c.handshake_message.uid,
                                                c.handshake_message.client_ts,
                                                c.app_version,
                                                statuspb_gogo.StatusRequest_ONLINE,
                                                statuspb_gogo.StatusRequest_UID);err != nil {
                        c.server.logf("send general online signal error")
                }

        } else {
        //ANONYMOUS
                atomic.AddUint32(&c.server.current_anonymous_user,1)

                //INFO
                c.server.logf("online %s:%d",c.handshake_message.m_device_id,c.conn_id)

                if debugTcpServer {
                        c.server.logf("anonymous app: %s, version: %s, from: %s",app,version,c.remoteAddr)

                }


                if err := c.server.t.sendStatusRequest(c.conn_id,
                                                c.handshake_message.m_device_id,
                                                //c.handshake_message.uid,
                                                0,
                                                c.handshake_message.client_ts,
                                                c.app_version,
                                                statuspb_gogo.StatusRequest_ONLINE,
                                                statuspb_gogo.StatusRequest_DEVICEID);err != nil {
                        c.server.logf("send annoymous online signal error")
                }

        }

        //set is_passive_offline to normal state
        atomic.StoreInt32(&c.is_passive_offline,1)







        //c.server.logf("send back finish")
        //TODO send back reject signal here
        
        alive = true
        atomic.AddUint64(&c.server.handshake_sucess_count2,1)

        } else {
                atomic.AddUint32(&c.server.token_fail_count,1)
                //send not valid response
                c.sendUidOrTokenNotValid()
                c.server.logf("check token error for %s %s %s ",c.handshake_message.m_token,c.handshake_message.m_user_id,c.remoteAddr)
                return
        }



        //main loop start here
        for alive {
                //read 4 bytes msg_len
                _,err := io.ReadFull(c.rwc,msg_len_buf[:])
                if err != nil {
                        c.server.logf("msg_len read error in mainloop %s,conn_id %d: %s\n", c.remoteAddr, c.conn_id, err)
                        break
                }
                msg_len = binary.BigEndian.Uint32(msg_len_buf[:])

                //check msg_len
                if msg_len <4 || msg_len > MAX_MSG_BODY_LEN {
                        c.sendDecodeFail()
                        c.server.logf("msg_len length error from %s,conn_id %d: %d\n", c.remoteAddr,c.conn_id, msg_len)
                        //TODO need review
                        //break
                        return
                }

                //get buf from pool
                temp_msg_len := msg_len -4

                if temp_msg_len <= 400 {

                        atomic.AddUint32(&c.server.size_lt_400b,1)

                        body_buf = local_buf[:]

                
                } else if temp_msg_len <= 900 {

                        atomic.AddUint32(&c.server.size_lt_900b,1)

                        body_buf = local_buf[:]

                } else if temp_msg_len <= 4096 {

                        atomic.AddUint32(&c.server.size_lt_4k,1)

                        body_buf = global4kPool.Get().([]byte)

                } else if temp_msg_len <= 4096 * 2 {

                        atomic.AddUint32(&c.server.size_lt_8k,1)

                        body_buf = global8kPool.Get().([]byte)

                } else if temp_msg_len <= 4096 * 4 {

                        atomic.AddUint32(&c.server.size_lt_16k,1)

                        body_buf = global16kPool.Get().([]byte)

                } else {

                        atomic.AddUint32(&c.server.size_gt_16k,1)

                        body_buf = make([]byte,temp_msg_len)
                }


                _,err = io.ReadFull(c.rwc,body_buf[:msg_len-4])

                if debugTcpServer {
                        if msg_len - 4 > 132 {
                c.server.logf("read msg: %v err: %s, from: %s, conn_id %d",body_buf[:132],err,c.remoteAddr,c.conn_id)
                        } else {
                c.server.logf("read msg: %v err: %s, from: %s, conn_id %d",body_buf[:msg_len-4],err,c.remoteAddr,c.conn_id)
                        

                        }

                }
                //check body_len
                if err != nil {
                        atomic.AddUint64(&c.server.total_msg_err_cnt,1)
                        c.server.logf("msg_body length error from %s,conn_id %d: %s\n", c.remoteAddr,c.conn_id,err)
                        break
                }
                err = c.handshake_message.parse(body_buf,msg_len - 4,stageMsgloop) 

                //TODO handle error here

                //need review
                if err != nil {
                        c.server.logf("payload : %s",string(c.handshake_message.payload))
                        c.server.logf("err in parse msg %s,%s",err,c.remoteAddr)
                        //c.sendDecodeFail()
                        c.err_count ++
                        atomic.AddUint64(&c.server.total_msg_err_cnt,1)
                        if c.err_count > MAX_PARSE_ERROR {
                                break
                        }
                        continue
                }

                atomic.AddUint64(&c.server.total_msg_ok_cnt,1)
                if debugTcpServer {
                        c.server.logf("payload : %s",string(c.handshake_message.payload))
                        c.server.logf("type : %d",c.handshake_message.m_type)
                }
                //comment to test parse message
                if c.channel_type == GENERAL_CHANNEL {

                err = c.server.t.sendUpstreamRequest(c.handshake_message.m_type,c.conn_id,
                                                        c.handshake_message.m_req_id,
                                                        upstreampb_gogo.UpstreamRequest_UID,
                                                        c.handshake_message.uid,
                                                        "",
                                                        c.handshake_message.payload)

                        if err != nil {
                                c.server.logf("send_upstream error %s\n",err)
                        } 

                } else {

                //ANONYMOUS_CHANNEL
                err = c.server.t.sendUpstreamRequest(c.handshake_message.m_type,c.conn_id,
                                                        c.handshake_message.m_req_id,
                                                        upstreampb_gogo.UpstreamRequest_DEVICEID,
                                                        0,
                                                        string(c.handshake_message.m_anonymous_deviceid),
                                                        c.handshake_message.payload)

                        if err != nil {
                                c.server.logf("send_upstream error %s\n",err)
                        } 

                }


                //put buf
                if temp_msg_len>900  && temp_msg_len <= 4096 {
                        global4kPool.Put(body_buf)
                } else if temp_msg_len>4096  && temp_msg_len <= 4096 * 2 {
                        global8kPool.Put(body_buf)

                } else if temp_msg_len>4096 * 2  && temp_msg_len <= 4096 * 4 {
                        global16kPool.Put(body_buf)

                }
                body_buf = nil

                c.msg_count += 1
        }

}




