package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	//"syscall"
	"runtime"
	"strings"
	"time"
)

const LISTENPORT = 5050
const BUFFSIZE = 1024

const LOGCOUNT = 10
const LOGDIR = "/data/log_node"

/* 是否多个频道， 如果是多个频道，则根据来源端口的不同，分解为不同的目录
10.0.0.1/5001/2016/02/29/log.xxxx  10.0.0.1/5002/2016/02/29/log.xxx
否则，保存到单个文件夹下 log_jrtool/2016/02/29/log.xxxx
*/
const MULCHANNEL = false

var buff = make([]byte, BUFFSIZE)

var chs = make(chan string, LOGCOUNT)

// YYYYMMDDHHmmSS
func GetTime() string {
	tm := time.Unix(time.Now().Unix(), 0)
	return tm.Format("20060102150405")
}

//YYYYMMDD
func GetDayTime() string {
	tm := time.Unix(time.Now().Unix(), 0)
	return tm.Format("20060102")
}

func HandleError(err error, msg string) {
	if err != nil {
		fmt.Println(err.Error())
		if len(msg) > 0 {
			fmt.Println("错误信息", msg)
		}
		os.Exit(2)
	}
}

func HandleLog(msg string, addr string) {
	var str_ret string
	var err error
	os_type := runtime.GOOS
	if os_type == "windows" {
		str_ret = "\\"
	} else {
		str_ret = "/"
	}
	this_date := GetDayTime()
	this_year := this_date[0:4]
	this_month := this_date[4:6]
	this_day := this_date[6:8]
	year_dir := LOGDIR + str_ret + this_year
	_, err = filepath.Abs(year_dir)
	if err == nil {
		os.Mkdir(year_dir, 0777)
	}
	month_dir := year_dir + str_ret + this_month
	_, err = filepath.Abs(month_dir)
	if err == nil {
		os.Mkdir(month_dir, 0777)
	}
	day_dir := month_dir + str_ret + this_day
	_, err = filepath.Abs(day_dir)
	if err == nil {
		os.Mkdir(day_dir, 0777)
	}
	dir, err := ioutil.ReadDir(day_dir)
	HandleError(err, "")
	var file_name string = ""
	for _, fi := range dir {
		if fi.IsDir() {
			continue
		}
		if strings.HasPrefix(fi.Name(), "log.") {
			file_name = fi.Name()
		}
	}
	if len(file_name) <= 0 {
		file_name = "log." + GetTime()
		_, err = os.Create(day_dir + str_ret + file_name)
		HandleError(err, "")
	}
	file_path := day_dir + str_ret + file_name
	fout, err := os.OpenFile(file_path, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	HandleError(err, "")
	_, err = io.WriteString(fout, msg)
	HandleError(err, "")
	fout.Close()
}

func ReadCHannel() {
	select {
	case log, ok := <-chs:
		if ok {
			ar := strings.SplitN(log, "#", 2)
			addr := ar[0]
			msg := ar[1]
			HandleLog(msg, addr)
		}
	}
}

/*
func HandleMessage(udpListener *net.UDPConn) {
	fmt.Println("in handle messgae")
	for {
		n, addr, err := udpListener.ReadFromUDP(buff)
		HandleError(err, "")
		if n > 0 {
			msg := AnalyzeMessage(buff, n)
			//HandleLog(msg, addr)
			chs <- (msg + "#" + addr.String())
			fmt.Println(msg)
		}
	}
}
*/

// 消息解析： []byte -> string
func AnalyzeMessage(buff []byte, len int) string {
	analMsg := ""
	strNow := ""
	for i := 0; i < len; i++ {
		if string(buff[i:i+1]) == ":" {
			analMsg = analMsg + strNow
			strNow = ""
		} else {
			strNow += string(buff[i : i+1])
		}
	}
	analMsg = analMsg + strNow
	return analMsg
}

/*
var data = `
buffer_size: Easy!
log_list:
  - /data/log 0.0.0.0 5050 mixed 0 127.0.0.1 6251
  - /data/log_node 0.0.0.0 5051 multi 6000 127.0.0.1 6251
`
err := yaml.Unmarshal([]byte(data), &appConfig)
*/

func main() {
	fmt.Println("start to init nlog ...")
	filePath := "../../etc/conf.yml"
	config, err := ioutil.ReadFile(filePath)
	HandleError(err, "")
	type AppConfig struct {
		buffer_size string `yaml:"buffer_size,omitempty"`
		log_list    struct {
			dir        string
			ip         string
			port       int
			log_type   string
			trans_type int
			trans_ip   string
			trans_port int
		}
	}

	data := `
buffer_size: 1024
log_list:
- dir: "/data/log"
  ip: 0.0.0.0
  port: 5050
  log_type: mixed
  trans_type: 0
  trans_ip: 127.0.0.1
  trans_port: 6251
- dir: "data/log_2"
  ip: 0.0.0.0
  port: 5051
  log_type: mixed
  trans_type: 2
  trans_ip: 172.19.44.10
  trans_port: 5050
`
	var appConfig1 AppConfig
	err = yaml.Unmarshal(config, &appConfig1)
	fmt.Println(err, appConfig1, appConfig1.log_list, appConfig1.log_list.dir)
	var appConfig AppConfig
	//appConfig := make(map[map[string]string{}]string{})
	//err = yaml.Unmarshal(config, &appConfig)
	err = yaml.Unmarshal([]byte(data), &appConfig)
	fmt.Println(err, appConfig, appConfig.log_list)
	HandleError(err, "")
	/*
		for k, v := range appConfig["log_list"] {
			fmt.Println(k, v)
		}
	*/
	/*
		for i := 0; i < len(appConfig["log_list"]); i++ {
			fmt.Println(appConfig["log_list"][i], appConfig["log_list"][0])
		}
	*/
	//fmt.Println("good boy", appConfig["buffer_size"])
	udpAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:"+strconv.Itoa(LISTENPORT))
	HandleError(err, "")
	udpListener, err := net.ListenUDP("udp4", udpAddr)
	HandleError(err, "")
	defer udpListener.Close()
	fmt.Println("开始监听 ... ")
	_, err = filepath.Abs(LOGDIR)
	HandleError(err, "日志目录不存在，请创建")
	for {
		n, addr, err := udpListener.ReadFromUDP(buff)
		HandleError(err, "")
		if n > 0 {
			msg := AnalyzeMessage(buff, n)
			chs <- (addr.String() + "#" + msg)
		}
		go ReadCHannel()
	}
}
