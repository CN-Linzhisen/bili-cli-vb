// Package filter 提供关键词匹配功能
package filter

import "strings"

// MatchResult 表示匹配结果
type MatchResult struct {
	Matched bool     // 是否有匹配
	Keywords []string // 命中的关键词列表
	ColorIndex int   // 命中关键词的颜色索引（用于 TUI 高亮）
}

// Color palette for keyword highlighting
var highlightColors = []string{
	"#FF6B6B", // 红色
	"#4ECDC4", // 青色
	"#FFE66D", // 黄色
	"#A8E6CF", // 绿色
	"#FF8A5C", // 橙色
	"#6C5CE7", // 紫色
	"#A29BFE", // 淡紫
	"#FD79A8", // 粉色
}

// Match 对弹幕文本执行关键词匹配
// text: 弹幕文本内容
// keywords: 要匹配的关键词列表
// 返回匹配结果
func Match(text string, keywords []string) *MatchResult {
	if len(keywords) == 0 || text == "" {
		return &MatchResult{Matched: false}
	}

	var matched []string
	var colorIdx int

	for i, kw := range keywords {
		if kw == "" {
			continue
		}
		if strings.Contains(text, kw) {
			matched = append(matched, kw)
			if len(matched) == 1 {
				colorIdx = i % len(highlightColors)
			}
		}
	}

	if len(matched) == 0 {
		return &MatchResult{Matched: false}
	}

	return &MatchResult{
		Matched:    true,
		Keywords:   matched,
		ColorIndex: colorIdx,
	}
}

// HighlightColor 返回指定颜色索引的 ANSI 颜色码
func HighlightColor(index int) string {
	return highlightColors[index%len(highlightColors)]
}
