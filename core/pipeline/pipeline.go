// Package pipeline 是文章标注管线：采集后的 Item(pending) 经处理变为 processed。
// 第一版为规则式实现(去 HTML 截断摘要、词频关键词、来源归类)，
// 预留 LLM 接口位，后续可替换/叠加大模型实现。
package pipeline

import (
	"log"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/store"
)

// Enricher 单步标注器接口。LLM 摘要器等后续实现同一接口插入 Chain。
type Enricher interface {
	Name() string
	Enrich(it *model.Item) error
}

// Chain 按顺序执行多个 Enricher，全部成功后置为 processed。
type Chain struct {
	steps []Enricher
	store *store.Store
}

func NewChain(st *store.Store, steps ...Enricher) *Chain {
	return &Chain{steps: steps, store: st}
}

// claimTimeout processing 超时回收: 认领后超过此时长仍未出结果的视为处理端崩溃,重新认领。
const claimTimeout = 30 * time.Minute

// RunOnce 认领并处理一批 pending 文章，返回处理数。
// 认领即置 processing(带认领时间),超时未完成的会被下一轮回收重试。
func (c *Chain) RunOnce(limit int) (int, error) {
	items, err := c.store.ClaimPending(limit, claimTimeout)
	if err != nil {
		return 0, err
	}
	for i := range items {
		it := &items[i]
		status := c.enrich(it)
		if err := c.store.SetEnrichment(it.ID, it.Summary, it.Keywords, it.Source, status); err != nil {
			return i, err
		}
	}
	return len(items), nil
}

// RunOne 重跑单篇标注(标注失败重试用),返回最终状态。
// 调用方需先保证该篇处于 pending(如 RetryItem),否则会覆盖现有标注。
func (c *Chain) RunOne(id int64) (model.ItemStatus, error) {
	it, err := c.store.GetItem(id)
	if err != nil {
		return "", err
	}
	status := c.enrich(it)
	if err := c.store.SetEnrichment(it.ID, it.Summary, it.Keywords, it.Source, status); err != nil {
		return "", err
	}
	return status, nil
}

// enrich 顺序执行标注步骤,全部成功为 processed,任一失败为 failed(已执行的步骤结果保留)。
func (c *Chain) enrich(it *model.Item) model.ItemStatus {
	for _, step := range c.steps {
		if err := step.Enrich(it); err != nil {
			log.Printf("pipeline %s item %d: %v", step.Name(), it.ID, err)
			return model.StatusFailed
		}
	}
	return model.StatusProcessed
}

// --- 内置规则式 Enricher ---

var htmlTag = regexp.MustCompile(`<[^>]*>`)
var scriptBlock = regexp.MustCompile(`(?is)<(script|style|noscript)[^>]*>.*?</(script|style|noscript)>`)
var spaces = regexp.MustCompile(`\s+`)

// stripHTML 去掉 script/style 整块再剥标签(JSON-LD 等脚本内容不能进摘要)。
func stripHTML(s string) string {
	return htmlTag.ReplaceAllString(scriptBlock.ReplaceAllString(s, " "), " ")
}

// Summary 摘要器：剥掉 HTML，截取前 N 个字符。
type Summary struct{ MaxRunes int }

func (s Summary) Name() string { return "summary" }

func (s Summary) Enrich(it *model.Item) error {
	text := strings.TrimSpace(spaces.ReplaceAllString(stripHTML(it.Content), " "))
	max := s.MaxRunes
	if max <= 0 {
		max = 200
	}
	if utf8.RuneCountInString(text) > max {
		r := []rune(text)
		text = string(r[:max]) + "…"
	}
	it.Summary = text
	return nil
}

// Source 来源归类：优先用已映射规则，否则取链接域名。
type Source struct{ Rules map[string]string }

func (s Source) Name() string { return "source" }

func (s Source) Enrich(it *model.Item) error {
	host := ""
	if u, err := url.Parse(it.Link); err == nil {
		host = u.Hostname()
	}
	for k, v := range s.Rules {
		if strings.Contains(host, k) {
			it.Source = v
			return nil
		}
	}
	it.Source = host
	return nil
}

// Keywords 关键词提取：英文按词频(去停用词)，CJK 取高频二字组。规则式占位实现。
type Keywords struct{ TopN int }

func (k Keywords) Name() string { return "keywords" }

func (k Keywords) Enrich(it *model.Item) error {
	text := it.Title + " " + stripHTML(it.Content)
	top := k.TopN
	if top <= 0 {
		top = 5
	}
	freq := map[string]int{}
	var word strings.Builder
	flush := func() {
		w := strings.ToLower(word.String())
		word.Reset()
		if len(w) >= 3 && !stopwords[w] {
			freq[w]++
		}
	}
	var prevCJK rune
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			if prevCJK != 0 {
				freq[string([]rune{prevCJK, r})]++
			}
			prevCJK = r
			flush()
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			prevCJK = 0
			word.WriteRune(r)
		default:
			prevCJK = 0
			flush()
		}
	}
	flush()
	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(freq))
	for k2, v := range freq {
		pairs = append(pairs, kv{k2, v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].v > pairs[j].v })
	var out []string
	for _, p := range pairs {
		if len(out) >= top {
			break
		}
		out = append(out, p.k)
	}
	it.Keywords = strings.Join(out, ",")
	return nil
}

var stopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "that": true,
	"this": true, "from": true, "are": true, "was": true, "were": true,
	"you": true, "your": true, "have": true, "has": true, "not": true,
	"but": true, "they": true, "their": true, "will": true, "would": true,
}
