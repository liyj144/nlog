package main

import (
        "fmt"
        "runtime"
        "./tcp_agent"
        "bufio"
        "os"
        "strings"
        "strconv"
        "flag"
)

var filename = flag.String("f", "", "configuration file name")
var logfile = flag.Bool("l", false, "log file bool value (overwrite nlog)")

func parseConf(filename string,s *tcp_agent.Server) {
        var token string
        var key string
        var value string
        var status_master_ipport string
        var status_slave_ipport string
        var temp_sid uint64
        var sid,cli_listen,push_listen,ra,cmdrange,status bool
        file, err := os.Open(filename)
        if err != nil {
                panic(err)
        }

        scanner := bufio.NewScanner(file)
        for scanner.Scan() {
                var rs []string
                var low_index uint64
                var high_index uint64
                var err error
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
                        case "ServerId" :
                                temp_sid,err =  strconv.ParseUint(value,10,32)
                                if err != nil {
                                        fmt.Println("parse server id error" + err.Error())
                                        os.Exit(-1)
                                }
                                s.ServerId =  uint32(temp_sid)
                                sid = true

                        case "ClientListenAddr" :
                                s.ClientListenAddr = value
                                cli_listen = true

                        case "Push" :
                                s.RegisterPush(value)
                                push_listen = true

                        case "Restagent" :
                                s.RegisterRestagent(value)
                                ra = true

                        case "CmdRange" :
                                rs = strings.Split(strings.Trim(rs[1]," ")," ")
                                fmt.Printf("%q ",rs)
                                if len(rs) != 3 {
                                        panic("cmdRange value len should be 3")
                                }
                                low_index,err = strconv.ParseUint(rs[0],10,16)
                                if err != nil {
                                        fmt.Println("parse low index error" + err.Error())
                                }
                                high_index,err = strconv.ParseUint(rs[1],10,16)
                                if err != nil {
                                        fmt.Println("parse high index error" + err.Error())
                                }
                                value = strings.Trim(rs[2]," ")
                                s.RegisterCmd(uint16(low_index),uint16(high_index),value)
                                cmdrange = true

                        case "StatusMaster" :
                                status_master_ipport = value
                                status = true

                        case "StatusSlave" :
                                status_slave_ipport = value

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

        if sid == false || cli_listen == false || push_listen == false || ra == false || cmdrange == false || status == false {
                panic("conf file not complete")
        }

        s.RegisterStatusMaster(status_master_ipport)
        if status_slave_ipport != "" {
                s.RegisterStatusSlave(status_slave_ipport)

        }
}

func main() {
        flag.Parse()
        if *filename == "" {
                flag.Usage()
                fmt.Println("no file name specify")
                os.Exit(-1)
        }

        var server tcp_agent.Server

        if *logfile == true {
        server.SetupFlog()
        }

        fmt.Printf("PRE Tcpd Compile At %s\n",tcp_agent.CompileTime)

        parseConf(*filename,&server)

        //server.ClientListenAddr = ":9000"
        //server.PushListenAddr = ":8999"
        //server.SetupNlog("0.0.0.0:12110","127.0.0.1:12113")
        //server.SetupFlog()
        //server.RegisterAllowPushIP("127.0.0.1:8999")
        //server.RegisterRestagent("10.0.2.28:5300")
        //server.RegisterCmd(2000,2999,"10.0.4.229:6101")
        //server.RegisterCmd(3000,3999,"10.0.4.229:6102")
        //server.RegisterStatus("127.0.0.1:3561")
        runtime.GOMAXPROCS(runtime.NumCPU())
        server.ListenAndServe()

}
