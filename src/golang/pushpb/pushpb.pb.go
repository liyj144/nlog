// Code generated by protoc-gen-go.
// source: pushpb.proto
// DO NOT EDIT!

/*
Package pushpb is a generated protocol buffer package.

It is generated from these files:
	pushpb.proto

It has these top-level messages:
	PushRequestFromUpstream
	PushRequestToDownstream
*/
package pushpb

import proto "github.com/golang/protobuf/proto"
import math "math"

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
	FieldType        *PushRequestFromUpstream_FieldType `protobuf:"varint,1,req,name=field_type,enum=pushpb.PushRequestFromUpstream_FieldType" json:"field_type,omitempty"`
	UserId           *uint64                            `protobuf:"varint,2,opt,name=user_id" json:"user_id,omitempty"`
	DeviceId         []byte                             `protobuf:"bytes,3,opt,name=device_id" json:"device_id,omitempty"`
	Payload          []byte                             `protobuf:"bytes,4,req,name=payload" json:"payload,omitempty"`
	XXX_unrecognized []byte                             `json:"-"`
}

func (m *PushRequestFromUpstream) Reset()         { *m = PushRequestFromUpstream{} }
func (m *PushRequestFromUpstream) String() string { return proto.CompactTextString(m) }
func (*PushRequestFromUpstream) ProtoMessage()    {}

func (m *PushRequestFromUpstream) GetFieldType() PushRequestFromUpstream_FieldType {
	if m != nil && m.FieldType != nil {
		return *m.FieldType
	}
	return PushRequestFromUpstream_UID
}

func (m *PushRequestFromUpstream) GetUserId() uint64 {
	if m != nil && m.UserId != nil {
		return *m.UserId
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
	NetId            *uint32 `protobuf:"varint,1,req,name=net_id" json:"net_id,omitempty"`
	Payload          []byte  `protobuf:"bytes,2,req,name=payload" json:"payload,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *PushRequestToDownstream) Reset()         { *m = PushRequestToDownstream{} }
func (m *PushRequestToDownstream) String() string { return proto.CompactTextString(m) }
func (*PushRequestToDownstream) ProtoMessage()    {}

func (m *PushRequestToDownstream) GetNetId() uint32 {
	if m != nil && m.NetId != nil {
		return *m.NetId
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
	proto.RegisterEnum("pushpb.PushRequestFromUpstream_FieldType", PushRequestFromUpstream_FieldType_name, PushRequestFromUpstream_FieldType_value)
}
