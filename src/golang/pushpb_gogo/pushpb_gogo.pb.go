// Code generated by protoc-gen-gogo.
// source: pushpb_gogo.proto
// DO NOT EDIT!

/*
	Package pushpb_gogo is a generated protocol buffer package.

	It is generated from these files:
		pushpb_gogo.proto

	It has these top-level messages:
		PushRequestFromUpstream
		PushRequestToDownstream
*/
package pushpb_gogo

import proto "github.com/gogo/protobuf/proto"
import math "math"

// discarding unused import gogoproto "github.com/gogo/protobuf/gogoproto/gogo.pb"

import io "io"
import fmt "fmt"
import github_com_gogo_protobuf_proto "github.com/gogo/protobuf/proto"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = math.Inf

type PushRequestFromUpstream_FieldType int32

const (
	PushRequestFromUpstream_UID      PushRequestFromUpstream_FieldType = 0
	PushRequestFromUpstream_DEVICEID PushRequestFromUpstream_FieldType = 1
)

var PushRequestFromUpstream_FieldType_name = map[int32]string{
	0: "UID",
	1: "DEVICEID",
}
var PushRequestFromUpstream_FieldType_value = map[string]int32{
	"UID":      0,
	"DEVICEID": 1,
}

func (x PushRequestFromUpstream_FieldType) Enum() *PushRequestFromUpstream_FieldType {
	p := new(PushRequestFromUpstream_FieldType)
	*p = x
	return p
}
func (x PushRequestFromUpstream_FieldType) String() string {
	return proto.EnumName(PushRequestFromUpstream_FieldType_name, int32(x))
}
func (x *PushRequestFromUpstream_FieldType) UnmarshalJSON(data []byte) error {
	value, err := proto.UnmarshalJSONEnum(PushRequestFromUpstream_FieldType_value, data, "PushRequestFromUpstream_FieldType")
	if err != nil {
		return err
	}
	*x = PushRequestFromUpstream_FieldType(value)
	return nil
}

type PushRequestFromUpstream struct {
	FieldType        PushRequestFromUpstream_FieldType `protobuf:"varint,1,req,name=field_type,enum=pushpb_gogo.PushRequestFromUpstream_FieldType" json:"field_type"`
	UserId           uint64                            `protobuf:"varint,2,opt,name=user_id" json:"user_id"`
	DeviceId         []byte                            `protobuf:"bytes,3,opt,name=device_id" json:"device_id,omitempty"`
	Payload          []byte                            `protobuf:"bytes,4,req,name=payload" json:"payload,omitempty"`
	XXX_unrecognized []byte                            `json:"-"`
}

func (m *PushRequestFromUpstream) Reset()         { *m = PushRequestFromUpstream{} }
func (m *PushRequestFromUpstream) String() string { return proto.CompactTextString(m) }
func (*PushRequestFromUpstream) ProtoMessage()    {}

func (m *PushRequestFromUpstream) GetFieldType() PushRequestFromUpstream_FieldType {
	if m != nil {
		return m.FieldType
	}
	return PushRequestFromUpstream_UID
}

func (m *PushRequestFromUpstream) GetUserId() uint64 {
	if m != nil {
		return m.UserId
	}
	return 0
}

func (m *PushRequestFromUpstream) GetDeviceId() []byte {
	if m != nil {
		return m.DeviceId
	}
	return nil
}

func (m *PushRequestFromUpstream) GetPayload() []byte {
	if m != nil {
		return m.Payload
	}
	return nil
}

type PushRequestToDownstream struct {
	NetId            uint32 `protobuf:"varint,1,req,name=net_id" json:"net_id"`
	Payload          []byte `protobuf:"bytes,2,req,name=payload" json:"payload,omitempty"`
	XXX_unrecognized []byte `json:"-"`
}

func (m *PushRequestToDownstream) Reset()         { *m = PushRequestToDownstream{} }
func (m *PushRequestToDownstream) String() string { return proto.CompactTextString(m) }
func (*PushRequestToDownstream) ProtoMessage()    {}

func (m *PushRequestToDownstream) GetNetId() uint32 {
	if m != nil {
		return m.NetId
	}
	return 0
}

func (m *PushRequestToDownstream) GetPayload() []byte {
	if m != nil {
		return m.Payload
	}
	return nil
}

func init() {
	proto.RegisterEnum("pushpb_gogo.PushRequestFromUpstream_FieldType", PushRequestFromUpstream_FieldType_name, PushRequestFromUpstream_FieldType_value)
}
func (m *PushRequestFromUpstream) Unmarshal(data []byte) error {
	l := len(data)
	index := 0
	for index < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if index >= l {
				return io.ErrUnexpectedEOF
			}
			b := data[index]
			index++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field FieldType", wireType)
			}
			for shift := uint(0); ; shift += 7 {
				if index >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[index]
				index++
				m.FieldType |= (PushRequestFromUpstream_FieldType(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field UserId", wireType)
			}
			for shift := uint(0); ; shift += 7 {
				if index >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[index]
				index++
				m.UserId |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field DeviceId", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if index >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[index]
				index++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			postIndex := index + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.DeviceId = append([]byte{}, data[index:postIndex]...)
			index = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Payload", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if index >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[index]
				index++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			postIndex := index + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Payload = append([]byte{}, data[index:postIndex]...)
			index = postIndex
		default:
			var sizeOfWire int
			for {
				sizeOfWire++
				wire >>= 7
				if wire == 0 {
					break
				}
			}
			index -= sizeOfWire
			skippy, err := github_com_gogo_protobuf_proto.Skip(data[index:])
			if err != nil {
				return err
			}
			if (index + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, data[index:index+skippy]...)
			index += skippy
		}
	}
	return nil
}
func (m *PushRequestToDownstream) Unmarshal(data []byte) error {
	l := len(data)
	index := 0
	for index < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if index >= l {
				return io.ErrUnexpectedEOF
			}
			b := data[index]
			index++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field NetId", wireType)
			}
			for shift := uint(0); ; shift += 7 {
				if index >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[index]
				index++
				m.NetId |= (uint32(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Payload", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if index >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[index]
				index++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			postIndex := index + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Payload = append([]byte{}, data[index:postIndex]...)
			index = postIndex
		default:
			var sizeOfWire int
			for {
				sizeOfWire++
				wire >>= 7
				if wire == 0 {
					break
				}
			}
			index -= sizeOfWire
			skippy, err := github_com_gogo_protobuf_proto.Skip(data[index:])
			if err != nil {
				return err
			}
			if (index + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, data[index:index+skippy]...)
			index += skippy
		}
	}
	return nil
}
func (m *PushRequestFromUpstream) Size() (n int) {
	var l int
	_ = l
	n += 1 + sovPushpbGogo(uint64(m.FieldType))
	n += 1 + sovPushpbGogo(uint64(m.UserId))
	if m.DeviceId != nil {
		l = len(m.DeviceId)
		n += 1 + l + sovPushpbGogo(uint64(l))
	}
	if m.Payload != nil {
		l = len(m.Payload)
		n += 1 + l + sovPushpbGogo(uint64(l))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *PushRequestToDownstream) Size() (n int) {
	var l int
	_ = l
	n += 1 + sovPushpbGogo(uint64(m.NetId))
	if m.Payload != nil {
		l = len(m.Payload)
		n += 1 + l + sovPushpbGogo(uint64(l))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovPushpbGogo(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozPushpbGogo(x uint64) (n int) {
	return sovPushpbGogo(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func NewPopulatedPushRequestFromUpstream(r randyPushpbGogo, easy bool) *PushRequestFromUpstream {
	this := &PushRequestFromUpstream{}
	this.FieldType = PushRequestFromUpstream_FieldType([]int32{0, 1}[r.Intn(2)])
	this.UserId = uint64(r.Uint32())
	if r.Intn(10) != 0 {
		v1 := r.Intn(100)
		this.DeviceId = make([]byte, v1)
		for i := 0; i < v1; i++ {
			this.DeviceId[i] = byte(r.Intn(256))
		}
	}
	v2 := r.Intn(100)
	this.Payload = make([]byte, v2)
	for i := 0; i < v2; i++ {
		this.Payload[i] = byte(r.Intn(256))
	}
	if !easy && r.Intn(10) != 0 {
		this.XXX_unrecognized = randUnrecognizedPushpbGogo(r, 5)
	}
	return this
}

func NewPopulatedPushRequestToDownstream(r randyPushpbGogo, easy bool) *PushRequestToDownstream {
	this := &PushRequestToDownstream{}
	this.NetId = r.Uint32()
	v3 := r.Intn(100)
	this.Payload = make([]byte, v3)
	for i := 0; i < v3; i++ {
		this.Payload[i] = byte(r.Intn(256))
	}
	if !easy && r.Intn(10) != 0 {
		this.XXX_unrecognized = randUnrecognizedPushpbGogo(r, 3)
	}
	return this
}

type randyPushpbGogo interface {
	Float32() float32
	Float64() float64
	Int63() int64
	Int31() int32
	Uint32() uint32
	Intn(n int) int
}

func randUTF8RunePushpbGogo(r randyPushpbGogo) rune {
	return rune(r.Intn(126-43) + 43)
}
func randStringPushpbGogo(r randyPushpbGogo) string {
	v4 := r.Intn(100)
	tmps := make([]rune, v4)
	for i := 0; i < v4; i++ {
		tmps[i] = randUTF8RunePushpbGogo(r)
	}
	return string(tmps)
}
func randUnrecognizedPushpbGogo(r randyPushpbGogo, maxFieldNumber int) (data []byte) {
	l := r.Intn(5)
	for i := 0; i < l; i++ {
		wire := r.Intn(4)
		if wire == 3 {
			wire = 5
		}
		fieldNumber := maxFieldNumber + r.Intn(100)
		data = randFieldPushpbGogo(data, r, fieldNumber, wire)
	}
	return data
}
func randFieldPushpbGogo(data []byte, r randyPushpbGogo, fieldNumber int, wire int) []byte {
	key := uint32(fieldNumber)<<3 | uint32(wire)
	switch wire {
	case 0:
		data = encodeVarintPopulatePushpbGogo(data, uint64(key))
		v5 := r.Int63()
		if r.Intn(2) == 0 {
			v5 *= -1
		}
		data = encodeVarintPopulatePushpbGogo(data, uint64(v5))
	case 1:
		data = encodeVarintPopulatePushpbGogo(data, uint64(key))
		data = append(data, byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)))
	case 2:
		data = encodeVarintPopulatePushpbGogo(data, uint64(key))
		ll := r.Intn(100)
		data = encodeVarintPopulatePushpbGogo(data, uint64(ll))
		for j := 0; j < ll; j++ {
			data = append(data, byte(r.Intn(256)))
		}
	default:
		data = encodeVarintPopulatePushpbGogo(data, uint64(key))
		data = append(data, byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)))
	}
	return data
}
func encodeVarintPopulatePushpbGogo(data []byte, v uint64) []byte {
	for v >= 1<<7 {
		data = append(data, uint8(uint64(v)&0x7f|0x80))
		v >>= 7
	}
	data = append(data, uint8(v))
	return data
}
func (m *PushRequestFromUpstream) Marshal() (data []byte, err error) {
	size := m.Size()
	data = make([]byte, size)
	n, err := m.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

func (m *PushRequestFromUpstream) MarshalTo(data []byte) (n int, err error) {
	var i int
	_ = i
	var l int
	_ = l
	data[i] = 0x8
	i++
	i = encodeVarintPushpbGogo(data, i, uint64(m.FieldType))
	data[i] = 0x10
	i++
	i = encodeVarintPushpbGogo(data, i, uint64(m.UserId))
	if m.DeviceId != nil {
		data[i] = 0x1a
		i++
		i = encodeVarintPushpbGogo(data, i, uint64(len(m.DeviceId)))
		i += copy(data[i:], m.DeviceId)
	}
	if m.Payload != nil {
		data[i] = 0x22
		i++
		i = encodeVarintPushpbGogo(data, i, uint64(len(m.Payload)))
		i += copy(data[i:], m.Payload)
	}
	if m.XXX_unrecognized != nil {
		i += copy(data[i:], m.XXX_unrecognized)
	}
	return i, nil
}

func (m *PushRequestToDownstream) Marshal() (data []byte, err error) {
	size := m.Size()
	data = make([]byte, size)
	n, err := m.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

func (m *PushRequestToDownstream) MarshalTo(data []byte) (n int, err error) {
	var i int
	_ = i
	var l int
	_ = l
	data[i] = 0x8
	i++
	i = encodeVarintPushpbGogo(data, i, uint64(m.NetId))
	if m.Payload != nil {
		data[i] = 0x12
		i++
		i = encodeVarintPushpbGogo(data, i, uint64(len(m.Payload)))
		i += copy(data[i:], m.Payload)
	}
	if m.XXX_unrecognized != nil {
		i += copy(data[i:], m.XXX_unrecognized)
	}
	return i, nil
}

func encodeFixed64PushpbGogo(data []byte, offset int, v uint64) int {
	data[offset] = uint8(v)
	data[offset+1] = uint8(v >> 8)
	data[offset+2] = uint8(v >> 16)
	data[offset+3] = uint8(v >> 24)
	data[offset+4] = uint8(v >> 32)
	data[offset+5] = uint8(v >> 40)
	data[offset+6] = uint8(v >> 48)
	data[offset+7] = uint8(v >> 56)
	return offset + 8
}
func encodeFixed32PushpbGogo(data []byte, offset int, v uint32) int {
	data[offset] = uint8(v)
	data[offset+1] = uint8(v >> 8)
	data[offset+2] = uint8(v >> 16)
	data[offset+3] = uint8(v >> 24)
	return offset + 4
}
func encodeVarintPushpbGogo(data []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		data[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	data[offset] = uint8(v)
	return offset + 1
}
