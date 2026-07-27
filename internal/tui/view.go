package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// 样式定义
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00BFFF")).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#333333")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1)

	highlightStyle = lipgloss.NewStyle().
			Bold(true)

	inputPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00BFFF")).
				Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF4444")).
			Bold(true)

	timeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700"))
)

const maxUsernameWidth = 20 // 最大用户名显示宽度（终端列数）

// View 实现 tea.Model.View
func (m *Model) View() string {
	switch m.state {
	case stateInput:
		return m.inputView()
	case stateConnecting:
		return m.connectingView()
	case stateConnected:
		return m.connectedView()
	case stateError:
		return m.errorView()
	default:
		return "未知状态"
	}
}

// inputView 显示房间号输入界面
func (m *Model) inputView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🎬 BiliBili 直播弹幕监控"))
	b.WriteString("\n\n")
	b.WriteString(inputPromptStyle.Render("请输入直播间房间号:"))
	b.WriteString("\n\n")
	b.WriteString("  " + m.roomIDInput)
	b.WriteString("█\n\n")
	b.WriteString(timeStyle.Render("  Ctrl+C/Esc 退出"))

	// 居中显示
	if m.height > 0 && m.width > 0 {
		content := b.String()
		lines := strings.Split(content, "\n")
		contentHeight := len(lines)
		topPadding := (m.height - contentHeight) / 2
		if topPadding > 0 {
			return strings.Repeat("\n", topPadding) + content
		}
	}

	return b.String()
}

// connectingView 显示连接中界面
func (m *Model) connectingView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("🎬 BiliBili 直播弹幕监控"))
	b.WriteString("\n\n")
	b.WriteString(m.connStatus)
	b.WriteString("\n\n请稍候...")
	return b.String()
}

// connectedView 显示弹幕流界面
func (m *Model) connectedView() string {
	if !m.ready {
		return "初始化中..."
	}

	statusBar := m.renderStatusBar()

	content := m.viewport.View()
	if content == "" && len(m.danmakuList) > 0 {
		content = m.renderDanmakuList()
	}

	return content + "\n" + statusBar
}

// formatUsername 格式化用户名，处理 CJK 字符宽度
func formatUsername(name string) string {
	if name == "" {
		return strings.Repeat(" ", maxUsernameWidth)
	}

	// 先按 rune 截断（最多允许 15 个字符，防止过长）
	runes := []rune(name)
	if len(runes) > 15 {
		runes = runes[:15]
	}
	name = string(runes)

	// 计算实际显示宽度，用空格补齐
	displayWidth := lipgloss.Width(name)
	if displayWidth > maxUsernameWidth {
		// 太长了，从尾部截断
		for len(runes) > 0 && lipgloss.Width(string(runes)) > maxUsernameWidth-1 {
			runes = runes[:len(runes)-1]
		}
		name = string(runes) + "…"
		displayWidth = lipgloss.Width(name)
	}

	if displayWidth < maxUsernameWidth {
		name += strings.Repeat(" ", maxUsernameWidth-displayWidth)
	}

	return name
}

// renderDanmakuList 渲染弹幕列表
func (m *Model) renderDanmakuList() string {
	if len(m.danmakuList) == 0 {
		return "等待弹幕..."
	}

	start := 0
	const maxDisplay = 500
	if len(m.danmakuList) > maxDisplay {
		start = len(m.danmakuList) - maxDisplay
	}

	var b strings.Builder
	for i := start; i < len(m.danmakuList); i++ {
		line := m.danmakuList[i]
		if i > start {
			b.WriteByte('\n')
		}

		// 时间
		b.WriteString(timeStyle.Render(line.Time))
		b.WriteString(" ")

		// 用户名（自适应 CJK 宽度）
		b.WriteString(userStyle.Render(formatUsername(line.Username)))
		b.WriteString(" ")

		// 弹幕内容
		if line.Highlight {
			style := highlightStyle.Foreground(lipgloss.Color(line.Color))
			b.WriteString(style.Render(line.Content))
		} else {
			b.WriteString(line.Content)
		}
	}

	return b.String()
}

// renderStatusBar 渲染底部状态栏
func (m *Model) renderStatusBar() string {
	roomInfo := fmt.Sprintf(" 房间: %d ", m.roomID)
	statusInfo := fmt.Sprintf(" 状态: %s ", m.connStatus)
	countInfo := fmt.Sprintf(" 弹幕数: %d ", m.danmakuCount)
	quitInfo := " [q]退出 "

	content := roomInfo + statusInfo + countInfo
	padding := m.width - lipgloss.Width(content) - lipgloss.Width(quitInfo)
	if padding < 1 {
		padding = 1
	}

	return statusBarStyle.Render(content + strings.Repeat(" ", padding) + quitInfo)
}

// errorView 显示错误界面
func (m *Model) errorView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("🎬 BiliBili 直播弹幕监控"))
	b.WriteString("\n\n")
	b.WriteString(errorStyle.Render(fmt.Sprintf("错误: %v", m.err)))
	b.WriteString("\n\n")
	b.WriteString("按 Ctrl+C 或 q 退出")
	return b.String()
}
