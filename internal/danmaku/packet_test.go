package danmaku

import (
	"bytes"
	"compress/zlib"
	"testing"
)

func makeZlibPayload(data []byte) []byte {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	w.Write(data)
	w.Close()
	return buf.Bytes()
}

func TestReadPacket_Valid(t *testing.T) {
	payload := []byte(`{"cmd":"test"}`)
	data := NewPacket(OpCommand, ProtoJSON, payload)

	pkt, err := ReadPacket(data)
	if err != nil {
		t.Fatalf("ReadPacket 失败: %v", err)
	}

	if pkt.Header.Operation != OpCommand {
		t.Fatalf("操作码期望 %d，实际 %d", OpCommand, pkt.Header.Operation)
	}
	if pkt.Header.ProtocolVersion != ProtoJSON {
		t.Fatalf("协议版本期望 %d，实际 %d", ProtoJSON, pkt.Header.ProtocolVersion)
	}
	if string(pkt.Payload) != string(payload) {
		t.Fatalf("负载不匹配")
	}
}

func TestReadPacket_TooShort(t *testing.T) {
	_, err := ReadPacket([]byte{0, 0, 0, 0})
	if err == nil {
		t.Fatal("过短的数据应返回错误")
	}
}

func TestReadPacket_Heartbeat(t *testing.T) {
	data := NewPacket(OpHeartbeatReply, ProtoJSON, []byte(`{"popularity":12345}`))
	pkt, err := ReadPacket(data)
	if err != nil {
		t.Fatalf("ReadPacket 失败: %v", err)
	}

	if pkt.Header.Operation != OpHeartbeatReply {
		t.Fatalf("操作码期望 %d，实际 %d", OpHeartbeatReply, pkt.Header.Operation)
	}
}

func TestDecompressPayload_Zlib(t *testing.T) {
	original := []byte(`{"cmd":"DANMU_MSG","info":[[0],"hello",["User"],["123"]]}`)
	compressed := makeZlibPayload(original)

	pkt := &Packet{
		Header: PacketHeader{
			ProtocolVersion: ProtoZlib,
		},
		Payload: compressed,
	}

	decompressed, err := pkt.DecompressPayload()
	if err != nil {
		t.Fatalf("解压失败: %v", err)
	}

	if string(decompressed) != string(original) {
		t.Fatalf("解压后数据不匹配:\n期望: %s\n实际: %s", original, decompressed)
	}
}

func TestDecompressPayload_Plain(t *testing.T) {
	payload := []byte(`{"cmd":"test"}`)
	pkt := &Packet{
		Header: PacketHeader{
			ProtocolVersion: ProtoPlain,
		},
		Payload: payload,
	}

	result, err := pkt.DecompressPayload()
	if err != nil {
		t.Fatalf("解压失败: %v", err)
	}
	if string(result) != string(payload) {
		t.Fatalf("未压缩数据应原样返回")
	}
}

func TestNewHeartbeatPacket(t *testing.T) {
	data := NewHeartbeatPacket()
	if len(data) < 16 {
		t.Fatal("心跳包太短")
	}

	pkt, err := ReadPacket(data)
	if err != nil {
		t.Fatalf("解析心跳包失败: %v", err)
	}
	if pkt.Header.Operation != OpHeartbeat {
		t.Fatalf("操作码期望 %d，实际 %d", OpHeartbeat, pkt.Header.Operation)
	}
}

func TestNewAuthPacket(t *testing.T) {
	data := NewAuthPacket(123, 456, "test_token")
	pkt, err := ReadPacket(data)
	if err != nil {
		t.Fatalf("解析认证包失败: %v", err)
	}
	if pkt.Header.Operation != OpAuth {
		t.Fatalf("操作码期望 %d，实际 %d", OpAuth, pkt.Header.Operation)
	}
}

func TestNewPacket_RoundTrip(t *testing.T) {
	payload := []byte(`{"test":"data"}`)
	data := NewPacket(OpCommand, ProtoJSON, payload)

	pkt, err := ReadPacket(data)
	if err != nil {
		t.Fatalf("ReadPacket 失败: %v", err)
	}

	if pkt.Header.Operation != OpCommand {
		t.Fatalf("操作码不匹配")
	}
	if string(pkt.Payload) != string(payload) {
		t.Fatalf("负载不匹配:\n期望: %s\n实际: %s", payload, pkt.Payload)
	}
}
