// LLM 增强标注步骤: 滑动窗口摘要 + 总摘要。
package pipeline

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/juncaifeng/digestly/core/llm"
	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/store"
)

// LLMSummary 长文滑动窗口分块 -> 逐块摘要 -> 归并总摘要。
// 未配置默认 chat 模型时跳过(规则摘要兜底),避免无 LLM 环境报错。
type LLMSummary struct {
	st       *store.Store
	Window   int // 每块字数(默认 2000)
	Overlap  int // 块间重叠字数(默认 400)
	MaxChunk int // 最多处理块数(默认 8,控成本)
}

func NewLLMSummary(st *store.Store) *LLMSummary {
	return &LLMSummary{st: st, Window: 2000, Overlap: 400, MaxChunk: 8}
}

func (s *LLMSummary) Name() string { return "llm-summary" }

func (s *LLMSummary) Enrich(it *model.Item) error {
	cfg, err := s.st.PipelineConfigOfKind("chat")
	if err != nil {
		if err == sql.ErrNoRows {
			return nil // 无 chat 配置: 跳过,保留规则摘要
		}
		return err
	}
	text := strings.TrimSpace(stripHTML(it.Content))
	if text == "" {
		return nil
	}
	window, overlap, maxChunk := s.Window, s.Overlap, s.MaxChunk
	if window <= 0 {
		window = 2000
	}
	if overlap < 0 || overlap >= window {
		overlap = window / 5
	}
	if maxChunk <= 0 {
		maxChunk = 8
	}
	parts := slideRunes(text, window, overlap)
	if len(parts) > maxChunk {
		parts = parts[:maxChunk]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	chunks := make([]store.ItemChunk, 0, len(parts))
	summaries := make([]string, 0, len(parts))
	for i, p := range parts {
		sum, err := llm.Chat(ctx, cfg,
			"你是一名专业编辑。",
			fmt.Sprintf("用中文概括以下内容的核心要点(100-150字,直接给要点):\n\n%s", p))
		if err != nil {
			return fmt.Errorf("块 %d 摘要失败: %w", i, err)
		}
		sum = strings.TrimSpace(sum)
		chunks = append(chunks, store.ItemChunk{Text: p, Summary: sum})
		summaries = append(summaries, sum)
	}

	// 总摘要: 单块直接用;多块归并
	overall := summaries[0]
	if len(summaries) > 1 {
		var sb strings.Builder
		for i, sum := range summaries {
			fmt.Fprintf(&sb, "%d. %s\n", i+1, sum)
		}
		merged, err := llm.Chat(ctx, cfg,
			"你是一名专业编辑。",
			fmt.Sprintf("以下是同一文章不同段落的要点,合并为一段连贯的总摘要(200字内,直接输出):\n\n%s", sb.String()))
		if err != nil {
			return fmt.Errorf("总摘要归并失败: %w", err)
		}
		overall = strings.TrimSpace(merged)
	}
	if err := s.st.ReplaceItemChunks(it.ID, chunks); err != nil {
		return err
	}
	it.Summary = overall
	log.Printf("pipeline llm-summary item %d: %d chunks -> summary %d runes", it.ID, len(chunks), len([]rune(overall)))
	return nil
}

// slideRunes 滑动窗口切块(按 rune,window 步进 window-overlap)。
func slideRunes(s string, window, overlap int) []string {
	r := []rune(s)
	if len(r) == 0 {
		return nil
	}
	step := window - overlap
	var out []string
	for start := 0; start < len(r); start += step {
		end := start + window
		if end > len(r) {
			end = len(r)
		}
		out = append(out, string(r[start:end]))
		if end == len(r) {
			break
		}
	}
	return out
}
