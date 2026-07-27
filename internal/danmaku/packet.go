package danmaku

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"github.com/andybalholm/brotli"
)

// Packet 表示一个完整的 B站 WebSocket 数据包
type Packet struct {
	Header  PacketHeader
	Payload []byte
}

// ReadPacket 从字节流中读取一个包
func ReadPacket(data []byte) (*Packet, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("数据包太短: %d 字节（最少需要 16 字节）", len(data))
	}

	header := PacketHeader{
		TotalSize:       int32(binary.BigEndian.Uint32(data[0:4])),
		HeaderLength:    int16(binary.BigEndian.Uint16(data[4:6])),
		ProtocolVersion: ProtocolVersion(binary.BigEndian.Uint16(data[6:8])),
		Operation:       OperationCode(binary.BigEndian.Uint32(data[8:12])),
		SequenceID:      int32(binary.BigEndian.Uint32(data[12:16])),
	}

	if int32(len(data)) < header.TotalSize {
		return nil, fmt.Errorf("数据不完整: 需要 %d 字节，实际 %d 字节", header.TotalSize, len(data))
	}

	payloadStart := int(header.HeaderLength)
	payloadEnd := int(header.TotalSize)
	payload := data[payloadStart:payloadEnd]

	return &Packet{
		Header:  header,
		Payload: payload,
	}, nil
}

// DecompressPayload 解压负载数据（支持 zlib 和 brotli）
func (p *Packet) DecompressPayload() ([]byte, error) {
	switch p.Header.ProtocolVersion {
	case ProtoPlain, ProtoJSON:
		// 未压缩
		return p.Payload, nil
	case ProtoZlib:
		return decompressZlib(p.Payload)
	case ProtoBrotli:
		return decompressBrotli(p.Payload)
	default:
		return nil, fmt.Errorf("不支持的协议版本: %d", p.Header.ProtocolVersion)
	}
}

// decompressZlib 对 zlib 压缩数据解压
func decompressZlib(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("创建 zlib reader 失败: %w", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("zlib 解压失败: %w", err)
	}
	return decompressed, nil
}

// decompressBrotli 对 brotli 压缩数据解压
func decompressBrotli(data []byte) ([]byte, error) {
	reader := brotli.NewReader(bytes.NewReader(data))
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("brotli 解压失败: %w", err)
	}
	return decompressed, nil
}

// ParseAsJSON 将包负载解析为 JSON map
func (p *Packet) ParseAsJSON() (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(p.Payload, &result); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	return result, nil
}

// NewHeartbeatPacket 创建心跳包
func NewHeartbeatPacket() []byte {
	return NewPacket(OpHeartbeat, ProtoJSON, []byte("[object Object]"))
}

// NewAuthPacket 创建认证包
func NewAuthPacket(uid int64, roomID int64, token string) []byte {
	authData := map[string]interface{}{
		"uid":      uid,
		"roomid":   roomID,
		"protover": 3,
		"token":    token,
		"platform": "web",
		"type":     2,
	}
	payload, _ := json.Marshal(authData)
	return NewPacket(OpAuth, ProtoJSON, payload)
}

// NewPacket 创建一个 WebSocket 包（字节序列）
func NewPacket(op OperationCode, proto ProtocolVersion, payload []byte) []byte {
	headerLen := int16(16)
	totalSize := int32(headerLen) + int32(len(payload))

	data := make([]byte, totalSize)
	binary.BigEndian.PutUint32(data[0:4], uint32(totalSize))
	binary.BigEndian.PutUint16(data[4:6], uint16(headerLen))
	binary.BigEndian.PutUint16(data[6:8], uint16(proto))
	binary.BigEndian.PutUint32(data[8:12], uint32(op))
	binary.BigEndian.PutUint32(data[12:16], 1) // sequence ID = 1
	copy(data[16:], payload)

	return data
}
