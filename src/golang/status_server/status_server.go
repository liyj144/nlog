package main

import (
        "fmt"
        "io"
        "runtime"
        "os"
        "os/signal"
        "log"
        "net"
        "time"
        "sync"
        "sync/atomic"
        "encoding/binary"
        "../statuspb"
        "github.com/golang/protobuf/proto"
        "bufio"
        "strings"
        "flag"
)

var noDeadline = time.Time{}


var filename = flag.String("f", "", "configuration file name")
var logfile = flag.Bool("l", false, "log file bool value (overwrite nlog)")

//TODO add replication support

//debug flag for accept logic
const debugStatusServer = true

var upstream_dialer = net.Dialer{
                Timeout:   30 * time.Second,
                KeepAlive: 30 * time.Second,
                }


type userStatus struct {
app_version  uint16
device_id    string
net_id       uint32
server_id    uint32
timestamp    uint64
}


type anonymousStatus struct {
app_version  uint16
net_id       uint32
server_id    uint32
timestamp    uint64
}

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
        return tc, nil
}

type Server struct {
        get_count      uint32
        offline_count    uint32
        online_count     uint32

        anonymous_get_count      uint32
        anonymous_offline_count    uint32
        anonymous_online_count     uint32

        current_general_online      uint32
        current_anonymous_online    uint32


        alive_conn     uint32
        counter        uint32
        Addr           string 
        ErrorLog       *log.Logger    //log.Logger has internal locking to protect concurrent access

        mu             sync.RWMutex
        status_map     map[uint64][]*userStatus

        conn_mu        sync.RWMutex
        conn_map       map[uint32][]*conn

        anonymous_mu  sync.RWMutex
        anonymous_map map[string][]*anonymousStatus
       
        replication_conn_array []*net.TCPConn
        replication_write_chan chan statuspb.StatusRequest
        once            uint32
        can_replicate      uint32

        sti_mu          sync.RWMutex
        string_to_int   map[string]uint32
        sti_cnt         uint32

}

//setup flog facility
func (s *Server) SetupFlog() {
        logFile, err := os.OpenFile("./status.log", os.O_WRONLY | os.O_CREATE | os.O_APPEND, 0644)
        logFile.Reserve(0,1024*1024*256)
        if err != nil {
                panic("SetupFlog: error")
        }

        s.ErrorLog = log.New(logFile,"[status_server]: ",log.LstdFlags)

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

        s.ErrorLog = log.New(conn,"[status_server]: ",log.LstdFlags)
}


func (s *Server) logf(format string, args ...interface{}) {
	s.ErrorLog.Printf(format, args...)
}

func (srv *Server) mapStringToInt(app_version string) uint32{
        srv.sti_mu.RLock()
        //fast path
        if element,exist := srv.string_to_int[app_version];exist {
                srv.sti_mu.RUnlock()
                return element
        }
        srv.sti_mu.RUnlock()

        temp := atomic.AddUint32(&srv.sti_cnt,1)

        srv.sti_mu.Lock()
        if element,exist := srv.string_to_int[app_version];exist {
                srv.sti_mu.Unlock()
                return element
        } else {
                srv.string_to_int[app_version] = temp
                srv.sti_mu.Unlock()
                return temp
        }

}

//parse anonymous map
func (srv *Server) parseAnonymousItems() {
        var count uint32

        var pb_map statuspb.AnonymousMap

        srv.logf("reading anonymous data")

        persistFile, err := os.OpenFile("./anonymous.data", os.O_RDONLY,0)
        if err != nil {
                srv.logf("error in open anonymous data file %s",err)
                return
        }

        len_buf := make([]byte,8)
        _,err = io.ReadFull(persistFile,len_buf)
        if err != nil {
                srv.logf("error in reading anonymous data file %s",err)
                return
        }
        msg_len := binary.BigEndian.Uint64(len_buf)
        srv.logf("reading anonymous data file length %d",msg_len)

        body_buf := make([]byte,msg_len - 8)

        _,err = io.ReadFull(persistFile,body_buf)
        if err != nil {
                srv.logf("error in reading anonymous data file %s",err)
                return
        }

        err = proto.Unmarshal(body_buf[:msg_len-8],&pb_map)
        if err != nil {
                srv.logf("unmarshal error in reading anonymous data %s", err)
                return
        }

        srv.anonymous_mu.Lock()
        for _,e := range pb_map.Map {
                srv.anonymous_map[*e.Key] = make([]*anonymousStatus ,len(e.Values))
                for i,e1 := range e.Values {
                        count++
                        anonstatus := new(anonymousStatus)
                        anonstatus.app_version = uint16(*e1.AppVersion)
                        anonstatus.timestamp = *e1.Timestamp
                        anonstatus.server_id = *e1.ServerId
                        anonstatus.net_id = *e1.NetId
                        srv.anonymous_map[*e.Key][i] = anonstatus

                }
        }
        srv.anonymous_mu.Unlock()
        srv.logf("anonymous data count %d",count)
        persistFile.Close()

}

//persist anonymous map
func (srv *Server) persistAnonymousItems() {
        var count uint32
        var pb_map statuspb.AnonymousMap
        var kv *statuspb.AnonymousMap_KVAnonymousEntry
        srv.anonymous_mu.Lock()

        for k,v := range srv.anonymous_map {       
                kv = new(statuspb.AnonymousMap_KVAnonymousEntry)
                kv.Key = proto.String(k)
                for _,e := range v {
                count++
                kv.Values = append(kv.Values,
                        &statuspb.AnonymousMap_KVAnonymousEntry_AnonymousEntry{
                        AppVersion:proto.Uint32(uint32(e.app_version)),
                        Timestamp:proto.Uint64(e.timestamp),
                        NetId:proto.Uint32(e.net_id),
                        ServerId:proto.Uint32(e.server_id),

                        })
                }
                pb_map.Map = append(pb_map.Map,kv)
        }
        srv.anonymous_mu.Unlock()
        srv.logf("anonymous data count %d",count)

        data,err := proto.Marshal(&pb_map)

        if err != nil {

                srv.logf("persist thread marshal error %s",err)
                return
        }

        len_buf := make([]byte,8)
        binary.BigEndian.PutUint64(len_buf,8 + uint64(len(data)))

        srv.logf("writing anonymous data length %d %v",len(data),len_buf)

        persistFile, err := os.OpenFile("./anonymous.data", os.O_WRONLY | os.O_CREATE, 0644)
        if err != nil {
                srv.logf("error in open anonymous data file %s",err)
        }

        persistFile.Reserve(0,int64(len(data)+8))
        n,err := persistFile.Write(len_buf)
        if n != 8 {
                srv.logf("write rc from first round is not 8")
        }
        if err != nil {
                srv.logf("met err %s in writing anonymous length data",err)

        }

        n,err = persistFile.Write(data)
        if n != len(data) {
                srv.logf("write rc from first round is not %d",len(data))
        }
        if err != nil {
                srv.logf("met err %s in writing encoded anonymous data",err)

        }
        persistFile.Close()

}

//parse STI 
func (srv *Server) parseSTI() {
        var sti_map statuspb.StringToIntMap

        srv.logf("reading sti data")

        persistFile, err := os.OpenFile("./sti.data", os.O_RDONLY,0)
        if err != nil {
                srv.logf("error in open sti data file %s",err)
                return
        }

        len_buf := make([]byte,8)
        _,err = io.ReadFull(persistFile,len_buf)
        if err != nil {
                srv.logf("error in reading sti data file %s",err)
                return
        }
        msg_len := binary.BigEndian.Uint64(len_buf)

        body_buf := make([]byte,msg_len - 8)

        _,err = io.ReadFull(persistFile,body_buf)
        if err != nil {
                srv.logf("error in reading sti data file %s",err)
                return
        }

        err = proto.Unmarshal(body_buf[:msg_len-8],&sti_map)
        if err != nil {
                srv.logf("unmarshal error in reading sti data %s", err)
                return
        }

        srv.sti_mu.Lock()
        for _,e := range sti_map.Map {
                srv.logf("unmarshal result in reading sti data %s", e.String())
                srv.string_to_int[*e.Str] = *e.Intger

        }
        srv.sti_mu.Unlock()
        persistFile.Close()
}

//persist map string to int
func (srv *Server) persistSTI() {
        var count uint32
        var sti_map statuspb.StringToIntMap
        srv.sti_mu.Lock()
        for k,v := range srv.string_to_int {
                count++
                sti_map.Map = append(sti_map.Map,
                        &statuspb.StringToIntMap_STI{
                        Str:proto.String(k),
                        Intger:proto.Uint32(v),
                        })
        }
        srv.sti_mu.Unlock()
        srv.logf("sti item counts %d",count)


        data,err := proto.Marshal(&sti_map)
        if err != nil {

                srv.logf("persist thread marshal error %s",err)
                return
        }

        len_buf := make([]byte,8)
        binary.BigEndian.PutUint64(len_buf,8 + uint64(len(data)))

        srv.logf("writing sti data length %d",len(data))

        persistFile, err := os.OpenFile("./sti.data", os.O_WRONLY | os.O_CREATE, 0644)
        if err != nil {
                srv.logf("error in open sti data file %s",err)
        }

        persistFile.Reserve(0,int64(len(data)+8))
        n,err := persistFile.Write(len_buf)
        if n != 8 {
                srv.logf("write rc from first round is not 8")
        }
        if err != nil {
                srv.logf("met err %s in writing sti length data",err)

        }

        n,err = persistFile.Write(data)
        if n != len(data) {
                srv.logf("write rc from first round is not %d",len(data))
        }
        if err != nil {
                srv.logf("met err %s in writing encoded sti data",err)

        }
        persistFile.Close()

}

//parse items
func (srv *Server) parseItems() {
        var count uint32

        var pb_map statuspb.StatusMap

        srv.logf("reading status data")

        persistFile, err := os.OpenFile("./status.data", os.O_RDONLY,0)
        if err != nil {
                srv.logf("error in open status data file %s",err)
                return
        }

        len_buf := make([]byte,8)
        _,err = io.ReadFull(persistFile,len_buf)
        if err != nil {
                srv.logf("error in reading status data file %s",err)
                return
        }
        msg_len := binary.BigEndian.Uint64(len_buf)

        body_buf := make([]byte,msg_len - 8)

        _,err = io.ReadFull(persistFile,body_buf)
        if err != nil {
                srv.logf("error in reading status data file %s",err)
                return
        }

        err = proto.Unmarshal(body_buf[:msg_len-8],&pb_map)
        if err != nil {
                srv.logf("unmarshal error in reading status data %s", err)
                return
        }

        fmt.Println(pb_map.String())
        var userstatus *userStatus
        srv.mu.Lock()
        for _,e := range pb_map.Map {
                srv.status_map[*e.Key] = make([]*userStatus,len(e.Values))
                for i,e1 := range e.Values {
                        count++
                        userstatus = new(userStatus)
                        userstatus.device_id = *e1.DeviceId
                        userstatus.timestamp = *e1.Timestamp
                        userstatus.net_id = *e1.NetId
                        userstatus.server_id = *e1.ServerId
                        userstatus.app_version = uint16(*e1.AppVersion)
                        srv.status_map[*e.Key][i] = userstatus
                }

        }
        srv.mu.Unlock()

        srv.logf("status data count %d",count)
        persistFile.Close()
}

//persist data to disk
func (srv *Server) persistItems() {
        var count uint32

        var pb_map statuspb.StatusMap
        var kv *statuspb.StatusMap_KVUserStatus
        //acquire lock                
        srv.mu.Lock()
        for k,v := range srv.status_map {
                kv = new(statuspb.StatusMap_KVUserStatus)
                kv.Key = proto.Uint64(k)
                for _,e := range v {
                        count++
                        kv.Values = append(kv.Values,
                                        &statuspb.StatusMap_KVUserStatus_UserStatus{
                                        DeviceId:proto.String(e.device_id),
                                        Timestamp:proto.Uint64(e.timestamp),
                                        ServerId:proto.Uint32(e.server_id),
                                        NetId:proto.Uint32(e.net_id),
                                        AppVersion:proto.Uint32(uint32(e.app_version)),
                                        })

                }

                pb_map.Map = append(pb_map.Map,kv)

        }
        
        //release lock
        srv.mu.Unlock()

        srv.logf("status data count %d",count)
        data,err := proto.Marshal(&pb_map)
        if err != nil {

                srv.logf("persist thread marshal error %s",err)
                return
        }

        len_buf := make([]byte,8)
        binary.BigEndian.PutUint64(len_buf,8 + uint64(len(data)))

        srv.logf("writing status data length %d",len(data))

        persistFile, err := os.OpenFile("./status.data", os.O_WRONLY | os.O_CREATE, 0644)
        if err != nil {
                srv.logf("error in open status data file %s",err)
        }

        persistFile.Reserve(0,int64(len(data)+8))
        n,err := persistFile.Write(len_buf)
        if n != 8 {
                srv.logf("write rc from first round is not 8")
        }
        if err != nil {
                srv.logf("met err %s in writing status length data",err)

        }

        n,err = persistFile.Write(data)
        if n != len(data) {
                srv.logf("write rc from first round is not %d",len(data))
        }
        if err != nil {
                srv.logf("met err %s in writing encoded status data",err)

        }
        persistFile.Close()
}


//background thread wakeup at 2am to purge expire item
func (srv *Server) purgeExipireItems() {
        //setup init sleep time
        h := time.Now().Hour()

        var ms_per_day int64
        ms_per_day = 24 * 60 * 60 * 1000

        if h <= 2 {
                h1 := 2-h
                srv.logf("purge thread sleep for %d hrs",h1)
                time.Sleep(time.Duration(h1)*time.Hour)
        } else {
                h1 := 26-h
                srv.logf("purge thread sleep for %d hrs",h1)
                time.Sleep(time.Duration(h1)*time.Hour)
        }

        var purged_items int
        for {
                

                current_ts := time.Now().UnixNano()/1e6

                srv.logf("start purge general map")

                var found bool
                //acquire lock                
                srv.mu.Lock()
                for k,v := range srv.status_map {
                        found = false
                        for i,e := range v {

                                if current_ts - int64(e.timestamp) > ms_per_day {
                                        found = true
                                        //delete item
                                        v[i], v[len(v)-1], v =
                                        v[len(v)-1], nil, v[:len(v)-1]

                                        purged_items ++

                                        srv.logf("purged uid:%d,ts:%d, dev_id: %s, server_id:%d",
                                        k,
                                        e.timestamp,
                                        e.device_id,
                                        e.server_id)
                                }

                        }

                        if found == true {
                        if len(v) == 0 {
                                delete(srv.status_map , k)
                        } else {
                                srv.status_map[k] = v
                        
                        }
                        }
                }
        
                //release lock
                srv.mu.Unlock()

                srv.logf("finish purge general map, purges :%d",purged_items)

                purged_items = 0
                srv.logf("start purge anonymous map")
                srv.anonymous_mu.Lock()
                for k,v := range srv.anonymous_map {
                        found = false

                        for i,e := range v {
                                if current_ts - int64(e.timestamp) > ms_per_day {
                                        found = true
                                        //delete item
                                        v[i], v[len(v)-1], v =
                                        v[len(v)-1], nil, v[:len(v)-1]

                                        //if v != srv.anonymous_map[k] {
                                        //        srv.logf("meet unequal situation for %s",k)
                                        //        srv.anonymous_map[k] = v
                                        //}

                                        purged_items ++
                                        srv.logf("purged dev_id: %s,ts:%d, server_id:%d",
                                                k,
                                                e.timestamp,
                                                e.server_id)
                                }
                        }

                        if found == true {
                        if len(v) == 0 {
                                delete(srv.anonymous_map , k)
                        } else {
                                srv.anonymous_map[k] = v
                        
                        }
                        }

                }
                srv.anonymous_mu.Unlock()
                srv.logf("finish purge anonymous map, purges :%d",purged_items)
                time.Sleep(24 * time.Hour)
        }
}


func (srv *Server) RegisterReplicationServer(ip_port string) error {
        srv.logf("register  replication server %s",ip_port)
        c , err := upstream_dialer.Dial("tcp",ip_port)
        if err != nil {
                panic("dial error in connecting upstream replication server")
        }
        //TODO add replication here
        //send sid to other status server uint32(-2)
        sid_buf := []byte{0,0,0,0}
        binary.BigEndian.PutUint32(sid_buf,^uint32(1))
        srv.logf("send server id %v",sid_buf)
        _,err = c.Write(sid_buf)
        if err != nil {
                srv.logf("err in send server id" + err.Error())
                c.Close()
                panic(err)
        }

        if srv.replication_write_chan == nil {
                srv.replication_write_chan = make(chan statuspb.StatusRequest,128)
        }
        
        srv.replication_conn_array = append(srv.replication_conn_array , c.(*net.TCPConn))

        if atomic.LoadUint32(&srv.once) == 0 {
                go srv.writeToReplicationServer()
                atomic.StoreUint32(&srv.once, 1)
        }
        atomic.StoreUint32(&srv.can_replicate, 1)
        go srv.readFromReplicationServer(c.(*net.TCPConn))
        return nil
}

func (srv *Server) writeToReplicationServer()  {
        var replr statuspb.StatusRequest
        var msg_len uint32
        buf := make([]byte,1024)
        for {
                replr = <- srv.replication_write_chan
                srv.conn_mu.RLock()
                if atomic.LoadUint32(&srv.can_replicate) == 1 {
                        data,err := proto.Marshal(&replr)
                        if err != nil {
                                srv.logf("writeReplicationServer: mar err %s",err)
                                continue
                        }
                        msg_len = uint32(len(data)) + 4
                        if msg_len > 1024 - 4 {
                                srv.logf("writeReplicationServer: copy err buf overflow")
                                continue
                        }
                        binary.BigEndian.PutUint32(buf[:4],msg_len)
                        copy(buf[4:],data)
                        for _,e := range srv.replication_conn_array {
                                //DEBUG
                                //fmt.Println(len(srv.replication_conn_array))
                                _,err = e.Write(buf[:msg_len])
                                if err != nil {
                                        srv.logf("writeReplicationServer: %s write err %s",e.RemoteAddr(),err)
                                }
                        }
                } else {
                        srv.logf("not avail")
                }
                srv.conn_mu.RUnlock()
        }
}

//TODO add redial
func (srv *Server) readFromReplicationServer(c *net.TCPConn) {
        var err error
        buf := make([]byte,4)
        for err == nil {
                _,err = io.ReadFull(c,buf)
                srv.logf("readReplicationServer: %s write err %s",c.RemoteAddr().String(),buf)
        }

        //delete element
        srv.conn_mu.Lock()
        for i,e := range srv.replication_conn_array{
                if e == c {
                srv.replication_conn_array[i], 
                srv.replication_conn_array[len(srv.replication_conn_array)-1],
                srv.replication_conn_array = 
                srv.replication_conn_array[len(srv.replication_conn_array)-1], 
                nil, 
                srv.replication_conn_array[:len(srv.replication_conn_array)-1]
                }
        }
        
        if len(srv.replication_conn_array) == 0 {
                srv.logf("readReplicationServer: set can_replication to 0")
                atomic.StoreUint32(&srv.can_replicate, 0)
        }
        srv.conn_mu.Unlock()

        //redial here
        var nc net.Conn
        for {
                nc , err = upstream_dialer.Dial("tcp",c.RemoteAddr().String())
                if err != nil {
                        srv.logf("dial error in redial %s,%s",c.RemoteAddr().String(),err)
                        time.Sleep(2*time.Second)
                } else {
                        //send sid to other status server uint32(-2)
                        sid_buf := []byte{0,0,0,0}
                        binary.BigEndian.PutUint32(sid_buf,^uint32(1))
                        srv.logf("send server id %v",sid_buf)
                        _,err = nc.Write(sid_buf)
                        if err != nil {
                                srv.logf("err in send server id" + err.Error())
                                nc.Close()
                                continue
                        }
                        break
                }
        }
        srv.conn_mu.Lock()
        srv.replication_conn_array = append(srv.replication_conn_array , nc.(*net.TCPConn))
        srv.conn_mu.Unlock()

        atomic.StoreUint32(&srv.can_replicate, 1)
        go srv.readFromReplicationServer(nc.(*net.TCPConn))

}

func (srv *Server) ListenAndServe() error {
        //TODO: comment out in production env
        go func(){for {fmt.Printf("%d,%d,%d,%d,%d\n",srv.counter,srv.alive_conn,len(srv.conn_map),srv.current_anonymous_online,srv.current_general_online);time.Sleep(20*time.Second)}}()

        c := make(chan os.Signal, 1)                                       
        signal.Notify(c, os.Interrupt)                                     
        go func() {                                                        
        sig := <- c
        srv.logf("captured %v, stopping status_server persisting and exiting", sig)
        srv.persistAnonymousItems()
        srv.persistSTI()
        srv.persistItems()
        os.Exit(1)                                                     
        }()    


        // register error log to stderr
        if srv.ErrorLog == nil {
                srv.ErrorLog = log.New(os.Stderr, "[status_server]: ", log.LstdFlags)
        }


        if srv.status_map == nil {
                srv.status_map = make(map[uint64][]*userStatus,10000)
        }

        if srv.conn_map == nil {
                srv.conn_map = make(map[uint32][]*conn,100)
        }

        if srv.anonymous_map == nil {
                srv.anonymous_map = make(map[string][]*anonymousStatus,10000)
        }

        if srv.string_to_int == nil {
                srv.string_to_int = make(map[string]uint32,100)
        }

        srv.parseAnonymousItems()
        srv.parseSTI()
        srv.parseItems()

        srv.logf("listening: %s",srv.Addr)
        if srv.Addr == "" {
                //addr = ":3541"    //Default port 3541
                panic("please specify listen addr")
        }


        ln, err := net.Listen("tcp", srv.Addr)
        if err != nil {
                return err
        }

        //start  purge background thread
        go srv.purgeExipireItems()

        return srv.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})
}

func (srv *Server) Serve(l net.Listener) error {
        var tempDelay time.Duration // how long to sleep on accept failure
        alive := true

        //go func(){time.Sleep(300*time.Second);alive = false}()

        for alive {
                rw, e := l.Accept()

                if debugStatusServer {
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
                                srv.logf("http: Accept error: %s; retrying in %v", e, tempDelay)
                                time.Sleep(tempDelay)
                                continue
                        }
                        return e
                }
                tempDelay = 0

                //increment alive connection counter
                atomic.AddUint32(&srv.alive_conn,1)

                conn_id := atomic.AddUint32(&srv.counter,1)

                e = srv.newConn(rw,conn_id)
                if e != nil {
                        srv.logf("err in new conn" + e.Error())
                        rw.Close()
                }
        }
        l.Close()
        return nil
}


func (srv *Server) newConn(rwc net.Conn,conn_id uint32) (err error) {
        c := new(conn)
        c.remoteAddr = rwc.RemoteAddr().String()
        c.server = srv
        c.conn_id = conn_id
        c.rwc = rwc
        c.now_msec = time.Now().UnixNano()/1e6
        c.GeneralRespWriteChan = make(chan statuspb.StatusResponse,1000)
        c.QueryRespWriteChan = make(chan statuspb.StatusQueryResponse,1000)
        
        var sid uint32
        server_id_buf := make([]byte,4)

        //register mapping between server_id and net_id
        rwc.(*net.TCPConn).SetReadDeadline(time.Now().Add(5500*time.Millisecond))
        _ , err = io.ReadFull(rwc,server_id_buf)
        rwc.(*net.TCPConn).SetReadDeadline(noDeadline)

        if err == nil {
                sid = binary.BigEndian.Uint32(server_id_buf)
                srv.logf("decode sid %d from %s",sid,c.remoteAddr)

        } else {
                return err

        }
        c.server_id = sid



        //add conn map
        srv.conn_mu.Lock()
        element := srv.conn_map[sid]
        srv.conn_map[sid] = append(element,c)
        srv.conn_mu.Unlock()
        go c.serve()
        go c.writeloop()
        return nil
}


type conn struct {
        now_msec   int64                // time stamp in msec
        conn_id    uint32               // connection id
        remoteAddr string               // network address of remote side
        server_id      uint32
        server     *Server              // the Server on which the connection arrived
        rwc        net.Conn             // i/o connection
        QueryRespWriteChan  chan statuspb.StatusQueryResponse
        GeneralRespWriteChan  chan statuspb.StatusResponse
        

        msg_count  uint32
}


// Close the connection.
func (c *conn) close() {
        if debugStatusServer {
                c.server.logf("status_server:closing: %s, conn_id: %d",c.remoteAddr,int(c.conn_id))
        }

        //del conn map
        c.server.conn_mu.Lock()
        element := c.server.conn_map[c.server_id]
        for i, s := range element {
                if s == c {
                        if debugStatusServer {
                                c.server.logf("status_server:close delete: %s, conn: %v",c.remoteAddr,c)
                        }
                        //delete slice element here
                        element[i], element[len(element)-1], element = element[len(element)-1], nil, element[:len(element)-1]
                        //element[i] = nil
                        //element := append(element[:i],element[i+1:]...)
                        if len(element) == 0 {
                                delete(c.server.conn_map , c.server_id)
                        } else {
                                c.server.conn_map[c.server_id] = element
                        }
                        break
                }
        }
        c.server.conn_mu.Unlock()

        close(c.GeneralRespWriteChan)
        close(c.QueryRespWriteChan)
 
        //decrement counter
        atomic.AddUint32(&c.server.alive_conn, ^uint32(0))

        //c.finalFlush()
        if c.rwc != nil {
                c.rwc.Close()
                c.rwc = nil
        }
}

func (c *conn) handleAnonymousOnlineMessage(sreq statuspb.StatusRequest) {
        var net_id uint32
        var timestamp uint64
        var anonymous_status *anonymousStatus
        var device_id   string
        var appversion   uint16


        var resp statuspb.StatusResponse
        //get net_id
        net_id =  sreq.GetNetId()

        //get timestamp
        timestamp = sreq.GetTimestamp()

        appversion = uint16(c.server.mapStringToInt(*sreq.AppVersion))

        device_id = sreq.GetDeviceId()

        if debugStatusServer {
        c.server.logf("net_id:%d,ts:%d,dev_id:%s",net_id,timestamp,device_id)

        }

        var found bool

        c.server.anonymous_mu.Lock()

        if element,exist := c.server.anonymous_map[device_id];exist {
                found = false

                //no need to use index here, we only perform update not insert
                for _, s := range element {
                        if s.app_version == appversion {
                                found = true
                                if s.timestamp > timestamp {

                                        if *sreq.ReqType == statuspb.StatusRequest_ONLINE {
                                                //force current user off line
                                                resp.RespType = statuspb.StatusResponse_FORCE_OFFLINE.Enum()
                                                resp.NetId = proto.Uint32(net_id)


                                                //check for blocking chan here
                                                if len(c.GeneralRespWriteChan) < cap(c.GeneralRespWriteChan){
                                                c.GeneralRespWriteChan <- resp
                                                } else {
                                                c.server.logf("channel full for %s",c.remoteAddr)
                                                }
                                        }

                                        if debugStatusServer {
                                        c.server.logf("device :%s: current ts %d smaller than ts in map %d, appv: %s forced current conn offline",
                                                        device_id,
                                                        timestamp,
                                                        s.timestamp,
                                                        appversion)

                                        }
                                        
                                } else {
                                        if *sreq.ReqType == statuspb.StatusRequest_ONLINE {
                                                //force old user offline, update item

                                                resp.RespType = statuspb.StatusResponse_OK.Enum()
                                                resp.NetId = proto.Uint32(net_id)

                                                //check for blocking chan here
                                                if len(c.GeneralRespWriteChan) < cap(c.GeneralRespWriteChan){
                                                c.GeneralRespWriteChan <- resp
                                                } else {
                                                c.server.logf("channel full for %s",c.remoteAddr)
                                                }

                                                //acquire read lock here
                                                c.server.conn_mu.RLock()
                                                conn_element := c.server.conn_map[s.server_id]
                                                if len(conn_element) == 0 {
                                                        c.server.logf("unable to send back off line signal, no online server:%s",
                                                                        c.remoteAddr)

                                                } else {
                                                        resp.RespType = statuspb.StatusResponse_FORCE_OFFLINE.Enum()
                                                        resp.NetId = proto.Uint32(s.net_id)
                                                        //check for blocking chan here
                                                        if len(conn_element[0].GeneralRespWriteChan) < cap(conn_element[0].GeneralRespWriteChan){
                                                                conn_element[0].GeneralRespWriteChan <- resp
                                                        } else {
                                                                c.server.logf("channel full for %s",conn_element[0].remoteAddr)
                                                        }
                                                }
                                                c.server.conn_mu.RUnlock()                        
                                        }


                                        s.app_version = appversion
                                        s.timestamp = timestamp
                                        s.net_id = net_id

                                        s.server_id = *sreq.ServerId

                                        
                                        if debugStatusServer {
                                                        c.server.logf("dev_id: %s,current ts %d larger than ts in map %d,appv: %s, forced  conn in map offline",
                                                                        device_id,timestamp,s.timestamp,appversion)
                                        }
                                }
                                break
                }
                }




        }

        if found == false {
                //not exist,add item
                anonymous_status = new(anonymousStatus)
                anonymous_status.app_version = appversion
                anonymous_status.timestamp = timestamp
                anonymous_status.net_id = net_id

                anonymous_status.server_id = *sreq.ServerId

                c.server.anonymous_map[device_id] = append(c.server.anonymous_map[device_id],
                                                        anonymous_status)


                atomic.AddUint32(&c.server.current_anonymous_online, 1)

                if debugStatusServer {
                        c.server.logf("repl: %d map item not exist for dev_id %s, append item %v",*sreq.ReqType,device_id,anonymous_status)
                }
                if *sreq.ReqType == statuspb.StatusRequest_ONLINE {
                        //send ok response
                        resp.RespType = statuspb.StatusResponse_OK.Enum()
                        resp.NetId = proto.Uint32(net_id)

                        //check for blocking chan here
                        if len(c.GeneralRespWriteChan) < cap(c.GeneralRespWriteChan){
                                c.GeneralRespWriteChan <- resp
                        } else {
                                c.server.logf("channel full for %s",c.remoteAddr)
                        }

                }
        }
        c.server.anonymous_mu.Unlock()


}

func (c *conn) handleGeneralOnlineMessage(sreq statuspb.StatusRequest) {
        var net_id uint32
        var timestamp uint64
        var userstatus *userStatus
        var user_id   uint64
        var device_id   string
        var app_version   uint16


        var resp statuspb.StatusResponse
        //get net_id
        net_id =  sreq.GetNetId()
        //get uid
        user_id = sreq.GetUid()

        //get timestamp
        timestamp = sreq.GetTimestamp()

        device_id = sreq.GetDeviceId()

        //map string to int
        app_version = uint16(c.server.mapStringToInt(sreq.GetAppVersion()))

        if debugStatusServer {
        c.server.logf("net_id:%d,uid:%d,ts:%d,dev_id:%s, app_version: %s",net_id,user_id,timestamp,device_id,sreq.GetAppVersion())

        }
        
        var found bool

        c.server.mu.Lock()
        if element,exist := c.server.status_map[user_id];exist {
                found = false

                //no need to use index here, we only perform update not insert
                for _, s := range element {
                        if s.device_id == device_id && s.app_version == app_version {
                                found = true
                                if s.timestamp > timestamp {

                                        if *sreq.ReqType == statuspb.StatusRequest_ONLINE {
                                                //sendback reject response
                                                resp.RespType = statuspb.StatusResponse_FORCE_OFFLINE.Enum()
                                                resp.NetId = proto.Uint32(net_id)
                                                //check for blocking chan here
                                                if len(c.GeneralRespWriteChan) < cap(c.GeneralRespWriteChan){
                                                c.GeneralRespWriteChan <- resp
                                                } else {
                                                c.server.logf("channel full for %s",c.remoteAddr)
                                                }

                                        }
                                        if debugStatusServer {
                                        c.server.logf("current ts %d smaller than ts in map %d, forced current conn offline",
                                                        timestamp,
                                                        s.timestamp)
                                        }
                                } else {
                                        if *sreq.ReqType == statuspb.StatusRequest_ONLINE {
                                                //send ok response, force old device offline, update entry
                                                        resp.RespType = statuspb.StatusResponse_OK.Enum()
                                                        resp.NetId = proto.Uint32(net_id)

                                                //check for blocking chan here
                                                if len(c.GeneralRespWriteChan) < cap(c.GeneralRespWriteChan){
                                                c.GeneralRespWriteChan <- resp
                                                } else {
                                                c.server.logf("channel full for %s",c.remoteAddr)
                                                }
                                                //acquire read lock here
                                                c.server.conn_mu.RLock()
                                                conn_element := c.server.conn_map[s.server_id]
                                                if len(conn_element) == 0 {
                                                        c.server.logf("unable to send back off line signal, no online server:%s",
                                                                        c.remoteAddr)

                                                } else {
                                                        resp.RespType = statuspb.StatusResponse_FORCE_OFFLINE.Enum()
                                                        resp.NetId = proto.Uint32(s.net_id)
                                                        //check for blocking chan here
                                                        if len(conn_element[0].GeneralRespWriteChan) < cap(conn_element[0].GeneralRespWriteChan){
                                                        conn_element[0].GeneralRespWriteChan <- resp
                                                        } else {
                                                        c.server.logf("channel full for %s",conn_element[0].remoteAddr)
                                                        }
                                                }

                                                c.server.conn_mu.RUnlock()
                                        }

                                        s.net_id = net_id
                                        s.device_id = device_id
                                        s.timestamp = timestamp
                                        s.server_id = *sreq.ServerId
                                        s.app_version = app_version
                                        if debugStatusServer {
                                        c.server.logf("current ts %d larger than ts in map %d, forced  conn in map offline",
                                                        timestamp,s.timestamp)
                                        }
                                }
                                break
                        }
                }

        }

        if found == false {
                atomic.AddUint32(&c.server.current_general_online, 1)
                //not exist
                //new a object
                userstatus = new(userStatus)
                userstatus.device_id = device_id
                userstatus.timestamp = timestamp
                userstatus.net_id = net_id

                        userstatus.server_id = *sreq.ServerId

                userstatus.app_version = app_version
                //TODO assign remote addr here
                c.server.status_map[user_id] = append(c.server.status_map[user_id],userstatus)
                if debugStatusServer {
                c.server.logf("repl: %d,map item not exist for uid %d, append item %v",*sreq.ReqType,user_id,userstatus)
                }

                if *sreq.ReqType == statuspb.StatusRequest_ONLINE {
                        //send ok response
                        resp.RespType = statuspb.StatusResponse_OK.Enum()
                        resp.NetId = proto.Uint32(net_id)

                        //check for blocking chan here
                        if len(c.GeneralRespWriteChan) < cap(c.GeneralRespWriteChan){
                                c.GeneralRespWriteChan <- resp
                        } else {
                                c.server.logf("channel full for %s",c.remoteAddr)
                        }
                }

        }
        c.server.mu.Unlock()



}


func (c *conn) handleAnonymousOfflineMessage(sreq statuspb.StatusRequest) {
        var device_id   string
        var appversion  uint16


        device_id = sreq.GetDeviceId()

        appversion = uint16(c.server.mapStringToInt(sreq.GetAppVersion()))

        if debugStatusServer {
                c.server.logf("offline: dev_id:%s",device_id)

        }

        c.server.anonymous_mu.Lock()

        if element,exist := c.server.anonymous_map[device_id];exist {
                found := false

                for i, s := range element {
                        if s.app_version == appversion{
                                found = true
                                //delete slice element here
                                element[i], element[len(element)-1], element =
                                element[len(element)-1], nil, element[:len(element)-1]
                                atomic.AddUint32(&c.server.current_anonymous_online, ^uint32(0))

                                break
                
                        }
                }

                if found == true {
                        if len(element) == 0 {
                        delete(c.server.anonymous_map , device_id)
                        } else {
                        c.server.anonymous_map[device_id] = element
                        
                        }        
                } else {

                 c.server.logf("offline: not found anonymous dev_id:%s,not found appv: %d",device_id,appversion)

                }
                
        } else {
                c.server.logf("offline: not found dev_id:%s",device_id)
        }
        c.server.anonymous_mu.Unlock()
        
}

func (c *conn) handleGeneralOfflineMessage(sreq statuspb.StatusRequest) {

        var app_version uint16
        var user_id   uint64
        var device_id   string


        //get uid
        user_id = sreq.GetUid()

        device_id = sreq.GetDeviceId()

        app_version = uint16(c.server.mapStringToInt(sreq.GetAppVersion()))

        if debugStatusServer {
        c.server.logf("offline: uid:%d,dev_id:%s, app_ver: %s",user_id,device_id,sreq.GetAppVersion())

        }
        c.server.mu.Lock()
        if element,exist := c.server.status_map[user_id];exist {
                found := false

                //no need to use index here, we only perform update not insert
                for i, s := range element {
                        if s.device_id == device_id && s.app_version == app_version{
                                found = true
                                //delete slice element here
                                element[i], element[len(element)-1], element =
                                element[len(element)-1], nil, element[:len(element)-1]

                                atomic.AddUint32(&c.server.current_general_online, ^uint32(0))
                                //element[i] = nil
                                //element = append(element[:i],element[i+1:]...)

                                if debugStatusServer {
                                c.server.logf("delete ok for uid:%d,dev_id:%s",user_id,device_id)

                                }

                                break
                        }

                }

                if found == true {
                                if len(element) == 0 {

                                delete(c.server.status_map , user_id)

                                } else {
                                c.server.status_map[user_id] = element
                                }

                } else {
                        c.server.logf("uid:%d  not found, device_id not found: %s",user_id,device_id)
                }
        } else {
                c.server.logf("uid:%d not found",user_id)
        }
        c.server.mu.Unlock()
}


func (c *conn) handleGeneralGetMessage(sreq statuspb.StatusRequest) {
        var user_id uint64
        var resume_net_id uint32

        resume_net_id = *sreq.ResumeNetId
        //get uid
        user_id = sreq.GetUid()


        c.server.mu.RLock()
        if element,exist := c.server.status_map[user_id];exist {
                if len(element) > 0 {

                        pb_obj := statuspb.StatusQueryResponse{
                        Resp:statuspb.StatusQueryResponse_FOUND.Enum(),
                        ResumeNetId:proto.Uint32(resume_net_id),
                        }
                        

                        for _, s := range element {
                                pb_obj.Result = append(pb_obj.Result,
                                &statuspb.StatusQueryResult{
                                        RemoteNetId :proto.Uint32(s.net_id),
                                        Ip : proto.Uint32(s.server_id),
                                })

                        }


                                //send to chan here
                                if len(c.QueryRespWriteChan) < cap(c.QueryRespWriteChan){
                                c.QueryRespWriteChan <- pb_obj
                                } else {
                                c.server.logf("channel full for %s",c.remoteAddr)
                                }

                } else {
                //should not be here
                // when len is zero, map item should be deleted at offline logic
                        c.server.logf("cmd_get found element of 0 length",user_id)
                }
                c.server.mu.RUnlock()

        } else {
                c.server.mu.RUnlock()
                        pb_obj := statuspb.StatusQueryResponse{
                        Resp:statuspb.StatusQueryResponse_NOT_FOUND.Enum(),
                        ResumeNetId:proto.Uint32(resume_net_id),
                        }
                                //send to chan here
                                if len(c.QueryRespWriteChan) < cap(c.QueryRespWriteChan){
                                c.QueryRespWriteChan <- pb_obj
                                } else {
                                c.server.logf("channel full for %s",c.remoteAddr)
                                }
                c.server.logf("cmd_get not found element for %d",user_id)
        }

}


func (c *conn) handleAnonymousGetMessage(sreq statuspb.StatusRequest) {

        var device_id   string
        var resume_net_id uint32


        device_id = sreq.GetDeviceId()

        if debugStatusServer {
                c.server.logf("get: dev_id:%s",device_id)

        }

        c.server.anonymous_mu.RLock()

        if element,exist := c.server.anonymous_map[device_id];exist {
                pb_obj := statuspb.StatusQueryResponse{
                        Resp:statuspb.StatusQueryResponse_FOUND.Enum(),
                        ResumeNetId:proto.Uint32(resume_net_id),
                }        
                
                for _, s := range element {
                pb_obj.Result = append(pb_obj.Result,
                                &statuspb.StatusQueryResult{
                                        RemoteNetId :proto.Uint32(s.net_id),
                                        Ip : proto.Uint32(s.server_id),
                                })
                }

                        //send to chan here
                        if len(c.QueryRespWriteChan) < cap(c.QueryRespWriteChan){
                        c.QueryRespWriteChan <- pb_obj
                        } else {
                        c.server.logf("channel full for %s",c.remoteAddr)
                        }

        } else {
                        pb_obj := statuspb.StatusQueryResponse{
                        Resp:statuspb.StatusQueryResponse_NOT_FOUND.Enum(),
                        ResumeNetId:proto.Uint32(resume_net_id),
                        }

                                //send to chan here
                                if len(c.QueryRespWriteChan) < cap(c.QueryRespWriteChan){
                                c.QueryRespWriteChan <- pb_obj
                                } else {
                                c.server.logf("channel full for %s",c.remoteAddr)
                                }
                c.server.logf("get: not found dev_id:%s",device_id)
        }
        c.server.anonymous_mu.RUnlock()

}


func (c *conn) serve() {
        defer func() {
                if err := recover(); err != nil {
                        c.server.logf("panic serving %s: %s\n", c.remoteAddr, err)
                        panic("panic")
                }
                c.close()
        }()
        

        alive := true
        var msg_len uint32
        msg_len_buf := []byte{0,0,0,0}   // 32bits buffer for msg_len
        body_buf    := make([]byte,4096)
        for alive {
                //read 4 bytes msg_len
                rc,err := io.ReadFull(c.rwc,msg_len_buf)
                if debugStatusServer {
                        c.server.logf("read msg_len: %v,rc: %d, err: %s, from: %s, conn_id %d",msg_len_buf,rc,err,c.remoteAddr,c.conn_id)

                }
                if err != nil {
                        if rc != 0 {
                                c.server.logf("msg_len read error %s,conn_id %d: %s, rc: %d\n", c.remoteAddr, c.conn_id, err, rc)
                        }
                        break
                }
                msg_len = binary.BigEndian.Uint32(msg_len_buf)

                //check msg_len
                if msg_len <4 || msg_len > 4096 {
                        c.server.logf("msg_len length error from %s,conn_id %d: %d\n", c.remoteAddr,c.conn_id, msg_len)
                        continue
                }

                //TODO no make buf here
                //body_buf    := make([]byte,msg_len - 4)
                rc,err = io.ReadFull(c.rwc,body_buf[:msg_len-4])

                if debugStatusServer {
                        c.server.logf("read msg_len: %d, msg_body: %v,rc: %d, from: %s,conn_id: %d\n",msg_len,body_buf[:msg_len-4],rc,c.remoteAddr,c.conn_id)

                }

                //TODO need review
                //check body_len
                //if err == io.ErrUnexpectedEOF {
                if err != nil {
                        c.server.logf("msg_body length error from %s,conn_id %d: %s %d\n", c.remoteAddr,c.conn_id,err, rc)
                        break
                }

                //process message
                var sreq statuspb.StatusRequest
                var rsreq statuspb.StatusRequest

                err = proto.Unmarshal(body_buf[:msg_len-4],&sreq)
                if err != nil {
                        c.server.logf("unmarshal error %s", err)
                        break
                }

                if debugStatusServer {
                        c.server.logf("unmarshal result %s", sreq.String())
                }


                if sreq.GetReqType() == statuspb.StatusRequest_ONLINE || sreq.GetReqType() == statuspb.StatusRequest_REPLICATE_ONLINE {
                        if sreq.FieldType != nil {

                                //TODO add replication here
                                if atomic.LoadUint32(&c.server.can_replicate) == 1 {
                                        
                                        //check for blocking chan here
                                        if len(c.server.replication_write_chan) < cap(c.server.replication_write_chan){
                                                rsreq = sreq
                                                rsreq.ReqType = statuspb.StatusRequest_REPLICATE_ONLINE.Enum()
                                                c.server.replication_write_chan <- rsreq
                                        } else {
                                                c.server.logf("channel full for replication server")
                                        }
                                }

                                if *sreq.FieldType == statuspb.StatusRequest_UID {
                                        atomic.AddUint32(&c.server.online_count,1)
                                        c.handleGeneralOnlineMessage(sreq)

                                } else {
                                //StatusRequest_DEVICEID

                                        atomic.AddUint32(&c.server.anonymous_online_count,1)
                                        c.handleAnonymousOnlineMessage(sreq)

                                }
                        } else {
                                c.server.logf("get nil field_type from %s",c.remoteAddr)
                        }
                } else if sreq.GetReqType() == statuspb.StatusRequest_OFFLINE || sreq.GetReqType() == statuspb.StatusRequest_REPLICATE_OFFLINE {
 
                        if sreq.FieldType != nil {

                                //TODO add replication here
                                if atomic.LoadUint32(&c.server.can_replicate) == 1 {
                                        
                                        //check for blocking chan here
                                        if len(c.server.replication_write_chan) < cap(c.server.replication_write_chan){

                                                rsreq = sreq
                                                rsreq.ReqType = statuspb.StatusRequest_REPLICATE_OFFLINE.Enum()
                                                c.server.replication_write_chan <- rsreq
                                        } else {
                                                c.server.logf("channel full for replication server")
                                        }
                                }

                                if *sreq.FieldType == statuspb.StatusRequest_UID {

                                        atomic.AddUint32(&c.server.offline_count,1)
                                        c.handleGeneralOfflineMessage(sreq)

                                } else {
                                //StatusRequest_DEVICEID

                                        atomic.AddUint32(&c.server.anonymous_offline_count,1)
                                        c.handleAnonymousOfflineMessage(sreq)

                                }
                        } else {
                                c.server.logf("get nil field_type from %s",c.remoteAddr)
                        }

                } else if sreq.GetReqType() == statuspb.StatusRequest_GET {

                        //check resume net_id here
                        if sreq.ResumeNetId == nil {
                                c.server.logf("get  nil resume_netid in get request")
                                break
                        }

                        if sreq.FieldType != nil {
                                if *sreq.FieldType == statuspb.StatusRequest_UID {

                                        atomic.AddUint32(&c.server.get_count,1)
                                        c.handleGeneralGetMessage(sreq)

                                } else {
                                //StatusRequest_DEVICEID

                                        atomic.AddUint32(&c.server.anonymous_get_count,1)
                                        c.handleAnonymousGetMessage(sreq)

                                }
                        } else {
                                c.server.logf("get nil field_type from %s",c.remoteAddr)
                        }

                } else {
                        //should not be here
                        c.server.logf("get unknown command in readloop %d",sreq.GetReqType())
                        break

                }
                c.msg_count += 1

                

        }

}


func (c *conn) writeloop() {
        c.rwc.(*net.TCPConn).SetNoDelay(true)
        var queryResp statuspb.StatusQueryResponse
        var generalResp statuspb.StatusResponse
        var ok bool
        var ok1 bool
        var err error
        ok = true
        ok1 = true
        var data []byte
        //9 = 4 + 1 + 4
        buf := make([]byte,2048)
        for ok || ok1 {
                select {
                case generalResp,ok = <- c.GeneralRespWriteChan:
                        if ok {
                        data, err = proto.Marshal(&generalResp)
                        if err != nil {
                                c.server.logf("sendback marshal error:%s",err)
                        } else {
                                binary.BigEndian.PutUint32(buf[:4],uint32(len(data)) + 4)
                                copy(buf[4:],data)
                                c.server.logf("sendback %v",buf[:len(data) + 4])
                                _,err := c.rwc.Write(buf[:len(data) + 4])
                                if err != nil {
                                        c.server.logf("sendback response error:%s",err)
                                        return
                                }
                                if debugStatusServer {
                                        c.server.logf("sendback general response in write loop: %s,%s",generalResp.String(),c.remoteAddr)

                                }
                        }
                        }
                        

                case queryResp,ok1  = <- c.QueryRespWriteChan:
                        if ok1 {
                        data, err = proto.Marshal(&queryResp)
                        if err != nil {
                                c.server.logf("sendback marshal error:%s",err)
                        } else {
                                binary.BigEndian.PutUint32(buf[:4],uint32(len(data)) + 4)
                                copy(buf[4:],data)
                                c.server.logf("sendback %v",buf[:len(data) + 4])
                                _,err := c.rwc.Write(buf[:len(data) + 4])
                                if err != nil {
                                        c.server.logf("sendback response error:%s",err)
                                        return
                                }
                                if debugStatusServer {
                                        c.server.logf("sendback query response in write loop: %s,%s",queryResp.String(),c.remoteAddr)

                                }
                        }
                        }


                }

                
        }
        c.server.logf("closing write loop: %s",c.remoteAddr)

}

func parseConf(filename string,s *Server) {
        var token string
        var key string
        var value string
        var cli_listen bool
        file, err := os.Open(filename)
        if err != nil {
                panic(err)
        }

        scanner := bufio.NewScanner(file)
        for scanner.Scan() {
                var rs []string
                token = scanner.Text()
                if token == "" || token[0] == '#' {
                        continue
                }
                //fmt.Printf("%s\n",token)
                rs  = strings.Split(token,"=")
                key = strings.Trim(rs[0]," ")
                value = strings.Trim(rs[1]," ")
                fmt.Printf("%q ",key)
                fmt.Printf("%q\n",value)
                switch key {
                        case "Addr" :
                                s.Addr = value
                                cli_listen = true


                        case "ReplicationServer" :
                                s.RegisterReplicationServer(value)

                        case "Nlog" :
                                rs = strings.Split(rs[1]," ")
                                if len(rs) != 2 {
                                        panic("nlog value len should be 2")
                                }
                                s.SetupNlog(strings.Trim(rs[0]," "),strings.Trim(rs[1]," "))

                        default:
                                panic("unknown directive" + token)

                }
        }

        if err := scanner.Err(); err != nil {
                panic(err)
        }
        file.Close()

        if cli_listen == false {
                panic("conf file not complete")
        }

}

var compile_time string

func main() {
        fmt.Printf("compile at %s\n",compile_time)
        var server Server

        flag.Parse()
        if *filename == "" {
                flag.Usage()
                fmt.Println("no file name specify")
                os.Exit(-1)
        }
        
        if *logfile == true {
                server.SetupFlog()
        }

        parseConf(*filename,&server)

        //server.SetupFlog()
        //server.Addr = ":3561"
        //server.RegisterReplicationServer("192.168.3.100:3561")
        runtime.GOMAXPROCS(runtime.NumCPU())
        server.ListenAndServe()
}
