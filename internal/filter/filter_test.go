package filter

import (
	"testing"
)

func TestMatch_ExactMatch(t *testing.T) {
	result := Match("感谢主播送的礼物", []string{"感谢"})
	if !result.Matched {
		t.Fatal("期望匹配，但未命中")
	}
	if len(result.Keywords) != 1 || result.Keywords[0] != "感谢" {
		t.Fatalf("期望命中关键词 '感谢'，实际: %v", result.Keywords)
	}
}

func TestMatch_PartialMatch(t *testing.T) {
	result := Match("今晚抽奖活动开始了", []string{"抽奖", "红包"})
	if !result.Matched {
		t.Fatal("期望匹配")
	}
	if len(result.Keywords) != 1 || result.Keywords[0] != "抽奖" {
		t.Fatalf("期望命中 '抽奖'，实际: %v", result.Keywords)
	}
}

func TestMatch_MultipleKeywords(t *testing.T) {
	result := Match("感谢主播抽奖红包", []string{"抽奖", "红包", "感谢"})
	if !result.Matched {
		t.Fatal("期望匹配")
	}
	if len(result.Keywords) != 3 {
		t.Fatalf("期望命中所有 3 个关键词，实际: %v", result.Keywords)
	}
}

func TestMatch_NoMatch(t *testing.T) {
	result := Match("普通的弹幕消息", []string{"抽奖", "红包"})
	if result.Matched {
		t.Fatal("期望不匹配，但命中了")
	}
}

func TestMatch_EmptyKeywords(t *testing.T) {
	result := Match("任何消息", nil)
	if result.Matched {
		t.Fatal("无关键词时应返回不匹配")
	}

	result = Match("任何消息", []string{})
	if result.Matched {
		t.Fatal("空关键词列表应返回不匹配")
	}
}

func TestMatch_EmptyText(t *testing.T) {
	result := Match("", []string{"抽奖"})
	if result.Matched {
		t.Fatal("空文本应返回不匹配")
	}
}

func TestMatch_ChineseAndEnglish(t *testing.T) {
	result := Match("感谢主播的SC", []string{"感谢", "SC"})
	if !result.Matched {
		t.Fatal("期望匹配中英文混合关键词")
	}
	if len(result.Keywords) != 2 {
		t.Fatalf("期望命中 2 个关键词，实际: %v", result.Keywords)
	}
}

func TestMatch_CaseSensitive(t *testing.T) {
	// B站弹幕保留大小写，strings.Contains 是大小写敏感的
	result := Match("Hello World", []string{"World"})
	if !result.Matched {
		t.Fatal("大小写完全匹配时应命中")
	}

	result = Match("Hello World", []string{"world"})
	if result.Matched {
		t.Fatal("大小写不匹配时应不命中")
	}
}

func TestMatch_KeywordInLongText(t *testing.T) {
	result := Match("这是一段很长的弹幕消息，包含了需要监控的关键词主播你好", []string{"主播"})
	if !result.Matched {
		t.Fatal("长文本中应能匹配关键词")
	}
}

func TestHighlightColor_Default(t *testing.T) {
	color := HighlightColor(0)
	if color == "" {
		t.Fatal("颜色不应为空")
	}
}

func TestHighlightColor_OutOfRange(t *testing.T) {
	// 超出调色板范围的索引应取模
	color := HighlightColor(100)
	if color == "" {
		t.Fatal("超出范围的索引也应返回有效颜色")
	}
}
