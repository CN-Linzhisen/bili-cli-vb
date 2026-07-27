package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/CN-Linzhisen/bili-cli-vb/internal/bilibili"
	"github.com/CN-Linzhisen/bili-cli-vb/internal/danmaku"
	"github.com/CN-Linzhisen/bili-cli-vb/internal/filter"
	"github.com/CN-Linzhisen/bili-cli-vb/internal/openlive"
	"github.com/CN-Linzhisen/bili-cli-vb/internal/session"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Update 实现 tea.Model.Update
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-3)
			m.viewport.YPosition = 0
			m.ready = true

			// 窗口就绪后检查是否自动连接（开放平台模式）
			if m.cfg.OpenLive != nil && m.state == stateConnecting {
				return m, m.startOpenLiveConnect()
			}
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 3
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case roomEnteredMsg:
		return m.handleRoomEntered(msg)

	case *danmaku.DanmakuEvent:
		return m.handleDanmakuEvent(msg)

	case connStatusMsg:
		m.connStatus = msg.status
		if m.state == stateConnecting {
			m.state = stateConnected
			m.connStatus = msg.status
		}
		return m, nil

	case connErrorMsg:
		m.state = stateError
		m.err = msg.err
		m.connStatus = fmt.Sprintf("错误: %v", msg.err)
		return m, nil

	default:
		return m, nil
	}
}

// startOpenLiveConnect 启动开放平台连接
func (m *Model) startOpenLiveConnect() tea.Cmd {
	olCfg := m.cfg.OpenLive
	client := openlive.NewClient(nil, olCfg.AppID, olCfg.AccessKeyID, olCfg.AccessKeySecret, olCfg.Code)

	w := openlive.NewWatcher(client)
	w.OnConnected = func(roomID int64) {
		if m.program != nil {
			m.program.Send(connStatusMsg{status: fmt.Sprintf("已连接到直播间 %d（开放平台）", roomID)})
		}
	}
	m.watcher = w

	// 启动弹幕读取循环
	go m.readDanmakuLoop()

	// 在后台 goroutine 中连接（不阻塞 tea.Cmd）
	go func() {
		if err := m.watcher.Start(); err != nil {
			if m.program != nil {
				m.program.Send(connErrorMsg{err: err})
			}
			return
		}
	}()

	return nil
}

// handleKeyMsg 处理键盘输入
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateInput:
		switch msg.Type {
		case tea.KeyEnter:
			roomID, err := strconv.ParseInt(strings.TrimSpace(m.roomIDInput), 10, 64)
			if err != nil || roomID <= 0 {
				return m, nil
			}
			return m, func() tea.Msg {
				return roomEnteredMsg{roomID: roomID}
			}

		case tea.KeyBackspace:
			if len(m.roomIDInput) > 0 {
				m.roomIDInput = m.roomIDInput[:len(m.roomIDInput)-1]
			}
			return m, nil

		case tea.KeyCtrlC, tea.KeyEscape:
			return m, tea.Quit
		case tea.KeyRunes:
			if len(msg.Runes) == 1 && (msg.Runes[0] == 'q' || msg.Runes[0] == 'Q') {
				return m, tea.Quit
			}
			m.roomIDInput += string(msg.Runes)
			return m, nil
		default:
			if msg.Type == tea.KeySpace {
				m.roomIDInput += " "
			}
			return m, nil
		}

	case stateConnected, stateError:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEscape:
			return m, tea.Quit
		case tea.KeyRunes:
			if len(msg.Runes) == 1 && (msg.Runes[0] == 'q' || msg.Runes[0] == 'Q') {
				return m, tea.Quit
			}
			return m, nil
		default:
			return m, nil
		}

	default:
		return m, nil
	}
}

// handleRoomEntered 处理房间号输入完成（旧版 API 模式）
func (m *Model) handleRoomEntered(msg roomEnteredMsg) (tea.Model, tea.Cmd) {
	m.roomID = msg.roomID
	m.state = stateConnecting
	m.connStatus = fmt.Sprintf("正在连接到直播间 %d...", m.roomID)

	// 初始化日志文件
	if m.jsonlLogger != nil {
		m.jsonlLogger.SetRoom(m.roomID)
	}

	// 确保 buvid3 存在
	buvid3 := m.session.Buvid3
	if buvid3 == "" {
		buvid3 = session.GenerateBuvid3()
	}

	// 启动旧版弹幕监听
	m.watcher = bilibili.NewRoomWatcher(m.roomID, m.session.Sessdata, buvid3, m.session.DedeUserID)

	// 启动弹幕读取循环
	go m.readDanmakuLoop()

	// 启动监听器
	go func() {
		if err := m.watcher.Start(); err != nil {
			if m.program != nil {
				m.program.Send(connErrorMsg{err: err})
			}
			return
		}
		if m.program != nil {
			m.program.Send(connStatusMsg{status: fmt.Sprintf("已连接到直播间 %d", m.roomID)})
		}
	}()

	return m, nil
}

// readDanmakuLoop 从 watcher 读取弹幕并发送到 TUI
func (m *Model) readDanmakuLoop() {
	if m.watcher == nil {
		return
	}
	for event := range m.watcher.Events() {
		if m.program != nil {
			m.program.Send(event)
		} else {
			m.handleDanmakuEvent(event)
		}
	}
}

// handleDanmakuEvent 处理收到的弹幕事件
func (m *Model) handleDanmakuEvent(event *danmaku.DanmakuEvent) (tea.Model, tea.Cmd) {
	// 更新房间号
	if m.roomID == 0 && event.RoomID != 0 {
		m.roomID = event.RoomID
		if m.jsonlLogger != nil {
			m.jsonlLogger.SetRoom(m.roomID)
		}
	}

	// 关键词检测
	matchResult := filter.Match(event.Content, m.keywords)

	line := DanmakuLine{
		Content:   event.Content,
		Username:  event.Username,
		Time:      event.Timestamp.Format("15:04:05"),
		Highlight: matchResult.Matched,
	}

	if matchResult.Matched {
		line.Color = filter.HighlightColor(matchResult.ColorIndex)
	}

	m.danmakuList = append(m.danmakuList, line)
	m.danmakuCount++

	// 写日志
	if m.jsonlLogger != nil {
		m.jsonlLogger.Log(event)
	}

	// 更新 viewport 内容
	if m.ready {
		m.viewport.SetContent(m.renderDanmakuList())
		m.viewport.GotoBottom()
	}

	return m, nil
}
