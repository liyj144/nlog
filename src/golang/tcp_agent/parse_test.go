package tcp_agent

import (
"testing"
"strconv"
"strings"
"github.com/golang/protobuf/proto"
"fmt"
"./statuspb"
"encoding/json"
"runtime"
)

var body = []byte{0,0,0,2,0,0,0,1,0,0,0,13,0,0,0,8,56,65,48,52,65,66,56,69,0,0,0,128,34,96,4,206,157,196,38,12,98,47,165,160,174,225,133,136,22,120,140,51,208,164,93,217,135,27,163,221,7,136,246,192,159,166,188,46,137,175,184,105,111,26,224,142,192,177,255,27,49,31,161,185,37,54,243,178,61,192,165,151,0,60,82,170,98,181,175,145,125,95,198,177,91,237,113,133,37,95,32,135,195,242,133,190,116,141,204,225,63,52,55,177,75,244,253,147,27,87,152,163,69,38,159,157,26,243,87,253,72,148,131,248,183,26,163,198,64,190,116,14,131,73,46,43,63,25,29,153}

func TestAllocd(t *testing.T) {
    str := "kacced"
        n := testing.AllocsPerRun(9, func() {
            b := []byte(str)
            s := string(b)
            _ = s
        })
        t.Log(n)
}

func TestAllocc(t *testing.T) {

        n := testing.AllocsPerRun(9, func() {
                //t.Logf("str:%s\n",string(buf[4:15]))
                bb := make([]byte,160)
                strconv.AppendInt(bb,23424,10)
                //t.Logf("str:%s\n",buf[4:15])
        })
        t.Log(n)
}

func TestAlloca(t *testing.T) {
        buf := []byte("1000000000000000000000000000000000000000000000000")
        //buf := "1000000000000000000000000000000000000000000000000"
        n := testing.AllocsPerRun(9, func() {
                //t.Logf("str:%s\n",string(buf[4:15]))
                _ = string(buf[1:9])
                _ = string(buf[3:10])
                _ = string(buf[5:15])
                _ = string(buf[3:15])
                //t.Logf("str:%s\n",buf[4:15])
        })
        t.Log(n)

}

func TestAllocb(t *testing.T) {
        //buf := []byte("1000000000000000000000000000000000000000000000000")
        //buf := "1000000000000000000000000000000000000000000000000"
        n := testing.AllocsPerRun(9, func() {
                var sreq statuspb.StatusRequest
                sreq.ReqType = statuspb.StatusRequestType_ONLINE.Enum()
                sreq.Uid=proto.Uint64(12306)
                sreq.DeviceId=proto.String("Android_IOS")
                sreq.NetId=proto.Uint32(12)
                sreq.Timestamp=proto.Uint64(19)
                //t.Logf("str:%s\n",string(buf[4:15]))
                data,_ := proto.Marshal(&sreq)
                _ = data
                //t.Logf("str:%s\n",buf[4:15])
        })
        t.Log(n)

}

func TestParse(t *testing.T) {
    var cm channelMessage
    
    err := cm.parse(body,uint32(len(body)),stageHandshake)
    
    t.Log(err)
    t.Log(cm)

}
func BenchmarkParse(b *testing.B) {
    runtime.GOMAXPROCS(4)
    var cm channelMessage
    for i := 0; i < b.N; i++ {
        cm.parse(body,uint32(len(body)),stageHandshake)
    }
    b.ReportAllocs()
}

//{"url":"/cc/token_check?token=sdafdsfsdfafsdsaf","id":"1","context":"abcdefg"} 
type readRequest struct {
Url string
Id string
Context string
}

func BenchmarkSerialize(b *testing.B) {
        str := `{"url":"/cc/token_check?token=sdafdsfsdfafsdsaf","id":"1","context":"abcdefg"}`
        var rr readRequest
        for i := 0; i < b.N; i++ {
            _ = json.Unmarshal([]byte(str), &rr)
        }

}


func BenchmarkIndex(b *testing.B) {
    b.ReportAllocs()
    rs := `{"context":"","headers":{"Connection":"keep-alive","Content-Length":"0","Content-Type":"text/plain","Date":"Wed, 07 Jan 2015 08:04:44 GMT","X-IS-UserID":"11111111"},"id":"131","ret":"200"}`
    for i := 0; i < b.N; i++ {
                rindex := strings.Index(rs,"\"ret\":\"")
                rindex += len("\"ret\":\"")
                rindex2 := strings.IndexByte(rs[rindex:],'"')
                //TODO not use atoi, use  []byte compare
                rc_strconv,_ := strconv.Atoi(rs[rindex:rindex+rindex2])

                //find user id
                rindex = strings.Index(rs,"\"X-IS-UserID\":\"")
                rindex += len("\"X-IS-UserID\":\"")
                rindex2 = strings.IndexByte(rs[rindex:],'"')

                //find id
                rindex = strings.Index(rs,"\"id\":\"")
                rindex += len("\"id\":\"")
                rindex2 = strings.IndexByte(rs[rindex:],'"')
                cid,_ := strconv.Atoi(rs[rindex:rindex+rindex2])
                _ = rc_strconv 
                _  = cid
    }



}



//go test -bench .

//run with: go test -v .
//=== RUN TestParse
//--- PASS: TestParse (0.01s)
//        parse_test.go:15: true
//        parse_test.go:16: {[56 65 48 52 65 66 56 69] [49 50 51 52 53 54 55 56] [115 100 97 102 100 115 102 115 100 102 97 102 115 100 115 97 102] [48 118 90 111 80 57 76 57] 0 0 2 0 1 1}
//PASS

var rootsValid = []string{"foo", "bar", "qwe", "asd","ewt","452","88gh", "bare"} 

var void struct{} 
var rootsValid2 = map[string]struct{}{"foo": void, "98756": void, "bar112": void, "barw": void, "bar": void, "qwe": 
void, "asd": void, "bare": void} 

// uses a slice 
func BenchmarkSearch1(b *testing.B) { 
    b.ReportAllocs()
        for i := 0; i < b.N; i++ { 
                for _, v := range rootsValid { 
                        if v == "bar" { 
                                break 
                        } 
                } 
        } 
} 

// uses a map 
func BenchmarkSearch2(b *testing.B) { 
    b.ReportAllocs()
        for i := 0; i < b.N; i++ { 
                if _, ok := rootsValid2["foo"]; ok { 
                        continue 
                } 
        } 
} 

func BenchmarkHashStringSpeed(b *testing.B) {
        size := 10000
        strings := make([]string, size)
        for i := 0; i < size; i++ {
                strings[i] = fmt.Sprintf("string#%d", i)
        }
        sum := 0
        m := make(map[string]int, size)
        for i := 0; i < size; i++ {
                m[strings[i]] = 0
        }
        idx := 0
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
                sum += m[strings[idx]]
                idx++
                if idx == size {
                        idx = 0
                }
        }
}
