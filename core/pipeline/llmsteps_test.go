package pipeline

import (
	"strings"
	"testing"
)

func TestSlideRunes(t *testing.T) {
	// 10 字文本,窗口 6 重叠 2 → 步长 4: [0:6][4:10]
	s := "0123456789"
	got := slideRunes(s, 6, 2)
	if len(got) != 2 || got[0] != "012345" || got[1] != "456789" {
		t.Fatalf("got %v", got)
	}

	// 短于窗口: 单块
	got = slideRunes("abc", 10, 2)
	if len(got) != 1 || got[0] != "abc" {
		t.Fatalf("short got %v", got)
	}

	// 空串
	if got := slideRunes("", 10, 2); got != nil {
		t.Fatalf("empty got %v", got)
	}

	// 中文按 rune 切(不切坏多字节): 25 字,窗口 10 重叠 2 → 步长 8 → [0:10][8:18][16:25] 3 块
	s = strings.Repeat("中", 25)
	got = slideRunes(s, 10, 2)
	if len(got) != 3 || []rune(got[0])[0] != '中' || len([]rune(got[0])) != 10 {
		t.Fatalf("cjk got %d blocks first=%d runes", len(got), len([]rune(got[0])))
	}
}
