package tcp_agent

import (
        "bufio"
        "io"
        "os"
        "net"
        "fmt"
        "sync"
        "sync/atomic"
        "time"
        "errors"
        "log"
        "strings"
        "strconv"
        "encoding/binary"
        "../statuspb_gogo"
        "../upstreampb_gogo"
        "../pushpb_gogo"
        "github.com/gogo/protobuf/proto"
)

var (
        errStatusChanFull      = errors.New("write channel for status_request is full")
        errRestChanFull        = errors.New("write channel for token_check is full")
        errUpstreamChanFull    = errors.New("write channel for upstream_request is full")
        errRtrBufLen           = errors.New("restagent read overflow")
        errStatusDead          = errors.New("all status server died")

)

const debugTransport = true

const cmdUpstreamConnsNum = 8

const MAX_BACKOFF_TIME = 60

const BUF_LEN_FOR_REST = 256

const STATUS_STATE_INIT = 0

const STATUS_STATE_MASTER = 1

const STATUS_STATE_SLAVE = 2

const STATUS_STATE_ERROR = 3

type Transport struct {
        //counter
        msg_counter     uint64

        status_full_cnt    uint32
        upstream_full_cnt  uint32
        rest_full_cnt      uint32

        status_ok_cnt      uint32
        upstream_ok_cnt    uint32
        rest_ok_cnt        uint32

        upstream_404_cnt  uint32

        s               *Server
        ErrorLog        *log.Logger    //log.Logger has internal locking to protect concurrent access
        connmapMu       sync.RWMutex
        status_conns    *cmdConnections
        status_slave_conns    *cmdConnections
        rest_conns_array      []*cmdConnections
        push_conns_array      []*cmdConnections

        //for range match 
        low_index       []uint16        
        high_index      []uint16        
        cmd_conns       []*cmdConnections

        cmd_count_map   map[uint16]*uint64
        cmd_count_mu    sync.RWMutex


        status_writech     chan statuspb_gogo.StatusRequest      // written by roundTrip; read by writeLoop
        status_slave_writech     chan statuspb_gogo.StatusRequest      // written by roundTrip; read by writeLoop

        rtr_writech     chan restTokenRequest            // written by roundTrip; read by writeLoop

        status_state    uint32  //0-init, 1-master, 2-slave, 3-error
        status_master_alive    uint32  //0-init, 1-master, 2-slave, 3-error
        status_slave_alive    uint32  //0-init, 1-master, 2-slave, 3-error
        //status_master_wg  sync.WaitGroup
        //status_slave_wg   sync.WaitGroup
}


func (t *Transport) logf(format string, args ...interface{}) {
        t.ErrorLog.Printf(format, args...)
}

func (t *Transport) registerServer(s *Server) {
        if t.cmd_count_map == nil {
                t.cmd_count_map = make(map[uint16]*uint64,60)
        }
        if t.s == nil {
                t.s = s
        }
}


func (t *Transport) registerPush(ip_port string) (err error){
        _,err = net.ResolveTCPAddr("tcp",ip_port)
        if err != nil{
                panic("error in RegisterRestagent: resolve error" + ip_port)
        }

        //setup log facility
        if t.ErrorLog == nil {
                //logFile, _ := os.OpenFile("./transport.log", os.O_WRONLY | os.O_CREATE, 0644)
                t.ErrorLog = log.New(os.Stderr, "[transport]: ", log.LstdFlags)
                //t.ErrorLog = log.New(logFile, "[transport]: ", log.LstdFlags)
        }

        t.logf("register push server %s",ip_port)

        push_conns := &cmdConnections{
                cmd:           LOOP_PUSH,
                ip_port:       ip_port,
                t:             t,
                }

        for i := 0;i < cmdUpstreamConnsNum;i++{
                dc,err := push_conns.dialConn(ip_port)
                if err == nil{
                        push_conns.conns = append(push_conns.conns,dc)
                } else {
                        panic(fmt.Sprintf("dial conn error for push, ip_port: %s",ip_port))
                }
        }

        t.push_conns_array = append(t.push_conns_array,push_conns)

        return nil
}

func (t *Transport) registerRestagent(ip_port string) (err error){
        _,err = net.ResolveTCPAddr("tcp",ip_port)
        if err != nil{
                panic("error in RegisterRestagent: resolve error" + ip_port)
        }

        //setup log facility
        if t.ErrorLog == nil {
                //logFile, _ := os.OpenFile("./transport.log", os.O_WRONLY | os.O_CREATE, 0644)
                t.ErrorLog = log.New(os.Stderr, "[transport]: ", log.LstdFlags)
                //t.ErrorLog = log.New(logFile, "[transport]: ", log.LstdFlags)
        }

        t.logf("register restagent server %s",ip_port)


        if t.rtr_writech == nil {
                t.rtr_writech =  make(chan restTokenRequest,800)
        }

        rest_conns := &cmdConnections{
                cmd:           LOOP_TOKEN_REST,
                ip_port:       ip_port,
                t:             t,
                }

        for i := 0;i < cmdUpstreamConnsNum;i++{
                dc,err := rest_conns.dialConn(ip_port)
                if err == nil{
                        rest_conns.conns = append(rest_conns.conns,dc)
                } else {
                        panic(fmt.Sprintf("dial conn error for restagent, ip_port: %s",ip_port))
                }
        }

        t.rest_conns_array = append(t.rest_conns_array,rest_conns)

        return nil
}

func (t *Transport) registerStatus(ip_port string,is_slave bool) (err error){
        _,err = net.ResolveTCPAddr("tcp",ip_port)
        if err != nil{
                panic("error in RegisterStatus: resolve error" + ip_port)
        }

        //setup log facility
        if t.ErrorLog == nil {
                t.ErrorLog = log.New(os.Stderr, "[transport]: ", log.LstdFlags)
        }

        t.logf("register status server %s,is_slave: %t",ip_port,is_slave)


        if t.status_writech == nil {
                t.status_writech =  make(chan statuspb_gogo.StatusRequest,900)
        }

        if t.status_slave_writech == nil {
                t.status_slave_writech =  make(chan statuspb_gogo.StatusRequest,900)
        }

        temp_conns := &cmdConnections {
                cmd:           LOOP_STATUS,
                ip_port:       ip_port,
                status_is_slave:is_slave,
                t:             t,
                }

        //WAITGROUP
        //t.status_master_wg.Add(2)
        //t.status_slave_wg.Add(2)

        for i := 0;i < cmdUpstreamConnsNum;i++{
                dc,err := temp_conns.dialConn(ip_port)
                if err == nil{
                        temp_conns.conns = append(temp_conns.conns,dc)
                        atomic.AddUint32(&temp_conns.alive_conns,1) 
                } else {
                        panic(fmt.Sprintf("dial conn error for status, ip_port: %s",ip_port))
                }
        }

        if is_slave == false {
                t.status_conns = temp_conns
                atomic.StoreUint32(&t.status_state,STATUS_STATE_MASTER)
                atomic.StoreUint32(&t.status_master_alive,1)
                t.logf("set master alive to 1 %s",ip_port)
        } else {
                t.status_slave_conns = temp_conns
                atomic.StoreUint32(&t.status_slave_alive,1)
                t.logf("set slave alive to 1 %s",ip_port)
        }

        //WAITGROUP
        //if is_slave == false {
        //        //master
        //        t.status_master_wg.Add(-2)
        //} else {
        //        //slave
        //        t.status_slave_wg.Add(-1)
        //}

        return nil
}

func (t *Transport) registerCmd(low_index uint16,high_index uint16,ip_port string) (err error){
        _ , err = net.ResolveTCPAddr("tcp",ip_port)
        if err != nil{
                panic("error in RegisterCmd: resolve error" + ip_port)
        }

        //setup log facility
        if t.ErrorLog == nil {
                t.ErrorLog = log.New(os.Stderr, "[transport]: ", log.LstdFlags)
        }

        t.logf("register api range %d - %d server %s",low_index,high_index,ip_port)




        t.connmapMu.Lock()
        //bound check
        for i,e := range t.low_index {
                if low_index >= e && low_index <= t.high_index[i] {
                        panic("cmd range overlap")
                }
        }

        for i,e := range t.low_index {
                if high_index >= e && high_index <= t.high_index[i] {
                        panic("cmd range overlap")
                }
        }

        t.low_index = append(t.low_index,low_index)
        t.high_index = append(t.high_index,high_index)

                temp_conns :=  &cmdConnections{
                cmd:           LOOP_ECHO,
                ip_port:       ip_port,
                t:             t,
                us_writech:       make(chan upstreampb_gogo.UpstreamRequest,3000),
                }


                for i := 0;i < cmdUpstreamConnsNum;i++{
                        dc,err := temp_conns.dialConn(ip_port)
                        if err == nil{
                                temp_conns.conns = append(temp_conns.conns,dc)
                        } else {
                                panic(fmt.Sprintf("dial conn error low_index:%d high_index:%d, ip_port: %s",low_index,high_index,ip_port))
                        }
                }
                t.cmd_conns = append(t.cmd_conns,temp_conns)
        t.connmapMu.Unlock()
        return nil
}

func (t *Transport) incrementCmdCounter(conn_id uint16) {
        t.cmd_count_mu.RLock()
        if element,exist := t.cmd_count_map[conn_id];exist {
                t.cmd_count_mu.RUnlock()
                atomic.AddUint64(element,1)
                return
        }
        t.cmd_count_mu.RUnlock()

        
        t.cmd_count_mu.Lock()
        if element,exist := t.cmd_count_map[conn_id];exist {
                t.cmd_count_mu.Unlock()
                atomic.AddUint64(element,1)
                return
        } else {
                temp := new(uint64) 
                t.cmd_count_map[conn_id] = temp
                t.cmd_count_mu.Unlock()
                atomic.AddUint64(temp,1)
                return
        }

        

}


// send token check request to upstream
func (t *Transport) sendStatusRequest(conn_id uint32,device_id []byte,uid uint64,timestamp uint64,app_version string,
                                        req_type statuspb_gogo.StatusRequest_Type,
                                        field_type statuspb_gogo.StatusRequest_FieldType) (err error){
        
        atomic.AddUint64(&t.msg_counter,1)

        switch atomic.LoadUint32(&t.status_state) {
                case STATUS_STATE_MASTER:
                        if cap(t.status_writech) == len(t.status_writech) {
                                atomic.AddUint32(&t.status_full_cnt,1)
                                return errStatusChanFull
                        }

                        atomic.AddUint32(&t.status_ok_cnt,1)

                        if field_type == statuspb_gogo.StatusRequest_UID {
                                t.status_writech <- statuspb_gogo.StatusRequest {
                                        ReqType : req_type,
                                        FieldType: field_type,
                                        Uid:proto.Uint64(uid),
                                        DeviceId:proto.String(string(device_id)),
                                        AppVersion:proto.String(app_version),
                                        NetId:proto.Uint32(conn_id),
                                        Timestamp:proto.Uint64(timestamp),
                                        ServerId:t.s.ServerId,
                                } 

                                if debugTransport {
                                        t.logf("send status uid %d",uid)
                                }
                        } else {
                                
                                t.status_writech <- statuspb_gogo.StatusRequest {
                                        ReqType : req_type,
                                        FieldType: field_type,
                                //        Uid:proto.Uint64(uid),
                                        DeviceId:proto.String(string(device_id)),
                                        AppVersion:proto.String(app_version),
                                        NetId:proto.Uint32(conn_id),
                                        Timestamp:proto.Uint64(timestamp),
                                        ServerId:t.s.ServerId,
                                } 

                                if debugTransport {
                                        t.logf("send dev_id %s",device_id)
                                }
                        }
                case STATUS_STATE_SLAVE:
                        if cap(t.status_slave_writech) == len(t.status_slave_writech) {
                                atomic.AddUint32(&t.status_full_cnt,1)
                                return errStatusChanFull
                        }

                        atomic.AddUint32(&t.status_ok_cnt,1)

                        if field_type == statuspb_gogo.StatusRequest_UID {
                                t.status_slave_writech <- statuspb_gogo.StatusRequest {
                                        ReqType : req_type,
                                        FieldType: field_type,
                                        Uid:proto.Uint64(uid),
                                        DeviceId:proto.String(string(device_id)),
                                        AppVersion:proto.String(app_version),
                                        NetId:proto.Uint32(conn_id),
                                        Timestamp:proto.Uint64(timestamp),
                                        ServerId:t.s.ServerId,
                                } 

                                if debugTransport {
                                        t.logf("send status uid %d",uid)
                                }
                        } else {
                                
                                t.status_slave_writech <- statuspb_gogo.StatusRequest {
                                        ReqType : req_type,
                                        FieldType: field_type,
                                //        Uid:proto.Uint64(uid),
                                        DeviceId:proto.String(string(device_id)),
                                        AppVersion:proto.String(app_version),
                                        NetId:proto.Uint32(conn_id),
                                        Timestamp:proto.Uint64(timestamp),
                                        ServerId:t.s.ServerId,
                                } 

                                if debugTransport {
                                        t.logf("send dev_id %s",device_id)
                                }
                        }
                case STATUS_STATE_ERROR:
                        return errStatusDead
                }

        return nil
}

// send token check request to upstream
func (t *Transport) sendTokenCheckRequest(conn_id uint32,token string,uid string) (err error){
        
        //increment msg counter
        atomic.AddUint64(&t.msg_counter,1)

                if cap(t.rtr_writech) == len(t.rtr_writech) {
                        atomic.AddUint32(&t.rest_full_cnt,1)
                        return errRestChanFull
                }

        atomic.AddUint32(&t.rest_ok_cnt,1)
        
                t.rtr_writech <- restTokenRequest {
                                 conn_id:        conn_id,
                                 token:          token,
                                 uid:            uid,
                } 
        return

}

//TODO not for generic type
func (t *Transport) sendUpstreamRequest(cmd uint16,conn_id uint32,msg_id uint32,
                                        field_type upstreampb_gogo.UpstreamRequest_FieldType,
                                        uid uint64,
                                        device_id string,
                                        body []byte) (err error) {
        //increment msg counter
        atomic.AddUint64(&t.msg_counter,1)

        for i,e := range t.low_index {
                if cmd >= e && cmd <= t.high_index[i] {
                        //sendback here
                        cmdConn := t.cmd_conns[i]
                        if len(cmdConn.us_writech) < cap(cmdConn.us_writech) {
                                if field_type == upstreampb_gogo.UpstreamRequest_UID {

                                //INFO
                                t.logf("send uid %d:%d:%d",uid,conn_id,msg_id)

                                cmdConn.us_writech <- upstreampb_gogo.UpstreamRequest {
                                       NetId :   (conn_id),
                                       ReqId:    (msg_id),
                                       FieldType:field_type, 
                                       Uid :     proto.Uint64(uid),
                                       Body:     (string(body)),

                                   }
                                } else {
                                //handle device id here

                                //INFO
                                t.logf("send did %s:%d:%d",device_id,conn_id,msg_id)

                                cmdConn.us_writech <- upstreampb_gogo.UpstreamRequest {
                                       NetId :   (conn_id),
                                       ReqId:    (msg_id),
                                       FieldType:field_type, 
                                       DeviceId: proto.String(device_id),
                                       Body:(string(body)),
                                   }
                                }
                                atomic.AddUint32(&t.upstream_ok_cnt,1)
                                t.incrementCmdCounter(cmd)

                                return nil
                        } else {

                                atomic.AddUint32(&t.upstream_full_cnt,1)
                                return errUpstreamChanFull
                        }
                }
        }

        
        atomic.AddUint32(&t.upstream_404_cnt,1)

        return fmt.Errorf("cmd: %d not exist",cmd)
}

type cmdConnections struct {
        cmd         uint16
        ip_port     string
        t           *Transport
        us_writech     chan upstreampb_gogo.UpstreamRequest  // written by roundTrip; read by writeLoop
        alive_conns uint32
        conns       []*persistConn
        Dial        func(network, addr string) (net.Conn, error)
        status_is_slave  bool
}

type restTokenRequest struct {
        conn_id         uint32
        token           string
        uid             string
}

func (rr *restTokenRequest) reset(){
        rr.conn_id = 0
        rr.token   = ""
        rr.uid     = ""
}

//we skip encoding/json here to improve performance
//write function share same buffer from calling function
func (rr *restTokenRequest) write(w *bufio.Writer,wb []byte) (err error){
        //print json string
        cplen := 4
        cplen += copy(wb[4:],"{\"url\":\"/ismsg/check_token_tcpd?token=")
        //cplen += copy(wb[4:],"{\"url\":\"/ccim/ccpsn/ccim/check_token?token=")
        cplen += copy(wb[cplen:],rr.token)
        cplen += copy(wb[cplen:],"\",\"id\":\"")
        cplen += copy(wb[cplen:],strconv.Itoa(int(rr.conn_id)))
        cplen += copy(wb[cplen:],"\",\"context\":\"abcdefg\"}")
        binary.BigEndian.PutUint32(wb[:4],uint32(cplen))

        _,err = w.Write(wb[:cplen])
        return err
}

func (rr *restTokenRequest) read(r *bufio.Reader,body_buf []byte) (err error){
        _ , err = io.ReadFull(r,body_buf[:4])
        if err == nil {
                msg_len :=  binary.BigEndian.Uint32(body_buf[:4])
                if msg_len > BUF_LEN_FOR_REST {
                        return errRtrBufLen

                }
                _ , err = io.ReadFull(r,body_buf[4:msg_len])
                if err != nil {
                        return err
                }

                rs := string(body_buf[4:msg_len])

                //find id
                rindex := strings.Index(rs,"\"id\":\"")
                rindex += len("\"id\":\"")
                rindex2 := strings.IndexByte(rs[rindex:],'"')
                cid,err := strconv.Atoi(rs[rindex:rindex+rindex2])
                if err != nil {
                        return err
                }
                rr.conn_id = uint32(cid)

                //check ret from upstream
                rindex = strings.Index(rs,"\"ret\":\"")
                if rindex == -1 {
                        return fmt.Errorf("restAgent: cannot find ret from upstream restagent")
                }
                rindex += len("\"ret\":\"")
                rindex2 = strings.IndexByte(rs[rindex:],'"')
                //TODO not use atoi, use  []byte compare
                rc_strconv,err := strconv.Atoi(rs[rindex:rindex+rindex2])
                if err != nil {
                        return err
                }
                if rc_strconv == -1 {
                        return fmt.Errorf("restAgent: ret = -1 from upstream restagent")
                }

                //find user id
                rindex = strings.Index(rs,"\"X-IS-UserID\":\"")
                if rindex == -1 {
                        //check for nil value in calling function
                        rr.uid = ""
                        return nil
                }
                rindex += len("\"X-IS-UserID\":\"")
                rindex2 = strings.IndexByte(rs[rindex:],'"')
                rr.uid = rs[rindex:rindex+rindex2]

        }
        return err

}


type noteEOFReader struct {
        r          io.Reader
        notify_eof chan struct{}         //channel for readloop no notify write loop, no buffer here
}

func (nr noteEOFReader) Read(p []byte) (n int, err error) {
        n, err = nr.r.Read(p)
        //TODO need review
        //if err == io.EOF {
        if err != nil {
                nr.notify_eof <- struct{}{}
        }
        return
}


func (cc *cmdConnections) dial(network, addr string) (c net.Conn, err error) {
        if cc.Dial == nil {
                cc.Dial = (&net.Dialer{
                Timeout:   30 * time.Second,
                KeepAlive: 30 * time.Second,
                }).Dial
        }
        c , err = cc.Dial(network,addr)
        return
}



func (cc *cmdConnections) dialConn(ip_port string) (*persistConn, error) {
        conn, err := cc.dial("tcp", ip_port)
        if err != nil {
                return nil, err
        }
        pconn := &persistConn{
                ipport:     ip_port,
                cmdConns:   cc,
                conn:       conn,
                notify_eof: make(chan struct{}),
                exp_backoff_time: 2,
        }
        pconn.br = bufio.NewReader(noteEOFReader{pconn.conn,pconn.notify_eof})
        pconn.bw = bufio.NewWriter(pconn.conn)
        pconn.spawnReadLoop()
        pconn.spawnWriteLoop()
        return pconn, nil
}


type persistConn struct {
        ipport          string
        cmdConns        *cmdConnections
        conn           net.Conn
        br             *bufio.Reader       // from conn
        bw             *bufio.Writer       // to conn
        notify_eof     chan struct{}         //channel for readloop no notify write loop, no buffer here
        exp_backoff_time  uint8 
}


func (pc *persistConn) close() {
        pc.cmdConns.t.logf("closing %s ",pc.ipport)

        if 0 == atomic.AddUint32(&pc.cmdConns.alive_conns,^uint32(0)) {
                //WAITGROUP
                //if pc.cmdConns.status_is_slave == true {
                //        pc.cmdConns.t.status_slave_wg.Add(2)
                //        pc.cmdConns.t.status_master_wg.Done()
                //} else {
                //        pc.cmdConns.t.status_master_wg.Add(2)
                //        pc.cmdConns.t.status_slave_wg.Done()
                //}
                if pc.cmdConns.cmd == LOOP_STATUS {

                        if pc.cmdConns.status_is_slave == false {
                                atomic.StoreUint32(&pc.cmdConns.t.status_master_alive,0)
                                pc.cmdConns.t.logf("set master alive to 0 %s",pc.cmdConns.ip_port)
                        } else {
                                atomic.StoreUint32(&pc.cmdConns.t.status_slave_alive,0)
                                pc.cmdConns.t.logf("set slave alive to 0 %s",pc.cmdConns.ip_port)
                        }

                        switch atomic.LoadUint32(&pc.cmdConns.t.status_state) {
                                case STATUS_STATE_MASTER:
                                        if pc.cmdConns.status_is_slave == false {
                                                if atomic.LoadUint32(&pc.cmdConns.t.status_slave_alive) == 1 {
                                                        atomic.StoreUint32(&pc.cmdConns.t.status_state,STATUS_STATE_SLAVE)
                                                        pc.cmdConns.t.logf("switch from master to slave")
                                                } else {
                                                        atomic.StoreUint32(&pc.cmdConns.t.status_state,STATUS_STATE_ERROR)
                                                        pc.cmdConns.t.logf("switch from master to error")
                                                }
                                        }
                                case STATUS_STATE_SLAVE:
                                        if pc.cmdConns.status_is_slave == true {
                                                if atomic.LoadUint32(&pc.cmdConns.t.status_master_alive) == 1 {
                                                        atomic.StoreUint32(&pc.cmdConns.t.status_state,STATUS_STATE_MASTER)
                                                        pc.cmdConns.t.logf("switch from slave to master")
                                                } else {
                                                        atomic.StoreUint32(&pc.cmdConns.t.status_state,STATUS_STATE_ERROR)
                                                        pc.cmdConns.t.logf("switch from slave to error")
                                                }
                                        }

                        }

                }
        }
        pc.conn.Close()
}

func (pc *persistConn) writeLoopForUpstream() {
        pc.conn.(*net.TCPConn).SetWriteBuffer(2*1024*1024)
        writeCounter := 0
        var upstream_request upstreampb_gogo.UpstreamRequest
        upstream_buffer := make([]byte,1024*1024)
        for {
                //wait for request
                select {
                        case   <- pc.notify_eof:
                                pc.cmdConns.t.logf("get notify eof signal %d",pc.cmdConns.cmd)
                                pc.bw.Flush()
                                return
                        case upstream_request = <-pc.cmdConns.us_writech:
                }


                if upstream_request.Size() > 1024*1024 - 4 {
                        pc.cmdConns.t.logf("size too big in upstream write loop  %d",upstream_request.Size())
                        continue
                }
                n,err := upstream_request.MarshalTo(upstream_buffer[4:])
                if err != nil {
                        pc.cmdConns.t.logf("pb marshal error %s",err)
                        continue
                }
                var msg_len uint32
                msg_len = 4 + uint32(n)
                binary.BigEndian.PutUint32(upstream_buffer[:4],msg_len)
                _,err = pc.bw.Write(upstream_buffer[:msg_len])


                if debugTransport {
                        pc.cmdConns.t.logf("write request to %d,%s upstream: %s, err: %s\n",pc.cmdConns.cmd,pc.cmdConns.ip_port,upstream_request.String(),err)
                }

                if err == nil && writeCounter >= 0  {
                        err = pc.bw.Flush()
                }
                if err != nil {
                        if cap(pc.cmdConns.us_writech) > len(pc.cmdConns.us_writech) + 10 {
                                pc.cmdConns.us_writech <- upstream_request
                        }
                        //should not return here
                        //return
                }
        }  


}


func (pc *persistConn) writeLoopForRestagent() {
        pc.conn.(*net.TCPConn).SetWriteBuffer(8*1024)
        writeCounter := 0
        ltr_buf := make([]byte,512)
        var rtr restTokenRequest
        for {

                //wait for request
                select {
                        case   <- pc.notify_eof:
                                pc.cmdConns.t.logf("get notify eof signal %d",pc.cmdConns.cmd)
                                pc.bw.Flush()
                                return
                        case rtr = <-pc.cmdConns.t.rtr_writech:
                }

                writeCounter += 1
                err := rtr.write(pc.bw,ltr_buf)

                if debugTransport {
                        pc.cmdConns.t.logf("write request to %d,%s upstream: %v, err: %s\n",pc.cmdConns.cmd,pc.cmdConns.ip_port,rtr,err)
                }

                if err == nil && writeCounter >= 0  {
                        err = pc.bw.Flush()
                }
                if err != nil {
                        if cap(pc.cmdConns.t.rtr_writech) > len(pc.cmdConns.t.rtr_writech) + 10 {
                        pc.cmdConns.t.rtr_writech <- rtr
                        }
                        
                        //should not return here, starvation will happen here, use notify_eof to syncronize read/write loop
                        //return
                }


        } //end of case LOOP_TOKEN_REST

}


func (pc *persistConn) writeLoopForStatus() {
        pc.conn.(*net.TCPConn).SetWriteBuffer(4*1024)
        writeCounter := 0
        var err error
        sta_buf := make([]byte,512)

        //send server id to status
        binary.BigEndian.PutUint32(sta_buf[:4],pc.cmdConns.t.s.ServerId)
        pc.cmdConns.t.logf("send server id %v",sta_buf[:4])
        _,err = pc.conn.Write(sta_buf[:4])
        if err != nil {
                pc.cmdConns.t.logf("err in send server id" + err.Error())
        }

        //WAITGROUP
        //if pc.cmdConns.status_is_slave == true {
        //        pc.cmdConns.t.status_slave_wg.Wait()
        //} else {
        //        pc.cmdConns.t.status_master_wg.Wait()
        //}

        //pc.cmdConns.t.logf("status  server transistion %t",pc.cmdConns.status_is_slave)

        var status_request statuspb_gogo.StatusRequest
        for {

                //wait for request
                select {
                        case   <- pc.notify_eof:
                                pc.cmdConns.t.logf("get notify eof signal %d",pc.cmdConns.cmd)
                                pc.bw.Flush()
                                return
                        case status_request = <-pc.cmdConns.t.status_writech:
                }

                writeCounter += 1
                pb_len,err := status_request.MarshalTo(sta_buf[4:])
                if err != nil {
                        pc.cmdConns.t.logf("pb marshal error %s",err)
                        continue
                }
                msg_len := 4 + uint32(pb_len)
                binary.BigEndian.PutUint32(sta_buf[:4],msg_len)
                _,err = pc.bw.Write(sta_buf[:msg_len])

                if debugTransport {
                        pc.cmdConns.t.logf("write request to %d,%s upstream: %s, err: %s",pc.cmdConns.cmd,pc.cmdConns.ip_port,status_request.String(),err)
                }

                if err == nil && writeCounter >= 0  {
                        err = pc.bw.Flush()
                }
                if err != nil {
                        if cap(pc.cmdConns.t.status_writech) > len(pc.cmdConns.t.status_writech) + 10 {
                        pc.cmdConns.t.status_writech <- status_request
                        }
                        //should not return here
                        //return
                }


        } //end of case LOOP_STATUS

}

func (pc *persistConn) writeLoopForStatusSlave() {
        pc.conn.(*net.TCPConn).SetWriteBuffer(4*1024)
        writeCounter := 0
        var err error
        sta_buf := make([]byte,512)

        //send server id to status
        binary.BigEndian.PutUint32(sta_buf[:4],pc.cmdConns.t.s.ServerId)
        pc.cmdConns.t.logf("send server id %v",sta_buf[:4])
        _,err = pc.conn.Write(sta_buf[:4])
        if err != nil {
                pc.cmdConns.t.logf("err in send server id" + err.Error())
        }

        //WAITGROUP
        //if pc.cmdConns.status_is_slave == true {
        //        pc.cmdConns.t.status_slave_wg.Wait()
        //} else {
        //        pc.cmdConns.t.status_master_wg.Wait()
        //}

        //pc.cmdConns.t.logf("status  server transistion %t",pc.cmdConns.status_is_slave)

        var status_request statuspb_gogo.StatusRequest
        for {

                //wait for request
                select {
                        case   <- pc.notify_eof:
                                pc.cmdConns.t.logf("get notify eof signal %d",pc.cmdConns.cmd)
                                pc.bw.Flush()
                                return
                        case status_request = <-pc.cmdConns.t.status_slave_writech:
                }

                writeCounter += 1
                pb_len,err := status_request.MarshalTo(sta_buf[4:])
                if err != nil {
                        pc.cmdConns.t.logf("pb marshal error %s",err)
                        continue
                }
                msg_len := 4 + uint32(pb_len)
                binary.BigEndian.PutUint32(sta_buf[:4],msg_len)
                _,err = pc.bw.Write(sta_buf[:msg_len])

                if debugTransport {
                        pc.cmdConns.t.logf("write request to %d,%s upstream: %s, err: %s",pc.cmdConns.cmd,pc.cmdConns.ip_port,status_request.String(),err)
                }

                if err == nil && writeCounter >= 0  {
                        err = pc.bw.Flush()
                }
                if err != nil {
                        if cap(pc.cmdConns.t.status_slave_writech) > len(pc.cmdConns.t.status_slave_writech) + 10 {
                        pc.cmdConns.t.status_slave_writech <- status_request
                        }
                        //should not return here
                        //return
                }


        } //end of case LOOP_STATUS

}

func (pc *persistConn) writeLoopForPush() {
        var err error
        sta_buf := make([]byte,4)

        //send server id to status
        binary.BigEndian.PutUint32(sta_buf[:4],pc.cmdConns.t.s.ServerId)
        pc.cmdConns.t.logf("send server id %v",sta_buf[:4])
        _,err = pc.conn.Write(sta_buf[:4])
        if err != nil {
                pc.cmdConns.t.logf("err in send server id" + err.Error())
        }

        <- pc.notify_eof
        pc.cmdConns.t.logf("get notify eof signal %d",pc.cmdConns.cmd)
        return
}

func (pc *persistConn) spawnWriteLoop() {
        switch pc.cmdConns.cmd {

                case LOOP_ECHO:
                        go pc.writeLoopForUpstream()

                case LOOP_TOKEN_REST:
                        go pc.writeLoopForRestagent()

                case LOOP_STATUS:
                        if pc.cmdConns.status_is_slave {
                                go pc.writeLoopForStatusSlave()
                        } else {
                                go pc.writeLoopForStatus()
                        }

                case LOOP_PUSH:
                        go pc.writeLoopForPush()

        }       //end of switch
}

func (pc *persistConn) readLoopForUpstream() {
        pc.conn.(*net.TCPConn).SetReadBuffer(2*1024*1024)
        alive := true

        var upstream_response upstreampb_gogo.UpstreamResponse
        var msg_len uint32
        //TODO can we share buffer across different iteration?
        //TODO check for buffer overrun
        body_buf    := make([]byte,1024*1024)
        var local_buf []byte
        for alive {
                //read request from upstream
                        _ , err := io.ReadFull(pc.br,body_buf[:4])
                if err == nil {
                        msg_len =  binary.BigEndian.Uint32(body_buf[:4])
                                if debugTransport {
                                        pc.cmdConns.t.logf("readloop: read msg_len:%d",msg_len)
                                }
                        //TODO need review
                        if msg_len > 1024*1024 {
                                pc.cmdConns.t.logf("readloop: msg_len overflow 1M:%d",msg_len)
                                local_buf = make([]byte,msg_len)
                                _ , err = io.ReadFull(pc.br,local_buf[4:msg_len])
                                local_buf = nil
                                continue
                        }
                        _ , err = io.ReadFull(pc.br,body_buf[4:msg_len])
                }

                


                if err == nil {
                        //we use first 4 bytes of "body []byte" to encoding msg_len, don't overwrite!
                        upstream_response.Reset()
                        err = upstream_response.Unmarshal(body_buf[4:msg_len])
                        if err != nil {
                                pc.cmdConns.t.logf("readloop: unmarshal error:%s",err)
                        } else {
                                //TODO
                                //send back status change signal to tcp_server
                                err = pc.cmdConns.t.s.sendback(upstream_response.NetId,
                                                                upstream_response.ReqId,
                                                                []byte(upstream_response.Body),
                                                                false)

                                if err != nil {
                                        pc.cmdConns.t.logf("readloop: upstream:%s",err)
                                }
                                //TODO check error here
                                if debugTransport {
                                        pc.cmdConns.t.logf("read request from %s upstream: net_id %d req_id %d, err: %s\n",
                                        pc.cmdConns.ip_port,
                                        upstream_response.NetId,
                                        upstream_response.ReqId,err)
                                }
                        }
                } else if err == io.EOF {
                        alive = false
                } else if err == io.ErrUnexpectedEOF {
                        alive = false
                } else {
                        alive = false
                        //panic(fmt.Sprintf("Unknown error in readloop: %s",err))
                        pc.cmdConns.t.logf("Unknowned read error from %s: %s",pc.cmdConns.ip_port,err)
                }


        }  //end of LOOP_echo

        pc.close()
        pc.redialConn()
}


func (pc *persistConn) readLoopForRestagent() {
        pc.conn.(*net.TCPConn).SetReadBuffer(64*1024)
        alive := true

        var rtr restTokenRequest
        body_buf    := make([]byte,BUF_LEN_FOR_REST) 
        for alive {
                //read request from upstream
                err := rtr.read(pc.br,body_buf)

                if debugTransport {
                        pc.cmdConns.t.logf("read request from %d,%s upstream: %v, err: %s\n",pc.cmdConns.cmd,pc.cmdConns.ip_port,rtr,err)
                }

                if err == nil  {
                        //handle nil uid in tcp_server
                        err = pc.cmdConns.t.s.checkTokenResume(rtr.conn_id,rtr.uid)
                        rtr.reset()
                        if err != nil {
                                pc.cmdConns.t.logf("readloop: checkTokenResume:%s",err)
                        }
                } else if err == io.EOF {
                        alive = false
                } else if err == io.ErrUnexpectedEOF {
                        alive = false
                } else {
                        alive = false
                        //panic(fmt.Sprintf("Unknown error in readloop: %s",err))
                        pc.cmdConns.t.logf("Unknowned read error from %s: %s",pc.cmdConns.ip_port,err)
                }



        } //end of case LOOP_TOKEN_REST

        pc.close()
        pc.redialConn()
}

func (pc *persistConn) readLoopForStatus() {
        pc.conn.(*net.TCPConn).SetReadBuffer(4*1024)
        alive := true

        var status_response statuspb_gogo.StatusResponse
        var msg_len uint32

        body_buf    := make([]byte,256) 
        for alive {
                //read request from upstream

                _ , err := io.ReadFull(pc.br,body_buf[:4])
                if err == nil {
                        msg_len =  binary.BigEndian.Uint32(body_buf[:4])
                        _ , err = io.ReadFull(pc.br,body_buf[4:msg_len])
                }

                if err == nil  {
                        status_response.Reset()
                        err = status_response.Unmarshal(body_buf[4:msg_len])
                        if err != nil {
                                pc.cmdConns.t.logf("readloop: unmarshal error:%s",err)
                        } else {
                                //TODO
                                //send back status change signal to tcp_server
                                err = pc.cmdConns.t.s.statusResponseResume(status_response.NetId,status_response.RespType)
                                if err != nil {
                                        pc.cmdConns.t.logf("readloop: statusResume error:%s",err)
                                }
                                if debugTransport {
                                        pc.cmdConns.t.logf("read request from %d,%s upstream: %s, err: %s\n",
                                                                pc.cmdConns.cmd,
                                                                pc.cmdConns.ip_port,
                                                                status_response.String(),err)
                                }
                        }
                } else if err == io.EOF {
                        alive = false
                } else if err == io.ErrUnexpectedEOF {
                        alive = false
                } else {
                        alive = false
                        //panic(fmt.Sprintf("Unknown error in readloop: %s",err))
                        pc.cmdConns.t.logf("Unknowned read error from %s: %s",pc.cmdConns.ip_port,err)
                }



        } //end of case LOOP_STATUS

        pc.close()
        pc.redialConn()
}


func (pc *persistConn) readLoopForPush() {
        var preq    pushpb_gogo.PushRequestToDownstream
        pc.conn.(*net.TCPConn).SetReadBuffer(64*1024)
        alive := true

        var msg_len uint32

        body_buf    := make([]byte,1024 * 1024) 
        for alive {
                //read request from upstream

                _ , err := io.ReadFull(pc.br,body_buf[:4])
                if err == nil {
                        msg_len =  binary.BigEndian.Uint32(body_buf[:4])
                        _ , err = io.ReadFull(pc.br,body_buf[4:msg_len])
                }

                if err == nil  {
                        preq.Reset()
                        err = preq.Unmarshal(body_buf[4:msg_len])
                        if err != nil {
                                pc.cmdConns.t.logf("readloop: unmarshal error:%s",err)
                        } else {
                                //TODO
                                //send back status change signal to tcp_server
                                err = pc.cmdConns.t.s.sendback(preq.NetId,0,preq.Payload,true)
                                if err != nil {
                                        pc.cmdConns.t.logf("readloop: sendback error:%s",err)
                                }
                                if debugTransport {
                                        pc.cmdConns.t.logf("read request from %d,%s upstream: %s, err: %s\n",
                                                                pc.cmdConns.cmd,
                                                                pc.cmdConns.ip_port,
                                                                preq.String(),err)
                                }
                        }
                } else if err == io.EOF {
                        alive = false
                } else if err == io.ErrUnexpectedEOF {
                        alive = false
                } else {
                        alive = false
                        //panic(fmt.Sprintf("Unknown error in readloop: %s",err))
                        pc.cmdConns.t.logf("Unknowned read error from %s: %s",pc.cmdConns.ip_port,err)
                }



        } //end of case LOOP_STATUS

        pc.close()
        pc.redialConn()
}

func (pc *persistConn) spawnReadLoop() {
        switch pc.cmdConns.cmd {

                case LOOP_ECHO:
                        go pc.readLoopForUpstream()

                case LOOP_TOKEN_REST:
                        go pc.readLoopForRestagent()

                case LOOP_STATUS:
                        go pc.readLoopForStatus()

                case LOOP_PUSH:
                        go pc.readLoopForPush()

        } //end of switch
}

func (pc *persistConn) redialConn() (err error) {
        conn,err := pc.cmdConns.dial("tcp",pc.cmdConns.ip_port)
        for err != nil{
		time.Sleep( time.Duration(pc.exp_backoff_time) * time.Second)
                if pc.exp_backoff_time <= MAX_BACKOFF_TIME {
                        pc.exp_backoff_time *= 2
                }
                pc.cmdConns.t.logf("redial error: %s",err)
                conn,err = pc.cmdConns.dial("tcp",pc.cmdConns.ip_port)
        }

        pc.conn = conn
        pc.br.Reset(noteEOFReader{conn,pc.notify_eof})
        pc.bw.Reset(conn)
        //re spawn read/write loop
        pc.spawnReadLoop()
        pc.spawnWriteLoop()

        //WAITGROUP
        //if cmdUpstreamConnsNum == atomic.AddUint32(&pc.cmdConns.alive_conns,1) {
        //        if pc.cmdConns.status_is_slave == true {
        //                pc.cmdConns.t.status_slave_wg.Done()
        //        } else {
        //                pc.cmdConns.t.status_master_wg.Done()
        //        }

        //}


        if cmdUpstreamConnsNum == atomic.AddUint32(&pc.cmdConns.alive_conns,1) {

                if pc.cmdConns.cmd == LOOP_STATUS {
                        if pc.cmdConns.status_is_slave == false {
                                atomic.StoreUint32(&pc.cmdConns.t.status_master_alive,1)
                                pc.cmdConns.t.logf("set master alive to 1 %s",pc.cmdConns.ip_port)
                        } else {
                                atomic.StoreUint32(&pc.cmdConns.t.status_slave_alive,1)
                                pc.cmdConns.t.logf("set slave alive to 1 %s",pc.cmdConns.ip_port)
                        }
                        switch atomic.LoadUint32(&pc.cmdConns.t.status_state) {
                                case STATUS_STATE_MASTER:
                                case STATUS_STATE_SLAVE:
                                case STATUS_STATE_ERROR:
                                        if pc.cmdConns.status_is_slave == true {
                                                atomic.StoreUint32(&pc.cmdConns.t.status_state,STATUS_STATE_SLAVE)
                                                pc.cmdConns.t.logf("switch from error to slave %s",pc.cmdConns.ip_port)
                                        } else {
                                                atomic.StoreUint32(&pc.cmdConns.t.status_state,STATUS_STATE_MASTER)
                                                pc.cmdConns.t.logf("switch from error to master %s",pc.cmdConns.ip_port)
                                                
                                        }

                        }

                }
        }

        pc.cmdConns.t.logf("respawn %s",pc.cmdConns.ip_port)
        return nil
}
