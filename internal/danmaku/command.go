package danmaku

import (
	"fmt"
	"strconv"
	"time"
)

// ParseDanmakuEvent 从 DANMU_MSG 命令中解析弹幕事件
// DANMU_MSG 的 info 字段结构：
//
//	info[0] = [type, fontsize, mode, color, timestamp, ...]
//	info[1] = 弹幕文本
//	info[2] = [用户昵称, ...]
//	info[3] = [用户ID, ...]
//
// 不同协议版本略有差异，这里兼容常见格式。
func ParseDanmakuEvent(raw map[string]interface{}, roomID int64) (*DanmakuEvent, error) {
	cmd, ok := raw["cmd"].(string)
	if !ok || cmd != "DANMU_MSG" {
		return nil, fmt.Errorf("不是 DANMU_MSG 命令: %s", cmd)
	}

	info, ok := raw["info"].([]interface{})
	if !ok || len(info) < 3 {
		return nil, fmt.Errorf("DANMU_MSG info 字段格式异常")
	}

	// 弹幕内容 — info[1]
	content, ok := info[1].(string)
	if !ok {
		return nil, fmt.Errorf("无法解析弹幕内容")
	}

	// 用户昵称 — info[2][0]
	username := "未知用户"
	if userInfo, ok := info[2].([]interface{}); ok && len(userInfo) > 0 {
		if name, ok := userInfo[0].(string); ok {
			username = name
		}
	}

	// 用户 ID — info[3][0]
	var userID int64
	if userData, ok := info[3].([]interface{}); ok && len(userData) > 0 {
		switch v := userData[0].(type) {
		case float64:
			userID = int64(v)
		case string:
			userID, _ = strconv.ParseInt(v, 10, 64)
		}
	}

	// 弹幕时间戳
	var ts time.Time
	if meta, ok := info[0].([]interface{}); ok && len(meta) > 4 {
		switch v := meta[4].(type) {
		case float64:
			ts = time.Unix(int64(v), 0)
		}
	}
	if ts.IsZero() {
		ts = time.Now()
	}

	return &DanmakuEvent{
		RoomID:    roomID,
		UserID:    userID,
		Username:  username,
		Content:   content,
		Timestamp: ts,
	}, nil
}

// IsDanmakuCmd 检查命令是否为弹幕命令
func IsDanmakuCmd(raw map[string]interface{}) bool {
	cmd, ok := raw["cmd"].(string)
	return ok && cmd == "DANMU_MSG"
}
