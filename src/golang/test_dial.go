package main

import (
"net"
"crypto/rand"
"fmt"
"encoding/binary"
"sync/atomic"
"sync"
"io"
"bytes"
)

const BUF_SIZE=40960
const CONN_NUM=60000

func main(){
        var wg sync.WaitGroup
        b := make([]byte,BUF_SIZE)
        rand.Read(b)
        var counter uint32
        ip,err := net.ResolveTCPAddr("tcp4","192.168.8.25:0")
        if err != nil {
                panic("resolve net addr error")
        }
        d := &net.Dialer{LocalAddr: ip}
        wg.Add(CONN_NUM)
        for i:=0;i<CONN_NUM;i++ {
                go dial_send(d,b,&counter,&wg)
        }
        wg.Wait()
        fmt.Println("done")

}

func dial_send(dialer *net.Dialer,rand_buf []byte,counter *uint32,wg *sync.WaitGroup) {
        coun := atomic.AddUint32(counter,1)
        conn,err := dialer.Dial("tcp","192.168.8.26:8999")
        if err!= nil {
                fmt.Println(err)
                return
        }
        local_buf := []byte{0,0,0,0,}
        binary.BigEndian.PutUint32(local_buf,132)
        conn.Write(local_buf)
        _,err = conn.Write(rand_buf[(coun%(BUF_SIZE-128)):(coun%(BUF_SIZE-128))+128])
        //fmt.Println(ret)
        //fmt.Println(rand_buf[(coun%(BUF_SIZE-128)):(coun%(BUF_SIZE-128))+128])
        msg_buf := make([]byte,132)
        _,err = io.ReadFull(conn,msg_buf)
        if err!= nil {
                fmt.Println(err)
                return
        }
        //fmt.Println(msg_buf[4:])
        if !bytes.Equal(msg_buf[4:],rand_buf[(coun%(BUF_SIZE-128)):(coun%(BUF_SIZE-128))+128]){
                fmt.Println("bytes not equal")
        }
        wg.Done()
}
