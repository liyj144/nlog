package main

import (
        "runtime"
        "./tcp_agent"
)

func main() {
var server tcp_agent.Server
server.Addr = ":8999"
server.RegisterCmd(114,"127.0.0.1:3540")
runtime.GOMAXPROCS(runtime.NumCPU())
server.ListenAndServe()
}
