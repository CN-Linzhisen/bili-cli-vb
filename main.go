package main

import (
	"fmt"
	"os"

	"github.com/CN-Linzhisen/bili-cli-vb/internal/config"
	"github.com/CN-Linzhisen/bili-cli-vb/internal/logger"
	"github.com/CN-Linzhisen/bili-cli-vb/internal/login"
	"github.com/CN-Linzhisen/bili-cli-vb/internal/session"
	"github.com/CN-Linzhisen/bili-cli-vb/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 检查并获取登录凭证（开放平台不需要 B站 Cookie 登录）
	var sess *session.Session
	if cfg.OpenLive == nil {
		sess = ensureLoggedIn()
		if sess == nil {
			os.Exit(1)
		}
	}

	// 初始化 JSONL 日志
	jsonlLogger := logger.NewLogger(cfg.LogDir)
	if cfg.LogRetentionDays > 0 {
		jsonlLogger.SetRetentionDays(cfg.LogRetentionDays)
	}

	// 启动 TUI
	model := tui.NewModel(sess, cfg, jsonlLogger)
	p := tea.NewProgram(model, tea.WithAltScreen())
	model.SetProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI 运行失败: %v\n", err)
		os.Exit(1)
	}

	// 清理资源
	model.Stop()
}

func ensureLoggedIn() *session.Session {
	sess, err := session.Load()
	if err != nil || session.IsExpired(sess) {
		if err != nil {
			fmt.Println("未找到登录凭证，开始扫码登录...")
		} else {
			fmt.Println("登录凭证已过期，需要重新扫码登录")
		}

		sess, err = login.Login()
		if err != nil {
			fmt.Fprintf(os.Stderr, "登录失败: %v\n", err)
			return nil
		}

		if err := sess.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "保存凭证失败: %v\n", err)
			return nil
		}
		fmt.Println("凭证已保存到 session.json")
	} else {
		fmt.Println("登录凭证有效，跳过扫码")
	}
	return sess
}

