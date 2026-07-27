// Package logger 提供弹幕 JSONL 日志写入能力
package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/CN-Linzhisen/bili-cli-vb/internal/danmaku"
)

// Logger 将弹幕事件写入 JSONL 文件
type Logger struct {
	file   *os.File
	writer *json.Encoder
	mu     sync.Mutex
	closed bool
	logDir string
}

// NewLogger 创建新的 JSONL 日志记录器
// logDir: 日志目录（为空则不写入文件）
func NewLogger(logDir string) *Logger {
	if logDir == "" {
		return &Logger{closed: true}
	}
	return &Logger{closed: false, logDir: logDir}
}

// SetRoom 设置房间号并打开对应的日志文件
func (l *Logger) SetRoom(roomID int64) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed || l.logDir == "" {
		return nil
	}

	// 关闭之前的文件
	if l.file != nil {
		l.file.Close()
	}

	if err := os.MkdirAll(l.logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	filename := fmt.Sprintf("danmaku_%d.jsonl", roomID)
	logPath := filepath.Join(l.logDir, filename)

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}

	l.file = f
	l.writer = json.NewEncoder(f)
	return nil
}

// Log 写入一条弹幕事件到日志
func (l *Logger) Log(event *danmaku.DanmakuEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed || l.writer == nil {
		return nil
	}

	// 使用一个可序列化的结构
	entry := map[string]interface{}{
		"room_id":  event.RoomID,
		"user_id":  event.UserID,
		"username": event.Username,
		"content":  event.Content,
		"time":     event.Timestamp.Format("2006-01-02T15:04:05-07:00"),
	}

	if err := l.writer.Encode(entry); err != nil {
		return fmt.Errorf("写入日志失败: %w", err)
	}

	return nil
}

// Close 关闭日志文件
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed || l.file == nil {
		return nil
	}
	l.closed = true
	return l.file.Close()
}
