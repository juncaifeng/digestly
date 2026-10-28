import { useEffect, useState } from "react"
import { AlertTriangle, CheckCircle2, Clock, ListChecks, Play, RefreshCw, RotateCcw, Zap } from "lucide-react"

import { api, pipelineApi, type Item, type ItemStatus, type PipelineLog, type PipelineStatus } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

const statusVariant: Record<ItemStatus, "default" | "secondary" | "destructive" | "outline"> = {
  pending: "secondary",
  processing: "outline",
  processed: "default",
  failed: "destructive",
}

type Filter = ItemStatus | ""

// 任务页: 标注管线管理 + 逐篇处理状态。管线新拉取自动跑,这里手动补跑/重试/重跑。
export function TasksView() {
  const [st, setSt] = useState<PipelineStatus | null>(null)
  const [filter, setFilter] = useState<Filter>("")
  const [items, setItems] = useState<Item[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const [notice, setNotice] = useState("")
  // 单任务展开: 管线运行日志
  const [openId, setOpenId] = useState<number | null>(null)
  const [logs, setLogs] = useState<PipelineLog[]>([])

  const openLogs = (id: number) => {
    if (openId === id) {
      setOpenId(null)
      return
    }
    setOpenId(id)
    setLogs([])
    api.getPipelineLogs(id).then(setLogs).catch(() => setLogs([]))
  }

  const loadStatus = () => {
    pipelineApi.status().then(setSt).catch((e) => setError(`状态加载失败: ${e.message}`))
  }
  const loadItems = () => {
    api.listItems({ status: filter || undefined, limit: 50 })
      .then(setItems)
      .catch(() => {})
  }
  useEffect(() => { loadStatus(); loadItems() }, [filter])
  // 轮询状态(2.5s),批次跑起来时进度实时可见
  useEffect(() => {
    const t = setInterval(loadStatus, 2500)
    return () => clearInterval(t)
  }, [])

  const act = async (fn: () => Promise<unknown>, msg: string) => {
    setBusy(true); setError(""); setNotice("")
    try {
      await fn()
      setNotice(msg)
      loadStatus()
      loadItems()
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const cards: { key: Filter; label: string; count: number; icon: React.ReactNode }[] = [
    { key: "pending", label: "待处理", count: st?.pending ?? 0, icon: <Clock className="size-4" /> },
    { key: "processing", label: "处理中", count: st?.processing ?? 0, icon: <RefreshCw className="size-4" /> },
    { key: "processed", label: "已处理", count: st?.processed ?? 0, icon: <CheckCircle2 className="size-4" /> },
    { key: "failed", label: "失败", count: st?.failed ?? 0, icon: <AlertTriangle className="size-4" /> },
  ]
  const running = st?.batch.running ?? false

  return (
    <div className="flex-1 overflow-y-auto p-4">
      <div className="max-w-4xl flex flex-col gap-4">
        <div className="flex items-center gap-2">
          <h2 className="text-base font-semibold flex items-center gap-2">
            <ListChecks className="size-5" /> 处理任务
          </h2>
          {running && <Badge variant="secondary">批次执行中…</Badge>}
        </div>
        {error && <p className="text-sm text-destructive">{error}</p>}
        {notice && <p className="text-sm text-green-600">{notice}</p>}

        {/* 状态卡片(点击筛选) */}
        <div className="grid grid-cols-4 gap-2">
          {cards.map((c) => (
            <button key={c.key}
              className={"border rounded-lg p-3 text-left flex flex-col gap-1 hover:bg-accent/50 " +
                (filter === c.key ? "ring-1 ring-ring bg-accent/40" : "")}
              onClick={() => setFilter(filter === c.key ? "" : c.key)}
            >
              <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
                {c.icon} {c.label}
              </span>
              <span className="text-xl font-semibold">{c.count}</span>
            </button>
          ))}
        </div>

        {/* 管线 */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm">标注管线</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-2">
            <div className="flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
              {(st?.steps ?? []).map((s, i) => (
                <span key={s} className="flex items-center gap-1">
                  {i > 0 && <span>→</span>}
                  <Badge variant="outline" className="font-mono">{s}</Badge>
                </span>
              ))}
            </div>
            <p className="text-xs text-muted-foreground">
              新拉取的文章自动进管线;LLM 步骤(滑窗摘要/实体图谱)在配置 chat 模型后生效,否则跳过。
              重跑会调用 LLM 产生费用。
            </p>
            {running && (
              <p className="text-xs text-muted-foreground">
                进度: {st?.batch.processed ?? 0} / {st?.batch.total ?? 0}
              </p>
            )}
            {!running && st?.batch.last_error && (
              <p className="text-xs text-destructive">上次批次错误: {st.batch.last_error}</p>
            )}
            <div className="flex flex-wrap gap-2">
              <Button size="sm" disabled={busy || running}
                title="处理一批待处理文章(后台执行)"
                onClick={() => act(() => pipelineApi.run(), "已启动一批处理,注意进度")}>
                <Play className="size-3.5" /> 开始处理
              </Button>
              <Button size="sm" variant="outline" disabled={busy || running}
                title="失败文章退回待处理并重跑一批"
                onClick={() => act(() => pipelineApi.retry(), "已启动失败重试")}>
                <RotateCcw className="size-3.5" /> 重试失败
              </Button>
              <Button size="sm" variant="outline" disabled={busy || running}
                title="已处理文章清空标注重新跑(会调用 LLM,产生费用)"
                onClick={() => {
                  if (window.confirm("全部重跑会清空现有标注并重新处理(含 LLM 调用,产生费用),确定?")) {
                    void act(() => pipelineApi.reprocess(false), "已启动全部重跑")
                  }
                }}>
                <Zap className="size-3.5" /> 全部重跑
              </Button>
              <Button size="sm" variant="ghost" onClick={() => { loadStatus(); loadItems() }}>
                <RefreshCw className="size-3.5" /> 刷新
              </Button>
            </div>
          </CardContent>
        </Card>

        {/* 任务列表 */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm">
              文章处理状态
              {filter && <Badge variant="secondary" className="ml-2">{filter}</Badge>}
              <span className="ml-2 text-xs font-normal text-muted-foreground">最近 50 篇</span>
            </CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-0.5">
            {items.length === 0 && (
              <p className="text-sm text-muted-foreground text-center py-4">没有文章</p>
            )}
            {items.map((it) => (
              <div key={it.id} className="flex flex-col">
                <div
                  className="flex items-center gap-2 px-1 py-1.5 rounded hover:bg-accent/50 cursor-pointer"
                  title="点击查看管线运行日志"
                  onClick={() => openLogs(it.id)}
                >
                  <Badge variant={statusVariant[it.status]} className="shrink-0">{it.status}</Badge>
                  <span className="text-sm truncate flex-1">{it.title || "(无标题)"}</span>
                  <span className="text-xs text-muted-foreground shrink-0">{it.source || "-"}</span>
                  <span className="text-xs text-muted-foreground shrink-0">
                    {it.published_at ? new Date(it.published_at).toLocaleDateString() : ""}
                  </span>
                  {it.status === "failed" && (
                    <Button size="sm" variant="ghost" className="h-6 px-2 shrink-0"
                      title="退回待处理并重跑单篇"
                      onClick={(e) => { e.stopPropagation(); api.retryItem(it.id).then(() => { loadStatus(); loadItems() }).catch((e2) => setError((e2 as Error).message)) }}>
                      <RotateCcw className="size-3" /> 重试
                    </Button>
                  )}
                </div>
                {openId === it.id && (
                  <div className="border rounded-md mx-1 mb-2 p-2 flex flex-col gap-1 bg-muted/20">
                    <div className="flex items-center gap-2">
                      <span className="text-xs font-medium">管线运行日志</span>
                      <Button size="sm" variant="ghost" className="h-5 px-1.5 text-xs"
                        onClick={() => api.getPipelineLogs(it.id).then(setLogs).catch(() => {})}>
                        <RefreshCw className="size-3" /> 刷新
                      </Button>
                      <Button size="sm" variant="ghost" className="h-5 px-1.5 text-xs"
                        onClick={() => api.retryItem(it.id).then(() => { loadStatus(); loadItems(); return api.getPipelineLogs(it.id) }).then(setLogs).catch((e2) => setError((e2 as Error).message))}>
                        <RotateCcw className="size-3" /> 重跑本篇
                      </Button>
                    </div>
                    {logs.length === 0 ? (
                      <p className="text-xs text-muted-foreground px-1">
                        还没有管线日志 — 该篇未跑过新版管线;点「重跑本篇」生成日志与摘要/切块/实体
                      </p>
                    ) : (
                      logs.map((l) => (
                        <div key={l.id} className="flex items-start gap-2 px-1 text-xs">
                          <span className="text-muted-foreground shrink-0 w-32">
                            {new Date(l.created_at).toLocaleString()}
                          </span>
                          <Badge variant="outline" className="font-mono shrink-0">{l.step}</Badge>
                          <Badge variant={l.status === "failed" ? "destructive" : l.status === "skipped" ? "secondary" : "default"} className="shrink-0">
                            {l.status}
                          </Badge>
                          <span className="text-muted-foreground shrink-0">{l.duration_ms}ms</span>
                          <span className="flex-1 whitespace-pre-wrap">{l.message}</span>
                        </div>
                      ))
                    )}
                  </div>
                )}
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
