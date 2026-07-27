package openlive

import "encoding/json"

// StartResponse /v2/app/start 接口响应
type StartResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    *StartData `json:"data"`
}

// StartData start 接口返回数据
type StartData struct {
	GameInfo      GameInfo      `json:"game_info"`
	WebsocketInfo WebsocketInfo `json:"websocket_info"`
	AnchorInfo    AnchorInfo    `json:"anchor_info"`
}

// GameInfo 场次信息
type GameInfo struct {
	GameID string `json:"game_id"`
}

// WebsocketInfo WebSocket 连接信息
type WebsocketInfo struct {
	AuthBody string   `json:"auth_body"`
	WssLink  []string `json:"wss_link"`
}

// AnchorInfo 主播信息
type AnchorInfo struct {
	RoomID     int64  `json:"room_id"`
	UID        int64  `json:"uid"`
	UserName   string `json:"user_name"`
	RoomName   string `json:"room_name"`
	LiveStatus int    `json:"live_status"`
}

// HeartbeatResponse /v2/app/heartbeat 响应
type HeartbeatResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// EndResponse /v2/app/end 响应
type EndResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// DanmakuData 开放平台弹幕消息
type DanmakuData struct {
	RoomID    int64  `json:"room_id"`
	UID       int64  `json:"uid"`
	UserName  string `json:"uname"`
	Content   string `json:"msg"`
	Timestamp int64  `json:"timestamp"`
}

// CmdMessage 通用 WebSocket 消息结构
type CmdMessage struct {
	Cmd  string          `json:"cmd"`
	Data json.RawMessage `json:"data"`
}
