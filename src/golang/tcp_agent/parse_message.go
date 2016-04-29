package tcp_agent

import (
        "math/big"
        "fmt"
        "strconv"
        "bytes"
        "errors"
        "crypto/rsa"
        "crypto/aes"
        "crypto/cipher"
        "crypto/md5"
        "encoding/binary"
        "encoding/hex"
)

var errNoApiType = errors.New("api_type not found")
var errApiTypeQuote = errors.New("api_type last quote not found")
var errParseUid = errors.New("uid convert error")
var errModuleNotFound = errors.New("8bytes module not match")

const debugParse = false

const stageHandshake = 0
const stageMsgloop   = 1

const GENERAL_CHANNEL = 0
const ANONYMOUS_CHANNEL = 1

const (
        
        S_COMPRESSION_NOT uint16 = 0
        S_COMPRESSION_GZIP uint16 = 1
        
        S_ENC_NOT uint16 = 0
        S_ENC_AES_STATIC uint16 = 1
        S_ENC_RSA uint16 = 2
        S_ENC_AES_DYNAMIC uint16 = 3
        
        S_ERROR_CODE_OK                 uint16 = 0
        S_ERROR_CODE_DEC_FAIL           uint16 = 1
        S_ERROR_CODE_BUSY               uint16 = 2
        S_ERROR_CODE_FORCE_OFFLINE      uint16 = 3
        
        S_TRANS_MSG                        uint16 = 0
        S_PTOTO_MSG_EXCHANGE_KEY           uint16 = 1
        S_PTOTO_MSG_EXCHANGE_KEY_ANONYMOUS uint16 = 2
        S_PTOTO_MSG_INVALID_TOKEN          uint16 = 3
        S_PTOTO_MSG_TOKEN_EXPIRE           uint16 = 4
        S_PTOTO_MSG_CLIENT_METADATA        uint16 = 6

)

func fromBase16(basen string) *big.Int {
        i, ok := new(big.Int).SetString(basen, 16)
        if !ok {
                panic("bad number: ")
        }
        return i
}


func fromBase10(basen string) *big.Int {
        i, ok := new(big.Int).SetString(basen, 10)
        if !ok {
                panic("bad number: ")
        }
        return i
}

var module_0 = []byte("8A04AB8E")

var private_key_0 = &rsa.PrivateKey{
	PublicKey: rsa.PublicKey{
		N: fromBase16("8A04AB8E796657CF6062090F2098B6FA1F8EDD" +
                              "1D81578244F5306B1D1FE5C385B33F063D120B1" +
                              "9876AFEF6FF91E7C5A16F8E5E06D87CC6B82B6E" + 
                              "78D4732160BBB25FA3CD651928D7DE5C34134F8" + 
                              "78CF2D8C87D4D56735695177F41B1F3A766482A" + 
                              "2F4DAF1000119A1B34FB7A1AAAC8E4A777DE2CF" + 
                              "3D2D809A0ABDFA500313939"),
		E: 16*16*16*16 + 1,
                },
	D: fromBase16("ECA475422404C62A5B27BC40A3B3348847F3BC" + 
                      "4C0AA8F0432BE388C4B71C4CD1C1341E8E3791" + 
                      "B083EF809A20391B1C505FE5CA72125E5E9B08" + 
                      "5CB1F01236F8924308C81F88BA1EFA50C79DD0" + 
                      "204F54E77C218674BC4A440B2CF1E56D1A3283" + 
                      "9F3410EFF010FB0989F913C02B420C5062C82C" + 
                      "BC97B0285588EF8C5D7DA6A059D"),
}

var module_1 = []byte("D05DD53D")

var private_key_1 = &rsa.PrivateKey{
        PublicKey: rsa.PublicKey{
                N: fromBase10("146319956869801663992443878496015709270422605595011559080683534285168490387715684243866903611452291827593062440435043553360716985525902124157375685909870398679059644156356595983120963115799711674329822273650765116038602864398364448628528543048275091034285211074913146987123461659139883694935076637454422857311"),
                E: 65537,
                },
        D: fromBase10("101198478493637182412766890162639486917320680589668262789724011757386394918961637238838444542720875714013621942377576317853750077100455705657588350324804224546230870706755283980116835625436554494132748426414544782112801415927121472690371604224115309570973699852776978207843493154112149411331768070108817039377"),
        Primes: []*big.Int{fromBase10("12364307519591839642326124615552511499802685908288065459735001159710466451717109003496236658644477984677269610521691571303855412690447741518761119934304809"),
                 fromBase10("11834059985805970810161840765760295834880402977377781347003352472465476560293552995520788807854693770400170792403555586154961248142439276641159791996388679")},
}



func padToBlockSize(payload []byte, blockSize int) (prefix, finalBlock []byte) {
        overrun := len(payload) % blockSize
        paddingLen := blockSize - overrun
        prefix = payload[:len(payload)-overrun]
        finalBlock = make([]byte, blockSize)
        copy(finalBlock, payload[len(payload)-overrun:])
        for i := overrun; i < blockSize; i++ {
                finalBlock[i] = byte(paddingLen)
        }
        return
}

const RSA_BLOCK_SIZE = 128      //we are in 1024 bit mode 
const AES_BLOCK_SIZE = 16      //we are in 128 bit mode 
var aes_iv = make([]byte,16)  //global empty iv object
const aes_key = "32F720C55DB22069"


func removePaddingSSL30(payload []byte) ([]byte, byte) {
        if len(payload) < 1 {
                return payload, 0
        }

        paddingLen := int(payload[len(payload)-1])
        if paddingLen > len(payload) {
                return payload, 0
        }

        return payload[:len(payload)-paddingLen], 255
}


type channelMessage struct {
//        m_body       []byte



//      should be 8 bytes 

        m_module     []byte

//      dynamic length fields

        m_anonymous_deviceid  []byte
        m_user_id    []byte
        m_token      []byte
        m_random8    []byte
        m_platform   uint32
        m_product    uint32
        m_protocol   uint32
        m_device_id    []byte
        m_timestamp    []byte
  
//      we only need m_cpt now

        m_length     uint32
        m_compress_flag    uint16
        m_crypt_flag   uint16
        m_code       uint16
        m_type       uint16
        m_req_id         uint32

        unix_msec    string
        now          int64
        uid          uint64
        client_ts    uint64


        aes_dynamic_key  []byte
        payload          []byte

}

func newChannelMessage(now int64) channelMessage {
        return channelMessage{now:now,}
}

func (cm *channelMessage)parse(body []byte,body_len uint32,stage int) (err error){
        if body_len < 12 {
                return fmt.Errorf("parse_msg: body_len < 12")
        }
        //decode header field
        cm.m_compress_flag = binary.BigEndian.Uint16(body[0:2])
        if cm.m_compress_flag > S_COMPRESSION_GZIP {
                return fmt.Errorf("parse_msg: m_compress_flag error")
        }

        cm.m_crypt_flag = binary.BigEndian.Uint16(body[2:4])
        if cm.m_crypt_flag > S_ENC_AES_DYNAMIC {
                return fmt.Errorf("parse_msg: m_crypt_flag error")
        }
 
        cm.m_code = binary.BigEndian.Uint16(body[4:6])
        cm.m_type = binary.BigEndian.Uint16(body[6:8])
        cm.m_req_id = binary.BigEndian.Uint32(body[8:12])

        //TODO check msg type here
        if cm.m_crypt_flag  == S_ENC_RSA && stage == stageHandshake {
                if cm.m_type != S_PTOTO_MSG_EXCHANGE_KEY && cm.m_type != S_PTOTO_MSG_EXCHANGE_KEY_ANONYMOUS {
                        return fmt.Errorf("parse_msg: m_type should be 1 or 2 in handshake stage")
                }
                if binary.BigEndian.Uint32(body[12:16]) != 8 {

                        //  m_module should be 8 bytes
                        return fmt.Errorf("parse_msg: m_module should be 8 bytes")
                }

                cm.m_module = body[16:24]

                encrypted_body_len := binary.BigEndian.Uint32(body[24:28])
                if 28 + encrypted_body_len != body_len {
                        return fmt.Errorf("parse_msg: encrypted_body_len error")
                }

                if encrypted_body_len  != 128 {
                        return fmt.Errorf("parseMessage:  rsa encrypted_body_len %d  should equal to blocksize(128)",encrypted_body_len)
                }

                var private_key *rsa.PrivateKey

                if 0 == bytes.Compare(cm.m_module,module_0) {
                        private_key = private_key_0
                } else if 0 == bytes.Compare(cm.m_module,module_1) {
                        private_key = private_key_1

                } else {

                        return errModuleNotFound
                }

                //decrypt message, 28 =   2+2+2+2+4 (header)   +  4+8  (8 bytes key and 4 bytes len) +  4  (encrypted body len)
                var out []byte
                out,err := rsa.DecryptPKCS1v15(nil,private_key,body[28:28 + encrypted_body_len])

                if debugParse {
                        fmt.Printf("debug parse: %v\n",out)
                }

                if err != nil {
                        return err
                }

                //fmt.Println(out)


                out_len := len(out)

                offset := 4 + binary.BigEndian.Uint32(out[:4])

                if int(offset) > out_len {
                        return fmt.Errorf("parse_msg: m_user_id error")
                }

                if cm.m_type == S_PTOTO_MSG_EXCHANGE_KEY {
                        cm.m_user_id = out[4:offset]
                        cm.uid,err = strconv.ParseUint(string(cm.m_user_id),10,64)
                } else {
                        //handle S_PTOTO_MSG_EXCHANGE_KEY_ANONYMOUS here
                        cm.m_anonymous_deviceid = out[4:offset]
                }

                if err != nil {
                        return errParseUid
                }

                //12 is the sum of length fields of uid,token,random8
                //13 is the length of 1420710278084
                //offset -4 is length of uid
                //12bytes metadatas
                //TODO: in ntest v1.2.8 we did not send 12bytes metadatas
                //md5_buf := make([]byte,out_len - 12 + 13 - int(offset) + 4 - 12) 
                offset2 := binary.BigEndian.Uint32(out[offset:offset + 4])
                offset += 4
                offset2 += offset
                if int(offset2) > out_len {
                        return fmt.Errorf("parse_msg: m_token error")
                }
                cm.m_token =  out[offset:offset2]
                md5_buf := make([]byte,int(offset2) - int(offset) + 8 + 13) 

                offset = offset2
                offset2 = binary.BigEndian.Uint32(out[offset:offset + 4])
                offset += 4
                offset2 += offset
                if int(offset2) > out_len {
                        return fmt.Errorf("parse_msg: m_random8 overflow error")
                }
                cm.m_random8 = out[offset:offset2]


                cm.m_platform = binary.BigEndian.Uint32(out[offset2:])
                cm.m_product = binary.BigEndian.Uint32(out[offset2 + 4:])
                cm.m_protocol = binary.BigEndian.Uint32(out[offset2 + 8:])

                offset2 += 12

                offset = offset2
                offset2 = binary.BigEndian.Uint32(out[offset:offset + 4])
                offset += 4
                offset2 += offset
                if int(offset2) > out_len {
                        return fmt.Errorf("parse_msg: device_id overflow error")
                }
                cm.m_device_id = out[offset:offset2]

                offset = offset2
                offset2 = binary.BigEndian.Uint32(out[offset:offset + 4])
                offset += 4
                offset2 += offset
                if int(offset2) > out_len {
                        return fmt.Errorf("parse_msg: timestamp error")
                }
                cm.m_timestamp = out[offset:offset2]
                cm.client_ts,_ = strconv.ParseUint(string(cm.m_timestamp),10,64)

                //md5 buffer = append(m_user_id,m_token,m_random8)


                copy_len := 0
                copy_len += copy(md5_buf[copy_len:],cm.m_token)
                copy_len += copy(md5_buf[copy_len:],cm.m_random8)
                //TODO lazy update in server, patched time.go
                cm.unix_msec = strconv.FormatInt(cm.now,10)
                copy_len += copy(md5_buf[copy_len:],cm.unix_msec)
                //fmt.Printf("%d,%d,%d,%d,%d\n",len(md5_buf),copy_len,len(cm.m_token),len(cm.unix_msec),len(cm.m_random8))

                //fmt.Printf("out:%v\n",out)
                //fmt.Printf("md5:%d,%d\n",copy_len,len(md5_buf))
                //fmt.Printf("md5:%s\n",string(md5_buf[:copy_len]))
                md5_out := md5.Sum(md5_buf[:copy_len])

                hex_out := make([]byte,16)
                hex.Encode(hex_out,md5_out[:8])
                //fmt.Println(string(hex_out))
                cm.aes_dynamic_key = hex_out

        } else if cm.m_crypt_flag  == S_ENC_AES_STATIC && stage == stageMsgloop {


                encrypted_body_len := binary.BigEndian.Uint32(body[12:16])
                if encrypted_body_len % AES_BLOCK_SIZE != 0 {
                        return fmt.Errorf("parseMessage: aes static encrypted_body_len %d  should be multiple of blocksize(16)",encrypted_body_len)
                }
                if 16 + encrypted_body_len != body_len {
                        return fmt.Errorf("parse_msg: encrypted_body_len error")
                }
                var prc byte
                aes_cipher, _ := aes.NewCipher([]byte(aes_key))
                decrypter := cipher.NewCBCDecrypter(aes_cipher,aes_iv)
                decrypter.CryptBlocks(body[16:16 + encrypted_body_len], body[16:16 + encrypted_body_len])
                cm.payload,prc = removePaddingSSL30(body[16:16 + encrypted_body_len])
                if prc == 0 {
                        return fmt.Errorf("parseMessage: padding error from aes static decrypt")
                }

                if cm.m_type == 0 {
                        sindex := bytes.Index(cm.payload,[]byte("\"api_type\":\""))
                        if sindex == -1 {
                                return errNoApiType
                        }
                        sindex += len("\"api_type\":\"")

                        sindex2 := bytes.IndexByte(cm.payload[sindex:],'"')

                        if sindex2 >  1 {
                                var temp_int int
                                temp_int,err = strconv.Atoi(string(cm.payload[sindex:sindex+sindex2]))
                                cm.m_type = uint16(temp_int)
                                return
                        } else {
                                return errApiTypeQuote
                        }
                }

                return nil

        } else if cm.m_crypt_flag  == S_ENC_AES_DYNAMIC && stage == stageMsgloop {



                encrypted_body_len := binary.BigEndian.Uint32(body[12:16])
                if encrypted_body_len % AES_BLOCK_SIZE != 0 {
                        return fmt.Errorf("parseMessage: aes dynamic  encrypted_body_len %d  should be multiple of blocksize(16)",encrypted_body_len)
                }
                if 16 + encrypted_body_len != body_len {
                        return fmt.Errorf("parse_msg: encrypted_body_len error")
                }
                if cm.aes_dynamic_key == nil {
                        return fmt.Errorf("parseMessage: aes dynamic decrypt: aes_dynamic_key is nil")
                }
                var prc byte
                aes_cipher, _ := aes.NewCipher(cm.aes_dynamic_key)
                decrypter := cipher.NewCBCDecrypter(aes_cipher,aes_iv)
                decrypter.CryptBlocks(body[16:16 + encrypted_body_len], body[16:16 + encrypted_body_len])
                cm.payload,prc = removePaddingSSL30(body[16:16 + encrypted_body_len])
                if prc == 0 {
                        return fmt.Errorf("parseMessage: padding error from aes dynamic decrypt")
                }

                if cm.m_type == 0 {
                        sindex := bytes.Index(cm.payload,[]byte("\"api_type\":"))
                        if sindex == -1 {
                                return errNoApiType
                        }

                        sindex += len("\"api_type\":")

                        sindex2 := bytes.IndexByte(cm.payload[sindex:],'"')

                        sindex = sindex + sindex2 + 1

                        sindex2 = bytes.IndexByte(cm.payload[sindex:],'"')
                        if sindex2 >  1 {
                                var temp_int int
                                temp_int,err = strconv.Atoi(string(cm.payload[sindex:sindex+sindex2]))
                                cm.m_type = uint16(temp_int)
                                return
                        } else {
                                return errApiTypeQuote
                        }
                }

                return nil

        } else if cm.m_crypt_flag  == S_ENC_NOT && stage == stageMsgloop {
                //not encrypted
                encrypted_body_len := binary.BigEndian.Uint32(body[12:16])
                
                cm.payload = make([]byte,encrypted_body_len)
                copy(cm.payload,body[16:16 + encrypted_body_len])
                return nil


        } else {

                return fmt.Errorf("parseMessage: stage and enc type not match")
        }
        //should not be here
        return nil
}




//msg = body + len,  msg is allocated by calling function
func (cm *channelMessage)assembly(msg []byte,payload []byte) (msg_len uint32,err error){

        //encode field
        binary.BigEndian.PutUint16(msg[4:6],cm.m_compress_flag)
        binary.BigEndian.PutUint16(msg[6:8],cm.m_crypt_flag)
        binary.BigEndian.PutUint16(msg[8:10],cm.m_code)
        binary.BigEndian.PutUint16(msg[10:12],cm.m_type)
        binary.BigEndian.PutUint32(msg[12:16],cm.m_req_id)

	payload_len := uint32(len(payload))
        if payload_len > 0 {
                if cm.m_crypt_flag  == S_ENC_NOT {
                        //copy payload
                        binary.BigEndian.PutUint32(msg[16:20],payload_len)
                        copy(msg[20:],payload)

                        msg_len = 16 + 4 + payload_len
                        if msg_len > uint32(len(msg)) {
                                return 0,fmt.Errorf("parse_message: body buffer overflow %d , %d",msg_len,len(msg))
                        }

                        binary.BigEndian.PutUint32(msg[0:4],msg_len)

                } else if cm.m_crypt_flag  == S_ENC_AES_STATIC {
                        aes_cipher, _ := aes.NewCipher([]byte(aes_key))
                        encrypter := cipher.NewCBCEncrypter(aes_cipher,aes_iv)
                        prefix, finalBlock := padToBlockSize(payload, AES_BLOCK_SIZE)

                        payload_len = uint32(len(prefix)) + uint32(len(finalBlock))

                        msg_len = 16 + 4 + payload_len
                        if msg_len > uint32(len(msg)) {
                                return 0,fmt.Errorf("parse_message: body buffer overflow %d , %d",msg_len,len(msg))
                        }

                        binary.BigEndian.PutUint32(msg[0:4],msg_len)

                        //encode payload
                        binary.BigEndian.PutUint32(msg[16:20],payload_len)

                        encrypter.CryptBlocks(msg[20:],prefix)
                        encrypter.CryptBlocks(msg[20 + len(prefix):],finalBlock)

                } else if cm.m_crypt_flag  == S_ENC_AES_DYNAMIC {

                        aes_cipher, _ := aes.NewCipher([]byte(cm.aes_dynamic_key))
                        encrypter := cipher.NewCBCEncrypter(aes_cipher,aes_iv)
                        prefix, finalBlock := padToBlockSize(payload, AES_BLOCK_SIZE)

                        payload_len = uint32(len(prefix)) + uint32(len(finalBlock))

                        msg_len = 16 + 4 + payload_len
                        if msg_len > uint32(len(msg)) {
                                return 0,fmt.Errorf("parse_message: body buffer overflow %d , %d",msg_len,len(msg))
                        }

                        binary.BigEndian.PutUint32(msg[0:4],msg_len)

                        //encode payload
                        binary.BigEndian.PutUint32(msg[16:20],payload_len)

                        encrypter.CryptBlocks(msg[20:],prefix)
                        encrypter.CryptBlocks(msg[20 + len(prefix):],finalBlock)

                }
        } else {
                //header only
                msg_len = 16
                binary.BigEndian.PutUint32(msg[0:4],msg_len)
        }

        return msg_len,nil
}
