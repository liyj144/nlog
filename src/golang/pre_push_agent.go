package main

import (
        "fmt"
        "runtime"
        "./push_agent"
        "bufio"
        "os"
        "strings"
        "flag"
)

var filename = flag.String("f", "", "configuration file name")
var logfile = flag.Bool("l", false, "log file bool value (overwrite nlog)")

func parseConf(filename string,s *push_agent.Server) {
        var token string
        var key string
        var value string
        var cli_listen,frontend_listen,status bool
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
                        case "ClientListenAddr" :
                                s.ClientListenAddr = value
                                cli_listen = true

                        case "FrontendListenAddr" :
                                s.FrontendListenAddr = value
                                frontend_listen = true

                        case "Status" :
                                s.RegisterStatus(value)
                                status = true

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

        if cli_listen == false || frontend_listen == false || status == false {
                panic("conf file not complete")
        }

}

func main() {

        flag.Parse()
        if *filename == "" {
                flag.Usage()
                fmt.Println("no file name specify")
                os.Exit(-1)
        }

        var server push_agent.Server

        if *logfile == true {
        server.SetupFlog()
        }

        fmt.Println(push_agent.CompileTime)
        parseConf(*filename,&server)
        //server.ClientListenAddr = ":6201"
        //server.SetupNlog("0.0.0.0:12110","127.0.0.1:12113")
        //server.SetupFlog()
        //server.RegisterStatus("127.0.0.1:3561")
        //server.RegisterDownstream("127.0.0.1:8999")
        //server.RegisterDownstream("127.0.0.1:8999")
        runtime.GOMAXPROCS(runtime.NumCPU())
        server.ListenAndServe()

}
