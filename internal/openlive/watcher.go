package openlive

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/CN-Linzhisen/bili-cli-vb/internal/danmaku"
	"github.com/gorilla/websocket"
)

const (
	heartbeatInterval = 30 * time.Second // WebSocket 心跳间隔
	restHeartbeat     = 30 * time.Second // REST API 项目心跳间隔
	wsWriteTimeout    = 10 * time.Second
	reconnectDelay    = 3 * time.Second
)

// ConnState 连接状态
type ConnState int

const (
	StateDisconnected ConnState = iota
	StateConnecting
	StateConnected
	StateReconnecting
)

// Watcher 管理开放平台 WebSocket 连接
type Watcher struct {
	client *Client
	roomID int64

	conn     *websocket.Conn
	gameID   string
	authBody string
	wssLinks []string

	mu       sync.Mutex
	state    ConnState

	events   chan *danmaku.DanmakuEvent
	done     chan struct{}
	stopOnce sync.Once

	// OnConnected 连接成功后调用（在 connect() 末尾触发）
	OnConnected func(roomID int64)
}

// NewWatcher 创建开放平台弹幕监听器
func NewWatcher(apiClient *Client) *Watcher {
	return &Watcher{
		client: apiClient,
		events: make(chan *danmaku.DanmakuEvent, 256),
		done:   make(chan struct{}),
		state:  StateDisconnected,
	}
}

// Events 返回弹幕事件通道
func (w *Watcher) Events() <-chan *danmaku.DanmakuEvent {
	return w.events
}

// State 返回当前连接状态
func (w *Watcher) State() ConnState {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.state
}

// RoomID 返回直播间 ID
func (w *Watcher) RoomID() int64 {
	return w.roomID
}

// Start 启动监听（阻塞直到停止）
func (w *Watcher) Start() error {
	for {
		if err := w.connect(); err != nil {
			w.setState(StateReconnecting)
			log.Printf("连接失败: %v，%v 后重试...", err, reconnectDelay)
			select {
			case <-time.After(reconnectDelay):
				continue
			case <-w.done:
				return fmt.Errorf("已停止")
			}
		}

		w.readLoop()

		select {
		case <-w.done:
			return nil
		default:
			w.setState(StateReconnecting)
			log.Printf("连接已断开，%v 后重试...", reconnectDelay)
			time.Sleep(reconnectDelay)
		}
	}
}

// connect 建立连接并认证
func (w *Watcher) connect() error {
	w.setState(StateConnecting)

	// 1. 调用 /v2/app/start 获取连接信息
	startData, err := w.client.Start()
	if err != nil {
		return fmt.Errorf("启动项目失败: %w", err)
	}

	w.roomID = startData.AnchorInfo.RoomID
	w.gameID = startData.GameInfo.GameID
	w.authBody = startData.WebsocketInfo.AuthBody
	w.wssLinks = startData.WebsocketInfo.WssLink

	log.Printf("开放平台启动成功: room=%d game_id=%s wss_link=%v",
		w.roomID, w.gameID, w.wssLinks)

	if len(w.wssLinks) == 0 {
		return fmt.Errorf("未获取到 WebSocket 地址")
	}

	// 2. 建立 WebSocket 连接
	var conn *websocket.Conn
	dialer := &websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	wsHeaders := http.Header{
		"User-Agent": []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"},
		"Origin":     []string{"https://live.bilibili.com"},
	}

	for i, link := range w.wssLinks {
		conn, _, err = dialer.Dial(link, wsHeaders)
		if err == nil {
			break
		}
		if i < len(w.wssLinks)-1 {
			log.Printf("WebSocket 连接 %s 失败: %v，尝试下一个...", link, err)
		}
	}
	if conn == nil {
		return fmt.Errorf("WebSocket 连接失败，已尝试所有地址")
	}
	w.conn = conn

	// 3. 发送认证包（auth_body 就是预生成的认证 JSON）
	authPacket := danmaku.NewPacket(danmaku.OpAuth, danmaku.ProtoJSON, []byte(w.authBody))
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

	w.setState(StateConnected)
	log.Printf("已连接到直播间 %d（开放平台）", w.roomID)

	// 通知 TUI 连接成功
	if w.OnConnected != nil {
		w.OnConnected(w.roomID)
	}

	// 5. 启动心跳
	go w.heartbeatLoop()
	go w.restHeartbeatLoop()

	return nil
}

// readLoop 持续读取消息
func (w *Watcher) readLoop() {
	defer func() {
		w.mu.Lock()
		if w.conn != nil {
			w.conn.Close()
			w.conn = nil
		}
		w.mu.Unlock()
	}()

	for {
		select {
		case <-w.done:
			return
		default:
		}

		w.mu.Lock()
		conn := w.conn
		w.mu.Unlock()

		if conn == nil {
			return
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("读取消息失败: %v", err)
			return
		}

		w.handleMessage(msg)
	}
}

// handleMessage 处理收到的消息
func (w *Watcher) handleMessage(msg []byte) {
	pkt, err := danmaku.ReadPacket(msg)
	if err != nil {
		log.Printf("解析包失败: %v", err)
		return
	}

	payload, err := pkt.DecompressPayload()
	if err != nil {
		log.Printf("解压失败: %v", err)
		return
	}

	switch pkt.Header.Operation {
	case danmaku.OpHeartbeatReply:
		// 心跳回复，忽略

	case danmaku.OpCommand:
		log.Printf("收到 OpCommand: %d 字节", len(payload))
		w.parseCommandPackets(payload)

	default:
		log.Printf("收到未处理操作码: op=%d, len=%d", pkt.Header.Operation, len(payload))
	}
}

// parseCommandPackets 解析命令包（可能包含多个子包）
func (w *Watcher) parseCommandPackets(data []byte) {
	// 1) 先尝试整个作为 JSON 解析
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		w.parseCmdJSON(raw)
		return
	}

	// 2) 失败则按多包序列解析
	offset := 0
	parsedCount := 0
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
			parsedCount++
			var rawCmd json.RawMessage
			if json.Unmarshal(payload, &rawCmd) == nil {
				w.parseCmdJSON(rawCmd)
			}
		}
		offset += int(pkt.Header.TotalSize)
	}
	if parsedCount == 0 {
		log.Printf("多包解析未找到 OpCommand，数据前64字节: %x", data[:min(len(data), 64)])
	}
}

// parseCmdJSON 解析单个命令 JSON
func (w *Watcher) parseCmdJSON(raw json.RawMessage) {
	var cmd CmdMessage
	if err := json.Unmarshal(raw, &cmd); err != nil {
		preview := string(raw)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		log.Printf("命令 JSON 解析失败: %v, data=%s", err, preview)
		return
	}

	switch cmd.Cmd {
	case "LIVE_OPEN_PLATFORM_DM":
		w.handleDanmaku(cmd.Data)
	case "":
		// 忽略空命令
	default:
		log.Printf("收到其他命令: %s", cmd.Cmd)
	}
}

// handleDanmaku 处理弹幕消息
func (w *Watcher) handleDanmaku(data json.RawMessage) {
	var dm DanmakuData
	if err := json.Unmarshal(data, &dm); err != nil {
		log.Printf("解析弹幕数据失败: %v", err)
		return
	}

	event := &danmaku.DanmakuEvent{
		RoomID:    dm.RoomID,
		UserID:    dm.UID,
		Username:  dm.UserName,
		Content:   dm.Content,
		Timestamp: time.Unix(dm.Timestamp, 0),
	}

	if w.roomID != 0 {
		event.RoomID = w.roomID
	}

	select {
	case w.events <- event:
	default:
	}
}

// heartbeatLoop WebSocket 心跳（每 30s）
func (w *Watcher) heartbeatLoop() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			w.mu.Lock()
			conn := w.conn
			w.mu.Unlock()

			if conn == nil {
				return
			}

			heartbeat := danmaku.NewHeartbeatPacket()
			conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := conn.WriteMessage(websocket.BinaryMessage, heartbeat); err != nil {
				log.Printf("发送 WebSocket 心跳失败: %v", err)
				return
			}
		}
	}
}

// restHeartbeatLoop REST API 项目心跳（每 30s）
func (w *Watcher) restHeartbeatLoop() {
	ticker := time.NewTicker(restHeartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			if err := w.client.Heartbeat(w.gameID); err != nil {
				log.Printf("发送项目心跳失败: %v", err)
			}
		}
	}
}

// Stop 停止监听
func (w *Watcher) Stop() {
	w.stopOnce.Do(func() {
		close(w.done)
		w.mu.Lock()
		if w.conn != nil {
			w.conn.Close()
			w.conn = nil
		}
		w.mu.Unlock()

		// 关闭项目
		if w.gameID != "" {
			if err := w.client.End(w.gameID); err != nil {
				log.Printf("关闭项目失败: %v", err)
			} else {
				log.Printf("已关闭项目 game_id=%s", w.gameID)
			}
		}
	})
}

func (w *Watcher) setState(s ConnState) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.state = s
}
