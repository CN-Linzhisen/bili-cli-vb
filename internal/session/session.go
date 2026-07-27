// Package session 管理 B站登录凭证 (session.json)
package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const sessionFile = "session.json"

// Session 存储 B站登录凭证
type Session struct {
	Sessdata     string `json:"sessdata"`
	BiliJCT     string `json:"bili_jct"`
	DedeUserID  string `json:"dede_user_id"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Buvid3      string `json:"buvid3"`
	CreatedAt   int64  `json:"created_at"`   // 创建时间戳
}

// IsValid 检查凭证是否存在且可能有效（基本存在性检查）
func IsValid(s *Session) bool {
	if s == nil {
		return false
	}
	return s.Sessdata != "" && s.Buvid3 != ""
}

// IsExpired 检查凭证是否已过期（B站 SESSDATA 有效期通常约 30 天）
func IsExpired(s *Session) bool {
	if s == nil || s.CreatedAt == 0 {
		return true
	}
	// SESSDATA 有效期约 30 天，保守按 25 天检查
	const maxAge = 25 * 24 * time.Hour
	return time.Unix(s.CreatedAt, 0).Add(maxAge).Before(time.Now())
}

// Load 从当前目录加载 session.json
func Load() (*Session, error) {
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("凭证文件 %s 不存在", sessionFile)
		}
		return nil, fmt.Errorf("读取凭证文件失败: %w", err)
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("解析凭证文件失败: %w", err)
	}
	return &s, nil
}

// GenerateBuvid3 生成一个随机的 buvid3 标识符
func GenerateBuvid3() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 极端情况：随机数生成失败，使用时间戳作为回退
		return fmt.Sprintf("XY%xinfoc", time.Now().UnixNano())
	}
	return "XY" + hex.EncodeToString(b) + "infoc"
}

// Save 将凭证保存到 session.json
func (s *Session) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化凭证失败: %w", err)
	}
	if err := os.WriteFile(sessionFile, data, 0644); err != nil {
		return fmt.Errorf("写入凭证文件失败: %w", err)
	}
	return nil
}
