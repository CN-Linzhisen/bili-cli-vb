// Package config 管理应用配置 (config.json)
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const configFile = "config.json"

// OpenLiveConfig B站开放平台配置（用于弹幕监控）
type OpenLiveConfig struct {
	AppID          int64  `json:"app_id"`
	AccessKeyID    string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	Code           string `json:"code"`
}

// Config 是应用程序的用户配置
type Config struct {
	Keywords         []string        `json:"keywords"`           // 监控关键词列表
	DanmakuLog       bool            `json:"danmaku_log"`        // 是否保存弹幕日志
	LogDir           string          `json:"log_dir"`            // 日志目录
	LogRetentionDays int             `json:"log_retention_days"` // 日志保留天数（默认 7）
	OpenLive         *OpenLiveConfig `json:"open_live,omitempty"` // 开放平台配置（可选）
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Keywords:         []string{"抽奖", "红包", "感谢", "主播"},
		DanmakuLog:       true,
		LogDir:           "./logs",
		LogRetentionDays: 7,
	}
}

// Load 从当前目录加载 config.json，不存在时生成默认配置
func Load() (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，创建默认配置
			if err := cfg.Save(); err != nil {
				return nil, fmt.Errorf("创建默认配置失败: %w", err)
			}
			fmt.Printf("已创建默认配置文件: %s\n", configFile)
			return cfg, nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 确保日志目录存在
	if cfg.DanmakuLog && cfg.LogDir != "" {
		absDir, err := filepath.Abs(cfg.LogDir)
		if err == nil {
			if err := os.MkdirAll(absDir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "警告: 创建日志目录失败: %v\n", err)
			}
		}
	}

	return cfg, nil
}

// Save 将配置保存到文件
func (c *Config) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	return nil
}
