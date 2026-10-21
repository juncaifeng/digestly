import { useEffect, useMemo, useState } from "react"
import { createPortal } from "react-dom"
import {
  BookTemplate, ChevronDown, ChevronLeft, ChevronRight, Eye, FolderOpen, Pencil,
  Plus, Save, Sparkles, Trash2,
} from "lucide-react"

import {
  api, articleApi, columnApi, creationApi, templateApi,
  type Article, type ArticleTemplate, type Column, type Creation, type CreationPlan, type CreationSource, type Feed,
} from "@/api/client"
import { TemplateManager } from "@/components/TemplateManager"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"

// 创作中心: 创作单元是一等公民(不与专栏强绑定),按文件夹分组;
// 每个创作 = 大纲模板 + 素材源(可勾选整个专栏或单个订阅源) + 历史创作。
// 模板管理作为本子菜单。
export function CreationCenterView({ onGenerated }: { onGenerated: () => void }) {
  const [creations, setCreations] = useState<Creation[]>([])
  const [columns, setColumns] = useState<Column[]>([])
  const [feeds, setFeeds] = useState<Feed[]>([])
  const [templates, setTemplates] = useState<ArticleTemplate[]>([])
  const [articles, setArticles] = useState<Article[]>([])
  const [error, setError] = useState("")

  // 左侧选中: 创作 id / "templates" / null
  const [sel, setSel] = useState<number | "templates" | null>(null)
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({})
  const [newName, setNewName] = useState("")
  const [newGroup, setNewGroup] = useState("")
  // 主侧栏插槽(App.tsx 渲染,挂载后才能取到)
  const [treeSlot, setTreeSlot] = useState<HTMLElement | null>(null)
  useEffect(() => { setTreeSlot(document.getElementById("creation-tree-slot")) }, [])

  // 配置草稿(选中创作后本地编辑,保存时整体提交)
  const [draft, setDraft] = useState<{ name: string; group_name: string; template_id: number; sources: CreationSource[] } | null>(null)
  const [busy, setBusy] = useState(false)
  // 未读耗尽时提供「强制生成」(取最新素材,不限未读)
  const [needForce, setNeedForce] = useState(false)
  // plan 预览(不调用 LLM)
  const [plan, setPlan] = useState<CreationPlan | null>(null)
  const [planBusy, setPlanBusy] = useState(false)

  // 查看/编辑单篇
  const [current, setCurrent] = useState<Article | null>(null)
  const [editing, setEditing] = useState(false)
  const [articleDraft, setArticleDraft] = useState({ title: "", content: "" })

  const load = () => {
    creationApi.list().then(setCreations).catch((e) => setError(e.message))
    columnApi.list().then(setColumns).catch(() => {})
    api.listFeeds().then(setFeeds).catch(() => {})
    templateApi.list().then(setTemplates).catch(() => {})
  }
  useEffect(() => { void load() }, [])

  const selected = typeof sel === "number" ? creations.find((c) => c.id === sel) ?? null : null

  // 选中创作 -> 初始化配置草稿 + 加载历史
  useEffect(() => {
    if (!selected) { setDraft(null); setArticles([]); return }
    setNeedForce(false)
    setPlan(null)
    setDraft({
      name: selected.name,
      group_name: selected.group_name,
      template_id: selected.template_id,
      sources: selected.sources ?? [],
    })
    articleApi.list(selected.id).then(setArticles).catch(() => setArticles([]))
  }, [selected?.id, creations])

  // 按文件夹分组,未分组在底部
  const groups = useMemo(() => {
    const m = new Map<string, Creation[]>()
    for (const c of creations) {
      const g = c.group_name.trim()
      if (!m.has(g)) m.set(g, [])
      m.get(g)!.push(c)
    }
    const named = [...m.entries()].filter(([g]) => g !== "").sort(([a], [b]) => a.localeCompare(b))
    const ungrouped = m.get("") ?? []
    return { named, ungrouped }
  }, [creations])

  const addCreation = async () => {
    if (!newName.trim()) return
    try {
      const c = await creationApi.create({ name: newName.trim(), group_name: newGroup.trim() || undefined })
      setNewName("")
      load()
      setSel(c.id)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const saveConfig = async () => {
    if (!selected || !draft) return
    setError("")
    try {
      await creationApi.update(selected.id, {
        name: draft.name,
        group_name: draft.group_name,
        template_id: draft.template_id,
        sources: draft.sources,
      })
      load()
    } catch (e) {
      setError(`保存失败: ${(e as Error).message}`)
    }
  }

  const removeCreation = async () => {
    if (!selected || !window.confirm(`删除创作「${selected.name}」? 其历史文章将一并删除。`)) return
    await creationApi.remove(selected.id)
    setSel(null)
    load()
  }

  const generate = async (force = false) => {
    if (!selected || !draft) return
    setBusy(true)
    setError("")
    try {
      // 先用草稿配置落库,再按草稿模板生成(未选模板则用创作默认)
      await creationApi.update(selected.id, {
        name: draft.name,
        group_name: draft.group_name,
        template_id: draft.template_id,
        sources: draft.sources,
      })
      const a = await creationApi.generate(selected.id, draft.template_id || undefined, force)
      setNeedForce(false)
      load()
      articleApi.list(selected.id).then(setArticles).catch(() => {})
      onGenerated() // 素材已标记已读,刷新未读数
      setCurrent(a)
      setArticleDraft({ title: a.title, content: a.content })
      setEditing(false)
    } catch (e) {
      const msg = (e as Error).message
      // 未读耗尽: 提供强制生成(取最新素材)
      setNeedForce(!force && msg.includes("没有未读文章"))
      setError(`生成失败: ${msg}`)
    } finally {
      setBusy(false)
    }
  }

  const preview = async (force = false) => {
    if (!selected || !draft) return
    setPlanBusy(true)
    setError("")
    try {
      await creationApi.update(selected.id, {
        name: draft.name,
        group_name: draft.group_name,
        template_id: draft.template_id,
        sources: draft.sources,
      })
      const p = await creationApi.plan(selected.id, draft.template_id || undefined, force)
      setPlan(p)
    } catch (e) {
      const msg = (e as Error).message
      setNeedForce(!force && msg.includes("没有未读文章"))
      setError(`预览失败: ${msg}`)
    } finally {
      setPlanBusy(false)
    }
  }

  const hasSrc = (kind: CreationSource["kind"], refId: number) =>
    draft?.sources.some((s) => s.kind === kind && s.ref_id === refId) ?? false

  const toggleSrc = (kind: CreationSource["kind"], refId: number) => {
    if (!draft) return
    const rest = draft.sources.filter((s) => !(s.kind === kind && s.ref_id === refId))
    setDraft({
      ...draft,
      sources: rest.length === draft.sources.length ? [...rest, { kind, ref_id: refId }] : rest,
    })
  }

  const openArticle = async (id: number) => {
    try {
      const a = await articleApi.get(id)
      setCurrent(a)
      setArticleDraft({ title: a.title, content: a.content })
      setEditing(false)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const saveArticle = async () => {
    if (!current) return
    await articleApi.update(current.id, articleDraft)
    setCurrent({ ...current, ...articleDraft })
    setEditing(false)
    if (selected) articleApi.list(selected.id).then(setArticles).catch(() => {})
  }

  const removeArticle = async () => {
    if (!current || !window.confirm(`删除「${current.title}」?`)) return
    await articleApi.remove(current.id)
    if (selected) articleApi.list(selected.id).then(setArticles).catch(() => {})
    setCurrent(null)
  }

  const tplName = (id: number) => templates.find((t) => t.id === id)?.name ?? ""
  const colName = (id: number | null) => columns.find((c) => c.id === id)?.name ?? "未分组"

  const creationRow = (c: Creation, indent: boolean) => (
    <div key={c.id} className={"flex items-center gap-0.5 " + (indent ? "pl-6" : "")}>
      <Button
        variant={sel === c.id ? "secondary" : "ghost"}
        size="sm"
        className="flex-1 justify-start"
        onClick={() => { setSel(c.id); setCurrent(null) }}
      >
        <span className="truncate">{c.name}</span>
      </Button>
    </div>
  )

  // 单篇查看/编辑
  if (current) {
    return (
      <div className="flex-1 overflow-y-auto p-4">
        <div className="max-w-3xl flex flex-col gap-3">
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="icon" onClick={() => setCurrent(null)}>
              <ChevronLeft className="size-4" />
            </Button>
            {editing ? (
              <Input className="h-8 flex-1" value={articleDraft.title}
                onChange={(e) => setArticleDraft({ ...articleDraft, title: e.target.value })} />
            ) : (
              <h2 className="text-lg font-semibold flex-1 truncate">{current.title}</h2>
            )}
            <Badge variant="outline">{tplName(current.template_id)}</Badge>
            {editing ? (
              <Button size="sm" onClick={saveArticle}><Save className="size-3.5" /> 保存</Button>
            ) : (
              <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
                <Pencil className="size-3.5" /> 编辑
              </Button>
            )}
            <Button size="sm" variant="ghost" onClick={removeArticle}><Trash2 className="size-3.5" /></Button>
          </div>
          {editing ? (
            <textarea
              className="w-full border rounded-md p-3 text-sm min-h-96 bg-transparent"
              value={articleDraft.content}
              onChange={(e) => setArticleDraft({ ...articleDraft, content: e.target.value })}
            />
          ) : (
            <div className="text-sm whitespace-pre-wrap leading-relaxed">{current.content}</div>
          )}
          <p className="text-xs text-muted-foreground">
            消耗素材 {current.item_ids?.length ?? 0} 篇 · {new Date(current.created_at).toLocaleString()}
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="flex-1 flex overflow-hidden">
      {/* 左树渲染到主侧栏(App.tsx 的 #creation-tree-slot),与文章页一致 */}
      {treeSlot && createPortal(
        <>
          <div className="flex flex-col gap-1.5">
            <Input className="h-8" placeholder="新建创作,如 日报1" value={newName}
              onChange={(e) => setNewName(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && addCreation()} />
            <div className="flex gap-1">
              <Input className="h-8" placeholder="文件夹(可选),如 AI 日报" value={newGroup}
                onChange={(e) => setNewGroup(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && addCreation()} />
              <Button size="sm" variant="outline" className="shrink-0" onClick={addCreation} disabled={!newName.trim()}>
                <Plus className="size-4" />
              </Button>
            </div>
          </div>

          <nav className="flex-1 flex flex-col gap-0.5 overflow-y-auto">
            {groups.named.map(([g, cs]) => (
              <div key={g} className="flex flex-col gap-0.5">
                <div className="flex items-center gap-0.5">
                  <Button variant="ghost" size="icon" className="size-7 shrink-0"
                    onClick={() => setCollapsed({ ...collapsed, [g]: !collapsed[g] })}>
                    {collapsed[g] ? <ChevronRight className="size-3.5" /> : <ChevronDown className="size-3.5" />}
                  </Button>
                  <Button variant="ghost" size="sm" className="flex-1 justify-start"
                    onClick={() => setCollapsed({ ...collapsed, [g]: !collapsed[g] })}>
                    <FolderOpen className="size-3.5 shrink-0" />
                    <span className="truncate">{g}</span>
                    <span className="ml-auto text-xs text-muted-foreground">{cs.length}</span>
                  </Button>
                </div>
                {!collapsed[g] && cs.map((c) => creationRow(c, true))}
              </div>
            ))}
            {groups.ungrouped.length > 0 && (
              <>
                {groups.named.length > 0 && (
                  <div className="px-2 pt-2 text-xs text-muted-foreground">未分组</div>
                )}
                {groups.ungrouped.map((c) => creationRow(c, false))}
              </>
            )}
            {creations.length === 0 && (
              <p className="text-xs text-muted-foreground px-2 pt-2">
                还没有创作 — 在上方新建一个,如「AI 日报 / 日报1」
              </p>
            )}
          </nav>

          <Button
            variant={sel === "templates" ? "secondary" : "ghost"}
            size="sm"
            className="justify-start shrink-0"
            onClick={() => { setSel("templates"); setCurrent(null) }}
          >
            <BookTemplate className="size-4" /> 模板管理
          </Button>
        </>,
        treeSlot,
      )}

      {/* 右: 配置 / 模板管理 */}
      <main className="flex-1 overflow-y-auto p-4">
        {error && <p className="text-sm text-destructive mb-3">{error}</p>}

        {sel === "templates" ? (
          <div className="max-w-3xl">
            <TemplateManager onChanged={() => templateApi.list().then(setTemplates).catch(() => {})} />
          </div>
        ) : selected && draft ? (
          <div className="max-w-3xl flex flex-col gap-4">
            <Card>
              <CardHeader className="flex-row items-center justify-between gap-2 space-y-0">
                <CardTitle className="text-base truncate">{selected.name}</CardTitle>
                <div className="flex gap-1 shrink-0">
                  <Button size="sm" variant="outline" onClick={saveConfig}>
                    <Save className="size-3.5" /> 保存配置
                  </Button>
                  <Button size="sm" variant="outline" disabled={planBusy || !draft.template_id || draft.sources.length === 0}
                    title="预览模板 + 素材 + 完整 prompt,不调用 LLM"
                    onClick={() => plan ? setPlan(null) : preview(needForce)}>
                    <Eye className="size-3.5" /> {planBusy ? "预览中…" : plan ? "收起预览" : "预览"}
                  </Button>
                  <Button size="sm" disabled={busy || !draft.template_id || draft.sources.length === 0}
                    title={!draft.template_id ? "先选择大纲模板" : draft.sources.length === 0 ? "先勾选素材源" : "基于最新未读素材生成"}
                    onClick={() => generate()}>
                    <Sparkles className="size-3.5" /> {busy ? "生成中…" : "生成"}
                  </Button>
                  {needForce && (
                    <Button size="sm" variant="outline" disabled={busy}
                      title="素材源未读已耗尽,取最新文章作为素材强制生成"
                      onClick={() => generate(true)}>
                      <Sparkles className="size-3.5" /> 强制生成
                    </Button>
                  )}
                  <Button size="sm" variant="ghost" onClick={removeCreation}>
                    <Trash2 className="size-3.5" />
                  </Button>
                </div>
              </CardHeader>
              <CardContent className="flex flex-col gap-3">
                <div className="grid grid-cols-2 gap-2">
                  <Input placeholder="创作名称" value={draft.name}
                    onChange={(e) => setDraft({ ...draft, name: e.target.value })} />
                  <Input placeholder="文件夹(留空 = 未分组)" value={draft.group_name}
                    onChange={(e) => setDraft({ ...draft, group_name: e.target.value })} />
                </div>
                <select
                  className="border rounded-md h-9 px-2 text-sm bg-transparent"
                  value={draft.template_id || 0}
                  onChange={(e) => setDraft({ ...draft, template_id: Number(e.target.value) })}
                >
                  <option value={0}>选择大纲模板…</option>
                  {templates.map((t) => (
                    <option key={t.id} value={t.id}>{t.name}</option>
                  ))}
                </select>

                {/* 素材源: 整个专栏 或 单个订阅源 */}
                <div className="flex flex-col gap-2">
                  <span className="text-xs text-muted-foreground">素材源(勾选专栏 = 取其全部订阅源)</span>
                  {columns.length > 0 && (
                    <div className="flex flex-col gap-1">
                      {columns.map((c) => (
                        <label key={c.id} className="flex items-center gap-2 text-sm px-1">
                          <input type="checkbox"
                            checked={hasSrc("column", c.id)}
                            onChange={() => toggleSrc("column", c.id)} />
                          <FolderOpen className="size-3.5 text-muted-foreground" />
                          <span className="truncate">{c.name}</span>
                          <span className="text-xs text-muted-foreground">{c.feed_count} 源</span>
                        </label>
                      ))}
                    </div>
                  )}
                  <div className="flex flex-col gap-1">
                    {feeds.map((f) => (
                      <label key={f.id} className="flex items-center gap-2 text-sm px-1">
                        <input type="checkbox"
                          checked={hasSrc("feed", f.id)}
                          onChange={() => toggleSrc("feed", f.id)} />
                        <span className="truncate">{f.title}</span>
                        <span className="text-xs text-muted-foreground shrink-0">{colName(f.column_id)}</span>
                        {f.unread_count > 0 && (
                          <span className="ml-auto text-xs text-muted-foreground shrink-0">{f.unread_count} 未读</span>
                        )}
                      </label>
                    ))}
                  </div>
                </div>
              </CardContent>
            </Card>

            {plan && (
              <Card>
                <CardHeader className="space-y-0">
                  <CardTitle className="text-base">
                    生成预览 · 模板「{plan.template?.name ?? ""}」 · 素材 {plan.item_ids?.length ?? 0} 篇
                    {plan.force && <Badge variant="secondary" className="ml-2">强制(不限未读)</Badge>}
                  </CardTitle>
                </CardHeader>
                <CardContent className="flex flex-col gap-2 text-sm">
                  <div className="flex flex-col gap-1">
                    {(plan.items ?? []).map((it) => (
                      <div key={it.id} className="flex items-baseline gap-2 px-1">
                        <span className="text-xs text-muted-foreground shrink-0">#{it.id}</span>
                        <span className="truncate">{it.title}</span>
                        {it.source && <span className="text-xs text-muted-foreground shrink-0">{it.source}</span>}
                        {!it.read && <Badge variant="secondary" className="shrink-0">未读</Badge>}
                      </div>
                    ))}
                  </div>
                  {(plan.history_titles?.length ?? 0) > 0 && (
                    <p className="text-xs text-muted-foreground">
                      历史上下文 {plan.history_titles.length} 篇: {plan.history_titles[0]} 等
                    </p>
                  )}
                  <details>
                    <summary className="text-xs text-muted-foreground cursor-pointer">完整 prompt({(plan.prompt ?? "").length} 字符)</summary>
                    <pre className="text-xs text-muted-foreground whitespace-pre-wrap border rounded-md p-2 mt-1 max-h-96 overflow-y-auto">{plan.prompt}</pre>
                  </details>
                </CardContent>
              </Card>
            )}

            <div className="flex flex-col gap-1">
              <span className="text-xs text-muted-foreground">历史创作 {articles.length} 篇</span>
              {articles.map((a) => (
                <button
                  key={a.id}
                  className="text-left text-sm px-2 py-1.5 rounded hover:bg-accent truncate"
                  onClick={() => openArticle(a.id)}
                >
                  {a.title}
                </button>
              ))}
              {articles.length === 0 && (
                <span className="text-xs text-muted-foreground px-2">还没有创作记录</span>
              )}
            </div>
          </div>
        ) : (
          <p className="text-sm text-muted-foreground text-center mt-10">
            在左侧选择一个创作,或进入「模板管理」
          </p>
        )}
      </main>
    </div>
  )
}
