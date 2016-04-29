package main

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"time"
)

func loop(startport int, endport int, inport chan int) {
	for i := startport; i <= endport; i++ {
		inport <- i
	}
}

func scanner(inport, outport, out chan int, ip net.IP, endport int) {
	for {
		in := <-inport
		/*
			tcpaddr := &net.TCPAddr{IP: ip, Port: in}
			conn, err := net.DialTCP("tcp", nil, tcpaddr)
		*/
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip.String(), in), time.Second)
		if err != nil {
			outport <- 0
		} else {
			outport <- in
			conn.Close()
		}
		if in == endport {
			out <- in
		}
	}
}

func main() {
	runtime.GOMAXPROCS(4)
	inport := make(chan int)
	starttime := time.Now().Unix()
	outport := make(chan int)
	out := make(chan int)
	collect := []int{}
	if len(os.Args) != 4 {
		fmt.Println("Usage: scanner ip startport endport")
		os.Exit(0)
	}
	ip := net.ParseIP(os.Args[1])
	if ip == nil {
		fmt.Fprintf(os.Stderr, "Err:无效的地址")
		return
	}
	if os.Args[3] < os.Args[2] {
		os.Args[2], os.Args[3] = os.Args[3], os.Args[2]
	}
	start, end := os.Args[2], os.Args[3]
	startport, err := strconv.Atoi(start)
	if err != nil {
		fmt.Println("startport must be int")
		os.Exit(0)
	}
	endport, err := strconv.Atoi(end)
	if err != nil {
		fmt.Println("endport must be int")
		os.Exit(0)
	}
	fmt.Printf("the ip is %s \r\n", ip)
	fmt.Printf("Scanning from %d ----- %d \r\n", startport, endport)
	go loop(startport, endport, inport)
	for {
		select {
		case <-out:
			fmt.Println(collect)
			endtime := time.Now().Unix()
			fmt.Printf("The scan proxess has spent %d seconds \r\n", (endtime - starttime))
			return
		default:
			go scanner(inport, outport, out, ip, endport)
			port := <-outport
			if port != 0 {
				fmt.Printf("The scan proxess has found %d port open \r\n", port)
				collect = append(collect, port)
			}
		}
	}
}
