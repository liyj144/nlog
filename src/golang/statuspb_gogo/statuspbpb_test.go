// Code generated by protoc-gen-gogo.
// source: statuspb.proto
// DO NOT EDIT!

/*
Package statuspb_gogo is a generated protocol buffer package.

It is generated from these files:
	statuspb.proto

It has these top-level messages:
	StatusRequest
	StatusResponse
	StatusQueryResult
	StatusQueryResponse
	StatusMap
	StringToIntMap
	AnonymousMap
*/
package statuspb_gogo

import testing "testing"
import math_rand "math/rand"
import github_com_gogo_protobuf_proto "github.com/gogo/protobuf/proto"

func BenchmarkStatusRequestProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StatusRequest, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedStatusRequest(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(data)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusRequestProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedStatusRequest(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = data
	}
	msg := &StatusRequest{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusResponseProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StatusResponse, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedStatusResponse(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(data)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusResponseProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedStatusResponse(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = data
	}
	msg := &StatusResponse{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusQueryResultProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StatusQueryResult, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedStatusQueryResult(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(data)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusQueryResultProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedStatusQueryResult(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = data
	}
	msg := &StatusQueryResult{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusQueryResponseProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StatusQueryResponse, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedStatusQueryResponse(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(data)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusQueryResponseProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedStatusQueryResponse(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = data
	}
	msg := &StatusQueryResponse{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusMapProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StatusMap, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedStatusMap(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(data)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusMapProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedStatusMap(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = data
	}
	msg := &StatusMap{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusMap_KVUserStatusProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StatusMap_KVUserStatus, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedStatusMap_KVUserStatus(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(data)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusMap_KVUserStatusProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedStatusMap_KVUserStatus(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = data
	}
	msg := &StatusMap_KVUserStatus{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusMap_KVUserStatus_UserStatusProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StatusMap_KVUserStatus_UserStatus, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedStatusMap_KVUserStatus_UserStatus(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(data)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusMap_KVUserStatus_UserStatusProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedStatusMap_KVUserStatus_UserStatus(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = data
	}
	msg := &StatusMap_KVUserStatus_UserStatus{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStringToIntMapProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StringToIntMap, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedStringToIntMap(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(data)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStringToIntMapProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedStringToIntMap(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = data
	}
	msg := &StringToIntMap{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStringToIntMap_STIProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StringToIntMap_STI, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedStringToIntMap_STI(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(data)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStringToIntMap_STIProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedStringToIntMap_STI(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = data
	}
	msg := &StringToIntMap_STI{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkAnonymousMapProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*AnonymousMap, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedAnonymousMap(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(data)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkAnonymousMapProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedAnonymousMap(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = data
	}
	msg := &AnonymousMap{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkAnonymousMap_AnonymousEntryProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*AnonymousMap_AnonymousEntry, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedAnonymousMap_AnonymousEntry(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(data)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkAnonymousMap_AnonymousEntryProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		data, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedAnonymousMap_AnonymousEntry(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = data
	}
	msg := &AnonymousMap_AnonymousEntry{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusRequestSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StatusRequest, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedStatusRequest(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusResponseSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StatusResponse, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedStatusResponse(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusQueryResultSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StatusQueryResult, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedStatusQueryResult(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusQueryResponseSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StatusQueryResponse, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedStatusQueryResponse(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusMapSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StatusMap, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedStatusMap(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusMap_KVUserStatusSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StatusMap_KVUserStatus, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedStatusMap_KVUserStatus(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStatusMap_KVUserStatus_UserStatusSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StatusMap_KVUserStatus_UserStatus, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedStatusMap_KVUserStatus_UserStatus(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStringToIntMapSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StringToIntMap, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedStringToIntMap(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStringToIntMap_STISize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StringToIntMap_STI, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedStringToIntMap_STI(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkAnonymousMapSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*AnonymousMap, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedAnonymousMap(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkAnonymousMap_AnonymousEntrySize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*AnonymousMap_AnonymousEntry, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedAnonymousMap_AnonymousEntry(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

//These tests are generated by github.com/gogo/protobuf/plugin/testgen