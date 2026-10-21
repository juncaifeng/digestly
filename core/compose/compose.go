// Package compose 创作编排: 取创作单元的未读素材 -> 模板拼装 -> LLM 生成 -> 落库 -> 素材标记已读。
package compose

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/juncaifeng/digestly/core/llm"
	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/store"
)

// MaxSourceItems 单次生成消耗的素材上限(最新未读优先)。
const MaxSourceItems = 20

// Plan 一次生成的完整输入预览: 不调用 LLM、不落库、不标记已读。
// 供外部 agent 取用 — 可直接拿 Prompt 喂自己的模型,或按 ItemIDs 分页细读素材。
type Plan struct {
	Creation      *model.Creation        `json:"creation"`
	Template      *model.ArticleTemplate `json:"template"`
	FeedIDs       []int64                `json:"feed_ids"`       // 素材源展开后的订阅源 id
	Force         bool                   `json:"force"`          // true = 不限未读,取最新素材
	ItemIDs       []int64                `json:"item_ids"`       // 将消耗的素材 id(排序截断后)
	Items         []model.Item           `json:"items"`          // 素材元数据(不含正文;正文按 id 单独取)
	HistoryTitles []string               `json:"history_titles"` // 注入上下文的历史标题(最近 10 篇)
	Prompt        string                 `json:"prompt"`         // 完整拼装后的 prompt
}

// BuildPlan 解析创作配置并拼装生成输入,不做任何副作用。
// templateID <= 0 时用创作关联的默认模板。
func BuildPlan(st *store.Store, creationID, templateID int64, force bool) (*Plan, error) {
	var creation *model.Creation
	creations, err := st.ListCreations()
	if err != nil {
		return nil, err
	}
	for i := range creations {
		if creations[i].ID == creationID {
			creation = &creations[i]
			break
		}
	}
	if creation == nil {
		return nil, fmt.Errorf("创作不存在(id=%d)", creationID)
	}
	if templateID <= 0 {
		templateID = creation.TemplateID
	}
	if templateID <= 0 {
		return nil, fmt.Errorf("该创作未关联大纲模板,请先选择模板")
	}
	tpl, err := st.GetTemplate(templateID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("模板不存在(id=%d)", templateID)
		}
		return nil, err
	}

	// 素材源展开为订阅源集合(专栏源展开为组内全部订阅),逐源取未读再合并
	feedIDs, err := st.CreationFeedIDs(creationID)
	if err != nil {
		return nil, err
	}
	if len(feedIDs) == 0 {
		return nil, fmt.Errorf("该创作还没有配置素材源,请先勾选专栏或订阅源")
	}
	var items = make([]model.Item, 0)
	for _, fid := range feedIDs {
		fis, err := st.ListItems(store.ItemFilter{FeedID: fid, Unread: !force, Limit: MaxSourceItems})
		if err != nil {
			return nil, err
		}
		items = append(items, fis...)
	}
	// 合并后按发布时间降序(无发布时间的排最后),截取上限
	sort.SliceStable(items, func(i, j int) bool {
		pi, pj := items[i].PublishedAt, items[j].PublishedAt
		if pi == nil {
			return false
		}
		if pj == nil {
			return true
		}
		if !pi.Equal(*pj) {
			return pi.After(*pj)
		}
		return items[i].ID > items[j].ID
	})
	if len(items) > MaxSourceItems {
		items = items[:MaxSourceItems]
	}
	if len(items) == 0 {
		if force {
			return nil, fmt.Errorf("素材源里没有任何文章")
		}
		return nil, fmt.Errorf("素材源里没有未读文章")
	}

	prompt := strings.ReplaceAll(tpl.Prompt, "{{items}}", renderItems(items))
	if !strings.Contains(tpl.Prompt, "{{items}}") {
		prompt = tpl.Prompt + "\n\n素材清单:\n" + renderItems(items)
	}
	// 历史创作标题作为上下文: 延续选题脉络、避免重复选题(最近 10 篇,新→旧)
	const maxHistory = 10
	var historyTitles = make([]string, 0, maxHistory)
	if history, err := st.ListArticles(creationID); err == nil {
		for i, a := range history {
			if i >= maxHistory {
				break
			}
			historyTitles = append(historyTitles, a.Title)
		}
	}
	if len(historyTitles) > 0 {
		var hb strings.Builder
		hb.WriteString("\n\n本创作已发布的历史文章标题(新→旧),请延续选题脉络、避免重复选题:\n")
		for _, t := range historyTitles {
			fmt.Fprintf(&hb, "- %s\n", t)
		}
		prompt += hb.String()
	}

	ids := make([]int64, 0, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
		items[i].Content = "" // 预览不带正文,按需按 id 单独取
	}
	return &Plan{
		Creation: creation, Template: tpl, FeedIDs: feedIDs, Force: force,
		ItemIDs: ids, Items: items, HistoryTitles: historyTitles, Prompt: prompt,
	}, nil
}

// Generate 基于创作单元配置的素材源(专栏 / 单个订阅源)生成一篇文章。
// templateID <= 0 时用创作关联的默认模板。
// force = true 时不限未读,直接取最新素材(用于未读耗尽后强制生成)。
// instruction 为本轮用户指示(可空),注入 prompt 并随文章保存为对话记录。
// 成功落库后素材标记为已读(防重复消费);生成失败不标记,素材可再次消费。
func Generate(ctx context.Context, st *store.Store, creationID, templateID int64, force bool, instruction string) (*model.Article, error) {
	plan, err := BuildPlan(st, creationID, templateID, force)
	if err != nil {
		return nil, err
	}
	cfg, err := st.DefaultLLMConfig()
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("还没有默认 LLM 配置,请先在「设置」页添加大模型配置")
		}
		return nil, err
	}
	prompt := plan.Prompt
	if instruction = strings.TrimSpace(instruction); instruction != "" {
		prompt += "\n\n用户本次指示(优先遵循):\n" + instruction
	}
	content, err := runBackend(ctx, st, cfg, prompt, plan)
	if err != nil {
		return nil, err
	}

	// 标题取首行 "# xxx";取不到则用日期兜底
	title := ""
	body := content
	if line, rest, ok := strings.Cut(content, "\n"); ok && strings.HasPrefix(strings.TrimSpace(line), "#") {
		title = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		body = strings.TrimSpace(rest)
	}
	if title == "" {
		title = plan.Creation.Name + " " + time.Now().Format("2006-01-02 15:04")
	}

	a := &model.Article{CreationID: creationID, TemplateID: plan.Template.ID, Title: title, Content: body, UserMessage: instruction, ItemIDs: plan.ItemIDs}
	if err := st.AddArticle(a); err != nil {
		return nil, err
	}
	for _, id := range plan.ItemIDs {
		if err := st.MarkRead(id, true); err != nil {
			return a, fmt.Errorf("文章已生成,但素材标记已读失败: %w", err)
		}
	}
	// 对话线程: 用户气泡(指示或「生成文章」) + assistant 气泡挂产出文件
	userText := instruction
	if userText == "" {
		userText = "生成文章"
	}
	_ = st.AddMessage(&model.CreationMessage{CreationID: creationID, Role: "user", Content: userText})
	_ = st.AddMessage(&model.CreationMessage{CreationID: creationID, Role: "assistant", Content: "生成文章: " + title, ArticleID: a.ID})
	return a, nil
}

// renderItems 把素材渲染成「序号. 标题 (来源)\n摘要」清单。
func renderItems(items []model.Item) string {
	var sb strings.Builder
	for i, it := range items {
		fmt.Fprintf(&sb, "%d. %s", i+1, it.Title)
		if it.Source != "" {
			fmt.Fprintf(&sb, " (%s)", it.Source)
		}
		sb.WriteString("\n")
		if it.Summary != "" {
			fmt.Fprintf(&sb, "   摘要: %s\n", it.Summary)
		}
	}
	return sb.String()
}

// Chat 多轮对话一轮: 不生成文件、不消耗素材。
// 线程上下文取最近 maxHistory 条会话; agent 后端额外给 digestly-cli 工具简报,
// 让它自己取素材全文(真正走 skills); chat 后端则静态注入素材清单。
func Chat(ctx context.Context, st *store.Store, creationID int64, message string) (string, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return "", fmt.Errorf("消息不能为空")
	}
	creations, err := st.ListCreations()
	if err != nil {
		return "", err
	}
	var creation *model.Creation
	for i := range creations {
		if creations[i].ID == creationID {
			creation = &creations[i]
			break
		}
	}
	if creation == nil {
		return "", fmt.Errorf("创作不存在(id=%d)", creationID)
	}
	cfg, err := st.DefaultLLMConfig()
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("还没有默认 LLM 配置,请先在「设置」页添加大模型配置")
		}
		return "", err
	}

	// 会话线程(最近 10 条)拼进 prompt
	var sb strings.Builder
	sb.WriteString("这是创作项目「" + creation.Name + "」的对话。\n")
	if msgs, err := st.ListMessages(creationID); err == nil && len(msgs) > 0 {
		const maxHistory = 10
		start := len(msgs) - maxHistory
		if start < 0 {
			start = 0
		}
		sb.WriteString("\n## 对话记录(旧→新)\n")
		for _, m := range msgs[start:] {
			who := "用户"
			if m.Role == "assistant" {
				who = "助手"
			}
			fmt.Fprintf(&sb, "%s: %s\n", who, m.Content)
		}
	}
	fmt.Fprintf(&sb, "\n## 用户本轮消息\n%s\n", message)

	plan, planErr := BuildPlan(st, creationID, 0, true) // 对话取最新素材做上下文,不限未读
	if cfg.Kind == "agent" {
		sb.WriteString(agentBrief(st, creation, plan, planErr))
	} else {
		if planErr == nil {
			sb.WriteString("\n## 素材上下文(仅参考)\n" + renderItems(plan.Items) + "\n")
		}
	}

	reply, err := runBackend(ctx, st, cfg, sb.String(), nil)
	if err != nil {
		return "", err
	}
	// 线程落库: 用户消息 + 助手回复(不挂文件)
	_ = st.AddMessage(&model.CreationMessage{CreationID: creationID, Role: "user", Content: message})
	_ = st.AddMessage(&model.CreationMessage{CreationID: creationID, Role: "assistant", Content: reply})
	return reply, nil
}

// runBackend 按配置的生成后端执行一次调用。
// plan 非空时(生成场景)把素材清单并入 agent 简报,让 agent 可自行用 CLI 深读。
func runBackend(ctx context.Context, st *store.Store, cfg *model.LLMConfig, prompt string, plan *Plan) (string, error) {
	if cfg.Kind == "agent" {
		env, brief := agentEnv(st)
		full := prompt
		if plan != nil {
			full += "\n" + brief
		}
		return llm.RunAgent(ctx, cfg.Command, full, env)
	}
	return llm.Chat(ctx, cfg, "你是一名专业的内容创作者。", prompt)
}

// agentEnv 组装 agent 进程环境: PATH 前置 digestly-cli 所在目录,便于其直接调用。
func agentEnv(st *store.Store) (env []string, brief string) {
	cli, dataDir := resolveCLI(st)
	env = append(env, "DIGESTLY_DATA="+dataDir)
	if d := filepath.Dir(cli); d != "." {
		env = append(env, "PATH="+d+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	return env, agentBriefData(dataDir)
}

// agentBrief 生成场景的工具简报(追加到 prompt 尾部)。
func agentBrief(st *store.Store, creation *model.Creation, plan *Plan, planErr error) string {
	_, dataDir := resolveCLI(st)
	var sb strings.Builder
	sb.WriteString(agentBriefData(dataDir))
	if creation != nil {
		fmt.Fprintf(&sb, "\n本创作: id=%d 名称=%q。先跑 plan 看素材,按需 get 深读。\n", creation.ID, creation.Name)
	}
	if planErr != nil {
		fmt.Fprintf(&sb, "plan 提示: %v(可自行用 list 检索其它素材)\n", planErr)
	} else if plan != nil {
		fmt.Fprintf(&sb, "plan 已就绪: %d 篇素材(id: %v)。建议按需 get 深读后成文。\n", len(plan.ItemIDs), plan.ItemIDs)
	}
	return sb.String()
}

// agentBriefData digestly-cli 工具简报(命令以绝对 -data 写就,开箱即用)。
func agentBriefData(dataDir string) string {
	cli, _ := resolveCLIPath()
	return fmt.Sprintf(`
## 你的工具: digestly-cli(已安装,直接执行即可调用本机 digestly 数据)

数据目录: %s
CLI: %s

常用命令:
- 列素材:   %q -cmd list -data %q [-unread] [-limit 20] [-offset 0] [-q 关键词]
- 素材计划: %q -cmd plan -creation <id> -data %q [-ids-only]
- 读全文:   %q -cmd get -id <素材id> -data %q
- 发现:     %q -cmd feeds|columns|templates|creation-list -data %q

策略: 先 plan 或 list 挑选相关素材 → get 深读重点几篇 → 再动笔成文。
`, dataDir, cli, cli, dataDir, cli, dataDir, cli, dataDir, cli, dataDir)
}

// resolveCLI 解析 digestly-cli 路径与数据目录(供 agent 调用)。
func resolveCLI(st *store.Store) (cli, dataDir string) {
	cli, _ = resolveCLIPath()
	dataDir = "."
	if st != nil && st.Path != "" {
		dataDir = filepath.Dir(st.Path)
	}
	return cli, dataDir
}

// resolveCLIPath 依次找 PATH、可执行文件同目录下的 digestly-cli。
func resolveCLIPath() (string, error) {
	name := "digestly-cli"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return name, fmt.Errorf("digestly-cli 不在 PATH 也不在可执行文件同目录")
}
