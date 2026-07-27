// Package logger 提供弹幕 JSONL 日志写入能力，支持按天轮转和自动清理
package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/CN-Linzhisen/bili-cli-vb/internal/danmaku"
)

// Logger 将弹幕事件写入 JSONL 文件
type Logger struct {
	file   *os.File
	writer *json.Encoder
	mu     sync.Mutex
	closed bool
	logDir string
	roomID int64

	retentionDays int // 保留天数，超过此期限的日志文件将被清理
}

// NewLogger 创建新的 JSONL 日志记录器
// logDir: 日志目录（为空则不写入文件）
func NewLogger(logDir string) *Logger {
	if logDir == "" {
		return &Logger{closed: true}
	}
	return &Logger{
		closed:        false,
		logDir:        logDir,
		retentionDays: 7,
	}
}

// SetRetentionDays 设置日志保留天数
func (l *Logger) SetRetentionDays(days int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if days > 0 {
		l.retentionDays = days
	}
}

// SetRoom 设置房间号并打开对应的日志文件（按天轮转）
func (l *Logger) SetRoom(roomID int64) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed || l.logDir == "" {
		return nil
	}

	l.roomID = roomID

	// 确保日志目录存在
	if err := os.MkdirAll(l.logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 关闭之前的文件
	if l.file != nil {
		l.file.Close()
		l.file = nil
		l.writer = nil
	}

	// 清理过期日志
	l.cleanupLocked()

	// 打开今天的日志文件（每天一个文件）
	filename := fmt.Sprintf("danmaku_%d_%s.jsonl", roomID, time.Now().Format("2006-01-02"))
	logPath := filepath.Join(l.logDir, filename)

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}

	l.file = f
	l.writer = json.NewEncoder(f)
	return nil
}

// Log 写入一条弹幕事件到日志文件
func (l *Logger) Log(event *danmaku.DanmakuEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed || l.writer == nil {
		return nil
	}

	entry := map[string]interface{}{
		"room_id":  event.RoomID,
		"user_id":  event.UserID,
		"username": event.Username,
		"content":  event.Content,
		"time":     event.Timestamp.Format("2006-01-02T15:04:05-07:00"),
	}

	return l.writer.Encode(entry)
}

// cleanupLocked 清理超过保留天数的日志文件（调用者需持有锁）
func (l *Logger) cleanupLocked() {
	if l.logDir == "" || l.retentionDays <= 0 {
		return
	}

	entries, err := os.ReadDir(l.logDir)
	if err != nil {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -l.retentionDays)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), "danmaku_") || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(l.logDir, entry.Name())
			os.Remove(path)
		}
	}
}

// CleanupNow 主动清理过期日志
func (l *Logger) CleanupNow() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cleanupLocked()
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

// ListLogFiles 列出日志目录下的所有日志文件
func (l *Logger) ListLogFiles() []string {
	if l.logDir == "" {
		return nil
	}

	entries, err := os.ReadDir(l.logDir)
	if err != nil {
		return nil
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			files = append(files, fmt.Sprintf("%s (%s, %.1f KB)",
				entry.Name(),
				info.ModTime().Format("2006-01-02"),
				float64(info.Size())/1024))
		}
	}

	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	return files
}
