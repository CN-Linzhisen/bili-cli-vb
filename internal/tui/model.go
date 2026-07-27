// Package tui 提供基于 Bubble Tea 的终端弹幕展示界面
package tui

import (
	"github.com/CN-Linzhisen/bili-cli-vb/internal/config"
	"github.com/CN-Linzhisen/bili-cli-vb/internal/danmaku"
	"github.com/CN-Linzhisen/bili-cli-vb/internal/logger"
	"github.com/CN-Linzhisen/bili-cli-vb/internal/session"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// TUI state machine states
type tuiState int

const (
	stateInput      tuiState = iota // 等待输入房间号
	stateConnecting                  // 正在连接
	stateConnected                   // 已连接，弹幕流
	stateError                       // 出错
)

// DanmakuLine 表示 TUI 中显示的一条弹幕
type DanmakuLine struct {
	Content   string
	Username  string
	Time      string
	Highlight bool
	Color     string // 颜色码
}

// EventSource 是弹幕事件源的通用接口
type EventSource interface {
	Events() <-chan *danmaku.DanmakuEvent
	Start() error
	Stop()
	RoomID() int64
}

// Model 是 Bubble Tea 的主模型
type Model struct {
	state tuiState

	// 程序引用（用于 goroutine 通信）
	program *tea.Program

	// 房间号输入
	roomIDInput string
	roomID      int64
	err         error

	// 弹幕数据
	danmakuList  []DanmakuLine
	danmakuCount int

	// 关键词配置
	keywords []string

	// 弹幕事件源（开放平台 或 旧版 API）
	watcher EventSource

	// 日志
	jsonlLogger *logger.Logger

	// 视图
	viewport   viewport.Model
	ready      bool
	connStatus string

	width  int
	height int

	session  *session.Session
	cfg      *config.Config
}

// NewModel 创建新的 TUI 模型
func NewModel(sess *session.Session, cfg *config.Config, jsonlLogger *logger.Logger) *Model {
	m := &Model{
		state:       stateInput,
		keywords:    cfg.Keywords,
		connStatus:  "等待输入",
		session:     sess,
		cfg:         cfg,
		jsonlLogger: jsonlLogger,
	}

	// 如果配置了开放平台，自动进入连接状态
	if cfg.OpenLive != nil {
		m.state = stateConnecting
		m.connStatus = "连接中（开放平台）..."
	}

	return m
}

// Init 实现 tea.Model.Init
func (m *Model) Init() tea.Cmd {
	return tea.EnterAltScreen
}

// SetProgram 设置程序引用（在 NewProgram 后调用）
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

// 消息类型定义
type roomEnteredMsg struct {
	roomID int64
}

type connStatusMsg struct {
	status string
}

type connErrorMsg struct {
	err error
}

// Stop 停止 TUI 并清理资源
func (m *Model) Stop() {
	if m.watcher != nil {
		m.watcher.Stop()
	}
	if m.jsonlLogger != nil {
		m.jsonlLogger.Close()
	}
}
