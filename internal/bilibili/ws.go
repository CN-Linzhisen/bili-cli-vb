package bilibili

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/CN-Linzhisen/bili-cli-vb/internal/danmaku"
	"github.com/gorilla/websocket"
)

const (
	wsURLTemplate     = "wss://%s:%d/sub"
	heartbeatInterval = 30 * time.Second
	reconnectDelay    = 3 * time.Second
	wsWriteTimeout    = 10 * time.Second
)

// ConnState 表示 WebSocket 连接状态
type ConnState int

const (
	StateDisconnected ConnState = iota
	StateConnecting
	StateConnected
	StateReconnecting
)

// RoomWatcher 管理到单个直播间的 WebSocket 连接
type RoomWatcher struct {
	roomID   int64
	sessData string
	buvid3   string
	uid      string
	danmuInfo *DanmuInfo

	conn   *websocket.Conn
	mu     sync.Mutex
	state  ConnState

	events   chan *danmaku.DanmakuEvent
	done     chan struct{}
	stopOnce sync.Once
}

// NewRoomWatcher 创建新的弹幕监听器
func NewRoomWatcher(roomID int64, sessData, buvid3, uid string) *RoomWatcher {
	return &RoomWatcher{
		roomID:   roomID,
		sessData: sessData,
		buvid3:   buvid3,
		uid:      uid,
		events:   make(chan *danmaku.DanmakuEvent, 256),
		done:     make(chan struct{}),
		state:    StateDisconnected,
	}
}

// Events 返回弹幕事件通道
func (rw *RoomWatcher) Events() <-chan *danmaku.DanmakuEvent {
	return rw.events
}

// State 返回当前连接状态
// RoomID 返回直播间ID
func (rw *RoomWatcher) RoomID() int64 {
	return rw.roomID
}

func (rw *RoomWatcher) State() ConnState {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return rw.state
}

// Start 启动弹幕监听（阻塞，直到连接断开或调用 Stop）
func (rw *RoomWatcher) Start() error {
	for {
		if err := rw.connect(); err != nil {
			rw.setState(StateReconnecting)
			log.Printf("连接失败: %v，%v 后重试...", err, reconnectDelay)
			select {
			case <-time.After(reconnectDelay):
				continue
			case <-rw.done:
				return fmt.Errorf("已停止")
			}
		}

		// 连接成功，开始读取循环
		rw.readLoop()

		// 读取循环退出（断开），检查是否应该重连
		select {
		case <-rw.done:
			return nil
		default:
			rw.setState(StateReconnecting)
			log.Printf("连接已断开，%v 后重连...", reconnectDelay)
			time.Sleep(reconnectDelay)
		}
	}
}

// connect 建立 WebSocket 连接并认证
func (rw *RoomWatcher) connect() error {
	rw.setState(StateConnecting)

	// 1. 获取弹幕服务器信息
	client := DefaultHTTPClient()
	info, err := FetchDanmuInfo(&client, rw.roomID, rw.sessData, rw.buvid3)
	if err != nil {
		return fmt.Errorf("获取弹幕服务器信息失败: %w", err)
	}
	rw.danmuInfo = info

	if len(info.HostList) == 0 {
		return fmt.Errorf("未获取到弹幕服务器地址")
	}

	// 调试：输出 getDanmuInfo 返回的完整信息
	for i, h := range info.HostList {
		log.Printf("getDanmuInfo host[%d]: host=%s wss_port=%d ws_port=%d", i, h.Host, h.WssPort, h.WSPort)
	}
	log.Printf("getDanmuInfo token=%q (len=%d)", info.Token, len(info.Token))

	// 2. 建立 WebSocket 连接（带浏览器请求头）
	var conn *websocket.Conn
	dialer := &websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	wsHeaders := http.Header{
		"User-Agent": []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"},
		"Origin":     []string{"https://live.bilibili.com"},
	}

	// 尝试 host 列表中的每个服务器（wss_port 为 0 时回退到 443）
	for i, hostInfo := range info.HostList {
		wsPort := hostInfo.WssPort
		if wsPort == 0 {
			wsPort = 443
		}
		u := url.URL{
			Scheme: "wss",
			Host:   fmt.Sprintf("%s:%d", hostInfo.Host, wsPort),
			Path:   "/sub",
		}

		conn, _, err = dialer.Dial(u.String(), wsHeaders)
		if err == nil {
			break
		}
		if i < len(info.HostList)-1 {
			log.Printf("WebSocket 连接 %s 失败: %v，尝试下一个...", u.Host, err)
		}
	}
	if conn == nil {
		return fmt.Errorf("WebSocket 连接失败，已尝试所有服务器")
	}
	rw.conn = conn

	// 3. 发送认证包（尝试使用真实 uid）
	uid := int64(0)
	if rw.uid != "" {
		if parsed, err := fmt.Sscanf(rw.uid, "%d", &uid); err != nil || parsed != 1 {
			uid = 0
		}
	}
	authPacket := danmaku.NewAuthPacket(uid, rw.roomID, info.Token)
	log.Printf("发送认证包: uid=%d roomID=%d token=%q", uid, rw.roomID, info.Token)
	if err := conn.WriteMessage(websocket.BinaryMessage, authPacket); err != nil {
		conn.Close()
		return fmt.Errorf("发送认证包失败: %w", err)
	}

	// 4. 等待认证回复
	_, msg, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return fmt.Errorf("读取认证回复失败: %w", err)
	}
	log.Printf("收到认证回复: %d 字节", len(msg))

	pkt, err := danmaku.ReadPacket(msg)
	if err != nil {
		conn.Close()
		return fmt.Errorf("解析认证回复包失败: %w", err)
	}

	if pkt.Header.Operation != danmaku.OpAuthReply {
		conn.Close()
		return fmt.Errorf("预期认证回复(op=8)，收到 op=%d", pkt.Header.Operation)
	}

	payload, err := pkt.DecompressPayload()
	if err != nil {
		conn.Close()
		return err
	}

	var authReply danmaku.AuthReply
	if err := json.Unmarshal(payload, &authReply); err != nil {
		conn.Close()
		return fmt.Errorf("解析认证回复 JSON 失败: %w", err)
	}

	if authReply.Code != 0 {
		conn.Close()
		return fmt.Errorf("认证失败 (code=%d)", authReply.Code)
	}

	rw.setState(StateConnected)
	log.Printf("已连接到直播间 %d", rw.roomID)

	// 5. 启动心跳
	go rw.heartbeatLoop()

	return nil
}

// readLoop 持续读取消息并解析弹幕
func (rw *RoomWatcher) readLoop() {
	defer func() {
		rw.mu.Lock()
		if rw.conn != nil {
			rw.conn.Close()
			rw.conn = nil
		}
		rw.mu.Unlock()
	}()

	for {
		select {
		case <-rw.done:
			return
		default:
		}

		// 在锁外读消息（ReadMessage 是阻塞操作，持锁会阻塞 Stop）
		rw.mu.Lock()
		conn := rw.conn
		rw.mu.Unlock()

		if conn == nil {
			return
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("读取消息失败: %v", err)
			return
		}

		rw.handleMessage(msg)
	}
}

// handleMessage 处理接收到的原始消息
func (rw *RoomWatcher) handleMessage(msg []byte) {
	// 尝试解析为单个包
	pkt, err := danmaku.ReadPacket(msg)
	if err != nil {
		log.Printf("解析包失败: %v", err)
		return
	}

	// 对于压缩包，先解压，然后解压后的数据可能包含多个包
	payload, err := pkt.DecompressPayload()
	if err != nil {
		log.Printf("解压失败: %v", err)
		return
	}

	switch pkt.Header.Operation {
	case danmaku.OpHeartbeatReply:
		// 心跳回复，包含人气值（可选，不做特殊处理）
		var reply danmaku.HeartbeatReply
		if err := json.Unmarshal(payload, &reply); err == nil {
			log.Printf("当前人气: %d", reply.Popularity)
		}

	case danmaku.OpCommand:
		// 如果是压缩过的，payload 可能包含多个包
		rw.parseCommandPackets(payload)

	default:
		// 其他操作码忽略
	}
}

// parseCommandPackets 解析可能包含多个命令包的数据
func (rw *RoomWatcher) parseCommandPackets(data []byte) {
	// 尝试将整个数据作为一个 JSON 解析（未压缩/单包情况）
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		rw.handleCommandJSON(raw)
		return
	}

	// 尝试按多包序列解析（压缩包中可能包含多个包）
	offset := 0
	for offset < len(data) {
		if offset+16 > len(data) {
			break
		}

		pkt, err := danmaku.ReadPacket(data[offset:])
		if err != nil {
			break
		}

		payload, err := pkt.DecompressPayload()
		if err != nil {
			offset += int(pkt.Header.TotalSize)
			continue
		}

		if pkt.Header.Operation == danmaku.OpCommand {
			var rawCmd json.RawMessage
			if err := json.Unmarshal(payload, &rawCmd); err == nil {
				rw.handleCommandJSON(rawCmd)
			}
		}

		offset += int(pkt.Header.TotalSize)
	}
}

// handleCommandJSON 处理单个命令 JSON
func (rw *RoomWatcher) handleCommandJSON(raw json.RawMessage) {
	var cmdMap map[string]interface{}
	if err := json.Unmarshal(raw, &cmdMap); err != nil {
		return
	}

	if !danmaku.IsDanmakuCmd(cmdMap) {
		return
	}

	event, err := danmaku.ParseDanmakuEvent(cmdMap, rw.roomID)
	if err != nil {
		log.Printf("解析弹幕事件失败: %v", err)
		return
	}

	select {
	case rw.events <- event:
	default:
	}
}

// heartbeatLoop 定时发送心跳包
func (rw *RoomWatcher) heartbeatLoop() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rw.done:
			return
		case <-ticker.C:
			rw.mu.Lock()
			conn := rw.conn
			rw.mu.Unlock()

			if conn == nil {
				return
			}

			heartbeat := danmaku.NewHeartbeatPacket()
			conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := conn.WriteMessage(websocket.BinaryMessage, heartbeat); err != nil {
				log.Printf("发送心跳失败: %v", err)
				return
			}
		}
	}
}

// Stop 停止监听
func (rw *RoomWatcher) Stop() {
	rw.stopOnce.Do(func() {
		close(rw.done)
		rw.mu.Lock()
		if rw.conn != nil {
			rw.conn.Close()
			rw.conn = nil
		}
		rw.mu.Unlock()
	})
}

func (rw *RoomWatcher) setState(s ConnState) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	rw.state = s
}
