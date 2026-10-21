import { useEffect, useMemo, useState } from "react"
import {
  BookTemplate, ChevronDown, ChevronLeft, ChevronRight, Eye, FileText, FolderOpen, Pencil,
  Plus, Save, Send, Settings2, Sparkles, Trash2, X,
} from "lucide-react"

import {
  api, articleApi, columnApi, creationApi, templateApi,
  type Article, type ArticleTemplate, type Column, type Creation, type CreationMessage, type CreationPlan, type CreationSource, type Feed,
} from "@/api/client"
import { TemplateManager } from "@/components/TemplateManager"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"

// 创作中心三栏: 左=项目列表, 中=对话(生成即一轮), 右=产出文件。
// 项目配置走弹窗;上下文注入仍以勾选素材源(专栏/订阅源)为准。
export function CreationCenterView({ onGenerated }: { onGenerated: () => void }) {
  const [creations, setCreations] = useState<Creation[]>([])
  const [columns, setColumns] = useState<Column[]>([])
  const [feeds, setFeeds] = useState<Feed[]>([])
  const [templates, setTemplates] = useState<ArticleTemplate[]>([])
  const [articles, setArticles] = useState<Article[]>([])
  const [msgs, setMsgs] = useState<CreationMessage[]>([])
  const [error, setError] = useState("")

  // 左栏: 选中项目 / "templates" / null
  const [sel, setSel] = useState<number | "templates" | null>(null)
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({})
  const [newName, setNewName] = useState("")
  const [newGroup, setNewGroup] = useState("")

  // 配置弹窗
  const [cfgOpen, setCfgOpen] = useState(false)
  const [draft, setDraft] = useState<{ name: string; group_name: string; template_id: number; sources: CreationSource[] } | null>(null)

  // 中栏: 对话输入与生成
  const [msg, setMsg] = useState("")
  const [busy, setBusy] = useState(false)
  const [needForce, setNeedForce] = useState(false)
  const [plan, setPlan] = useState<CreationPlan | null>(null) // 预览气泡(临时,不落库)
  const [planBusy, setPlanBusy] = useState(false)

  // 右栏: 产出文件
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

  // 切换项目: 载入会话线程 + 产出文件, 清空本轮状态
  useEffect(() => {
    if (!selected) { setArticles([]); setMsgs([]); return }
    setNeedForce(false)
    setPlan(null)
    setMsg("")
    setCurrent(null)
    articleApi.list(selected.id).then(setArticles).catch(() => setArticles([]))
    creationApi.messages(selected.id).then(setMsgs).catch(() => setMsgs([]))
  }, [selected?.id, creations])

  const reloadThread = () => {
    if (!selected) return
    creationApi.messages(selected.id).then(setMsgs).catch(() => {})
    articleApi.list(selected.id).then(setArticles).catch(() => {})
  }

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

  const openConfig = () => {
    if (!selected) return
    setDraft({
      name: selected.name,
      group_name: selected.group_name,
      template_id: selected.template_id,
      sources: selected.sources ?? [],
    })
    setCfgOpen(true)
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
      setCfgOpen(false)
      load()
    } catch (e) {
      setError(`保存失败: ${(e as Error).message}`)
    }
  }

  const removeCreation = async () => {
    if (!selected || !window.confirm(`删除项目「${selected.name}」? 其产出文件将一并删除。`)) return
    await creationApi.remove(selected.id)
    setCfgOpen(false)
    setSel(null)
    load()
  }

  // 生成一轮(成文): 消耗素材并产出文件,同时记入会话线程
  const generate = async (force = false) => {
    if (!selected) return
    setBusy(true)
    setError("")
    try {
      const a = await creationApi.generate(selected.id, undefined, force, msg.trim() || undefined)
      setNeedForce(false)
      setMsg("")
      load()
      reloadThread()
      onGenerated() // 素材已标记已读,刷新未读数
      setCurrent(a)
      setArticleDraft({ title: a.title, content: a.content })
      setEditing(false)
    } catch (e) {
      const m = (e as Error).message
      setNeedForce(!force && m.includes("没有未读文章"))
      setError(`生成失败: ${m}`)
    } finally {
      setBusy(false)
    }
  }

  // 对话一轮: 不生成文件、不消耗素材;agent 后端会用 digestly-cli 自行取材
  const sendChat = async () => {
    if (!selected || !msg.trim()) return
    setBusy(true)
    setError("")
    try {
      await creationApi.chat(selected.id, msg.trim())
      setMsg("")
      reloadThread()
    } catch (e) {
      setError(`对话失败: ${(e as Error).message}`)
    } finally {
      setBusy(false)
    }
  }

  const preview = async () => {
    if (!selected) return
    setPlanBusy(true)
    setError("")
    try {
      setPlan(await creationApi.plan(selected.id))
    } catch (e) {
      setError(`预览失败: ${(e as Error).message}`)
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
    <div key={c.id} className={"group flex items-center gap-0.5 " + (indent ? "pl-4" : "")}>
      <Button
        variant={sel === c.id ? "secondary" : "ghost"}
        size="sm"
        className="flex-1 justify-start"
        onClick={() => { setSel(c.id); setCurrent(null) }}
      >
        <span className="truncate">{c.name}</span>
      </Button>
      <Button variant="ghost" size="icon" className="size-6 shrink-0 opacity-0 group-hover:opacity-100"
        title="项目配置" onClick={() => { setSel(c.id); setTimeout(openConfig, 0) }}>
        <Settings2 className="size-3.5" />
      </Button>
    </div>
  )

  return (
    <div className="flex-1 flex overflow-hidden">
      {/* 左: 项目 */}
      <aside className="w-56 shrink-0 border-r flex flex-col p-3 gap-2 overflow-hidden">
        <div className="flex flex-col gap-1.5">
          <Input className="h-8" placeholder="新建项目,如 日报1" value={newName}
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
              还没有项目 — 在上方新建一个,如「AI 日报 / 日报1」
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
      </aside>

      {/* 中: 对话 */}
      <main className="flex-1 flex flex-col overflow-hidden">
        {error && <p className="text-sm text-destructive px-4 pt-3">{error}</p>}

        {sel === "templates" ? (
          <div className="flex-1 overflow-y-auto p-4">
            <div className="max-w-3xl">
              <TemplateManager onChanged={() => templateApi.list().then(setTemplates).catch(() => {})} />
            </div>
          </div>
        ) : selected ? (
          <>
            <header className="flex items-center gap-2 border-b px-4 py-2.5">
              <h2 className="text-base font-semibold truncate flex-1">{selected.name}</h2>
              <Button size="sm" variant="outline" onClick={openConfig} title="项目配置(模板 / 素材源)">
                <Settings2 className="size-3.5" /> 配置
              </Button>
              <Button size="sm" variant="outline" disabled={planBusy}
                title="预览模板 + 素材 + 完整 prompt,不调用 LLM"
                onClick={() => plan ? setPlan(null) : preview()}>
                <Eye className="size-3.5" /> {planBusy ? "预览中…" : plan ? "收起预览" : "预览"}
              </Button>
            </header>

            <div className="flex-1 overflow-y-auto p-4 flex flex-col gap-3">
              {msgs.length === 0 && !plan && (
                <p className="text-sm text-muted-foreground text-center mt-10">
                  还没有对话 — 下方输入内容:「发送」是纯对话,「生成」成文并产出文件
                </p>
              )}
              {msgs.map((m) => (
                m.role === "user" ? (
                  <div key={m.id} className="self-end max-w-[80%] rounded-lg bg-accent px-3 py-2 text-sm whitespace-pre-wrap">
                    {m.content}
                  </div>
                ) : (
                  <div key={m.id} className="self-start max-w-[85%] rounded-lg border px-3 py-2 flex flex-col gap-1.5">
                    <div className="text-sm whitespace-pre-wrap leading-relaxed">
                      {m.article_id ? (
                        <div className="flex items-center gap-2">
                          <Sparkles className="size-3.5 shrink-0 text-muted-foreground" />
                          <button className="text-sm font-medium text-left truncate hover:underline"
                            onClick={() => openArticle(m.article_id!)}>
                            {m.content}
                          </button>
                        </div>
                      ) : (
                        m.content
                      )}
                    </div>
                    <div className="flex items-center gap-2 text-xs text-muted-foreground">
                      <span>{new Date(m.created_at).toLocaleString()}</span>
                      {m.article_id && (
                        <button className="hover:underline" onClick={() => openArticle(m.article_id!)}>打开文件</button>
                      )}
                    </div>
                  </div>
                )
              ))}

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
            </div>

            <footer className="border-t p-3 flex flex-col gap-2">
              {needForce && (
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  素材未读已耗尽
                  <Button size="sm" variant="outline" disabled={busy} onClick={() => generate(true)}>
                    <Sparkles className="size-3.5" /> 强制生成(取最新)
                  </Button>
                </div>
              )}
              <div className="flex gap-2 items-end">
                <textarea
                  className="flex-1 border rounded-md p-2 text-sm min-h-14 max-h-40 bg-transparent resize-y"
                  placeholder="输入内容:「发送」= 对话(不产文件);「生成」= 成文(消耗素材);上下文取自配置的素材源"
                  value={msg}
                  onChange={(e) => setMsg(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) generate()
                  }}
                />
                <Button size="sm" variant="outline" disabled={busy || !msg.trim()} title="纯对话,不产文件不消耗素材"
                  onClick={sendChat}>
                  <Send className="size-3.5" /> 发送
                </Button>
                <Button size="sm" disabled={busy} title="成文: 消耗素材生成文件,并记入会话"
                  onClick={() => generate()}>
                  <Sparkles className="size-3.5" /> {busy ? "生成中…" : "生成"}
                </Button>
              </div>
            </footer>
          </>
        ) : (
          <p className="text-sm text-muted-foreground text-center mt-10">
            在左侧选择一个项目,或进入「模板管理」
          </p>
        )}
      </main>

      {/* 右: 产出文件 */}
      <aside className="w-80 shrink-0 border-l flex flex-col overflow-hidden">
        {current ? (
          <div className="flex-1 overflow-y-auto p-3 flex flex-col gap-2">
            <div className="flex items-center gap-1">
              <Button variant="ghost" size="icon" className="size-7" onClick={() => setCurrent(null)}>
                <ChevronLeft className="size-4" />
              </Button>
              {editing ? (
                <Input className="h-8 flex-1" value={articleDraft.title}
                  onChange={(e) => setArticleDraft({ ...articleDraft, title: e.target.value })} />
              ) : (
                <h3 className="text-sm font-semibold flex-1 truncate">{current.title}</h3>
              )}
              {editing ? (
                <Button size="sm" variant="outline" onClick={saveArticle}><Save className="size-3.5" /> 保存</Button>
              ) : (
                <Button variant="ghost" size="icon" className="size-7" title="编辑"
                  onClick={() => setEditing(true)}>
                  <Pencil className="size-3.5" />
                </Button>
              )}
              <Button variant="ghost" size="icon" className="size-7" title="删除" onClick={removeArticle}>
                <Trash2 className="size-3.5" />
              </Button>
            </div>
            {current.user_message && (
              <p className="text-xs text-muted-foreground border rounded-md px-2 py-1.5">
                本轮指示: {current.user_message}
              </p>
            )}
            {editing ? (
              <textarea
                className="flex-1 w-full border rounded-md p-2 text-sm min-h-96 bg-transparent"
                value={articleDraft.content}
                onChange={(e) => setArticleDraft({ ...articleDraft, content: e.target.value })}
              />
            ) : (
              <div className="text-sm whitespace-pre-wrap leading-relaxed">{current.content}</div>
            )}
            <p className="text-xs text-muted-foreground">
              {tplName(current.template_id)} · 消耗素材 {current.item_ids?.length ?? 0} 篇 · {new Date(current.created_at).toLocaleString()}
            </p>
          </div>
        ) : (
          <>
            <div className="px-3 py-2.5 border-b text-sm font-medium flex items-center gap-2">
              <FileText className="size-4" /> 产出文件
              <span className="ml-auto text-xs text-muted-foreground">{articles.length} 篇</span>
            </div>
            <div className="flex-1 overflow-y-auto p-2 flex flex-col gap-0.5">
              {articles.map((a) => (
                <button
                  key={a.id}
                  className="text-left text-sm px-2 py-1.5 rounded hover:bg-accent truncate"
                  onClick={() => openArticle(a.id)}
                  title={a.title}
                >
                  {a.title}
                </button>
              ))}
              {articles.length === 0 && (
                <span className="text-xs text-muted-foreground px-2 py-1">还没有产出</span>
              )}
            </div>
          </>
        )}
      </aside>

      {/* 项目配置弹窗 */}
      {cfgOpen && draft && selected && (
        <div className="fixed inset-0 z-50 bg-black/40 flex items-center justify-center p-4"
          onClick={() => setCfgOpen(false)}>
          <div className="bg-background border rounded-xl shadow-lg w-full max-w-lg max-h-[85vh] overflow-y-auto p-4 flex flex-col gap-3"
            onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center gap-2">
              <h3 className="text-base font-semibold flex-1">项目配置 · {selected.name}</h3>
              <Button variant="ghost" size="icon" className="size-7" onClick={() => setCfgOpen(false)}>
                <X className="size-4" />
              </Button>
            </div>

            <div className="grid grid-cols-2 gap-2">
              <Input placeholder="项目名称" value={draft.name}
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

            {/* 上下文注入: 勾选素材源(专栏 = 其全部订阅源) */}
            <div className="flex flex-col gap-2">
              <span className="text-xs text-muted-foreground">素材源(上下文注入,勾选专栏 = 取其全部订阅源)</span>
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

            <div className="flex gap-2 pt-1">
              <Button size="sm" onClick={saveConfig} disabled={!draft.name.trim()}>
                <Save className="size-3.5" /> 保存配置
              </Button>
              <Button size="sm" variant="ghost" onClick={removeCreation}>
                <Trash2 className="size-3.5" /> 删除项目
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
