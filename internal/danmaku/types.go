// Package danmaku 提供弹幕协议解析
package danmaku

import "time"

// OperationCode 定义 WebSocket 包操作码
type OperationCode int32

const (
	OpHeartbeat     OperationCode = 2 // 心跳
	OpHeartbeatReply OperationCode = 3 // 心跳回复（含人气值）
	OpCommand       OperationCode = 5 // 命令包（含弹幕等）
	OpAuth          OperationCode = 7 // 认证包
	OpAuthReply     OperationCode = 8 // 认证回复
)

// ProtocolVersion 定义包协议版本
type ProtocolVersion int16

const (
	ProtoPlain   ProtocolVersion = 0 // 普通 JSON
	ProtoJSON    ProtocolVersion = 1 // 普通 JSON
	ProtoZlib    ProtocolVersion = 2 // zlib 压缩
	ProtoBrotli  ProtocolVersion = 3 // brotli 压缩
)

// PacketHeader 表示 B站 WebSocket 二进制包头部（16 字节，大端序）
type PacketHeader struct {
	TotalSize      int32
	HeaderLength   int16
	ProtocolVersion ProtocolVersion
	Operation      OperationCode
	SequenceID     int32
}

// DanmakuEvent 表示一条解析后的弹幕事件
type DanmakuEvent struct {
	RoomID    int64     `json:"room_id"`
	UserID    int64     `json:"user_id"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Extra     string    `json:"extra,omitempty"`
}

// AuthReply 表示认证回复
type AuthReply struct {
	Code int `json:"code"`
}

// HeartbeatReply 表示心跳回复（含人气值）
type HeartbeatReply struct {
	Popularity int `json:"popularity"`
}
