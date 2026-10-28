package pipeline

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/juncaifeng/digestly/core/llm"
	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/store"
)

// LLMGraph 实体/关系抽取 + 向量化:
//   - LLM 结构化抽取 实体(name,type) 与 关系三元组(subject,predicate,object),落知识图谱;
//   - 配置了默认向量模型(kind=embedding)时,文章摘要与实体名向量化入库,供相似检索。
//
// 无 chat 配置时跳过;抽取 JSON 解析失败按步骤失败处理(可重试)。
type LLMGraph struct {
	st     *store.Store
	MaxRun int // 进抽取的正文上限(默认 3000 字)
}

func NewLLMGraph(st *store.Store) *LLMGraph {
	return &LLMGraph{st: st, MaxRun: 3000}
}

func (g *LLMGraph) Name() string { return "llm-graph" }

type extractEntity struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
type extractRelation struct {
	Subject   string  `json:"subject"`
	Predicate string  `json:"predicate"`
	Object    string  `json:"object"`
	Confidence float64 `json:"confidence"`
}
type extractResult struct {
	Entities  []extractEntity  `json:"entities"`
	Relations []extractRelation `json:"relations"`
}

func (g *LLMGraph) Enrich(it *model.Item) error {
	cfg, err := g.st.PipelineConfigOfKind("chat")
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	text := strings.TrimSpace(stripHTML(it.Content))
	maxRun := g.MaxRun
	if maxRun <= 0 {
		maxRun = 3000
	}
	if r := []rune(text); len(r) > maxRun {
		text = string(r[:maxRun])
	}
	body := it.Title + "\n\n" + text
	if strings.TrimSpace(body) == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	raw, err := llm.Chat(ctx, cfg, "你是信息抽取引擎。",
		fmt.Sprintf(`从以下内容抽取实体与实体关系,只输出 JSON(不要其他文字),格式:
{"entities":[{"name":"实体名","type":"人|组织|产品|项目|技术|其他"}],
 "relations":[{"subject":"实体名","predicate":"关系短语","object":"实体名","confidence":0.9}]}
实体名用原文常见称呼;关系主客体必须在 entities 里出现;没有则输出空数组。

内容:
%s`, body))
	if err != nil {
		return fmt.Errorf("实体抽取失败: %w", err)
	}
	var res extractResult
	if err := json.Unmarshal([]byte(extractJSON(raw)), &res); err != nil {
		return fmt.Errorf("实体抽取结果解析失败: %w", err)
	}

	// 实体入库(名字归一 upsert)
	nameIDs := map[string]int64{}
	for _, e := range res.Entities {
		if strings.TrimSpace(e.Name) == "" {
			continue
		}
		id, err := g.st.UpsertEntity(e.Name, e.Type)
		if err != nil {
			return err
		}
		nameIDs[strings.ToLower(strings.TrimSpace(e.Name))] = id
		if err := g.st.LinkItemEntity(it.ID, id, e.Name, 1); err != nil {
			return err
		}
	}
	// 关系入库(主客体必须能解析到实体)
	for _, r := range res.Relations {
		sid, ok1 := nameIDs[strings.ToLower(strings.TrimSpace(r.Subject))]
		oid, ok2 := nameIDs[strings.ToLower(strings.TrimSpace(r.Object))]
		if !ok1 || !ok2 || sid == oid || strings.TrimSpace(r.Predicate) == "" {
			continue
		}
		if err := g.st.AddRelation(sid, r.Predicate, oid, it.ID, r.Confidence); err != nil {
			return err
		}
	}

	// 向量化(有默认 embedding 配置才做)
	if ecfg, err := g.st.PipelineConfigOfKind("embedding"); err == nil && ecfg != nil {
		textForVec := it.Summary
		if strings.TrimSpace(textForVec) == "" {
			textForVec = it.Title
		}
		if vecs, err := llm.Embed(ctx, ecfg, []string{textForVec}); err == nil && len(vecs) == 1 {
			_ = g.st.SetItemEmbedding(it.ID, ecfg.Model, vecs[0])
		}
		// 实体名批量向量化
		if len(nameIDs) > 0 {
			names := make([]string, 0, len(nameIDs))
			ids := make([]int64, 0, len(nameIDs))
			for n, id := range nameIDs {
				names = append(names, n)
				ids = append(ids, id)
			}
			if vecs, err := llm.Embed(ctx, ecfg, names); err == nil {
				for i, v := range vecs {
					_ = g.st.SetEntityEmbedding(ids[i], ecfg.Model, v)
				}
			}
		}
	}
	return nil
}

// extractJSON 从 LLM 输出里抠出第一个 JSON 对象(容忍 ```json 围栏与前后杂文字)。
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			return s[i : j+1]
		}
	}
	return s
}
