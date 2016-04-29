package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"runtime"
	//"strconv"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var counter uint64

func http_worker(in <-chan string, out_ok chan<- string, out_err chan<- string, wg *sync.WaitGroup, worker int) {
	for {
		tc := atomic.AddUint64(&counter, 1)
		if tc%10 == 0 {
			fmt.Printf("process line:%d\n", tc)
		}
		str, ok := <-in
		if !ok {
			break
		}
		/*
			if worker%2 == 0 {
				fmt.Println("pass : " + str + ", manage by " + strconv.Itoa(worker))
			} else {
				fmt.Println("here : " + str + ", manage by " + strconv.Itoa(worker))
			}
		*/
		items := strings.Split(str, "	")
		if len(items) != 6 {
			fmt.Printf("%d len is not 6\n", len(items))
			fmt.Printf("%s len is not 6\n", str)
			continue
		}
		url := fmt.Sprintf("http://127.0.0.1/bakup/upload?file_name=%s&group_id=%s&file_md5=%s&file_len=%s&file_size=%s&file_pos=%s", items[0], items[1], items[2], items[3], items[4], items[5])
		fmt.Println(url)
		http_client := http.Client{Timeout: 2 * time.Second}
		resp, err := http_client.Get(url)
		if err == nil && http.StatusOK == resp.StatusCode {
			out_ok <- str
		} else {
			out_err <- str
		}
	}
	wg.Done()
}

func log_worker(in_ok <-chan string, in_err <-chan string, wg1 *sync.WaitGroup) {
	var s1, s2 string
	var s1_ok bool = true
	var s2_ok bool = true
	for s1_ok || s2_ok {
		select {
		case s1, s1_ok = <-in_ok:
			if s1_ok {
				fmt.Println("Manage ok : " + s1)
			}
		case s2, s2_ok = <-in_err:
			if s2_ok {
				fmt.Println("Error happens : " + s2)
			}
		}
	}
	wg1.Done()
}

func main() {
	file, err := os.Open("test.log")
	if err != nil {
		log.Fatal(err)
	}
	runtime.GOMAXPROCS(4)
	var wg sync.WaitGroup
	var wg1 sync.WaitGroup

	var in chan string
	var f_ok chan string
	var f_err chan string

	in = make(chan string, 100)
	f_ok = make(chan string, 1)
	f_err = make(chan string, 1)

	wg.Add(8)
	wg1.Add(1)

	for i := 0; i < 8; i++ {
		go http_worker(in, f_ok, f_err, &wg, i)
	}
	go log_worker(f_ok, f_err, &wg1)

	bf := bufio.NewReader(file)
	for {
		//l, err := bf.ReadString('\n')
		l, _, err := bf.ReadLine()
		if err != nil {
			//if not eof, sleep 1 minute and retry
			if "EOF" == err.Error() {
				//fmt.Println("File end : ", err)
				break
			} else {
				fmt.Println("error happens(just waite 1 minute) : ", err)
				time.Sleep(time.Second * 60)
				continue
			}
		}
		in <- string(l)
	}
	close(in)
	file.Close()
	wg.Wait()

	close(f_ok)
	close(f_err)
	wg1.Wait()
	fmt.Println("file manage all ok")
}
