import { useEffect, useState } from "react"
import { Download, Link2, Plus, RefreshCw, Trash2, Upload, X } from "lucide-react"

import { api, type Item } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Input } from "@/components/ui/input"

type Asset = Item & { kind: string; refs: string[] }

// ShotFlow · 素材库: 截图/录屏/图表/Logo/音频/字幕,带来源与引用位置。
export function ShotFlowAssets() {
  const [assets, setAssets] = useState<Asset[]>([])
  const [tab, setTab] = useState("全部")
  const [query, setQuery] = useState("")
  const [current, setCurrent] = useState<Asset | null>(null)

  useEffect(() => {
    api.listItems({ limit: 24 }).then((its) => {
      const next = its.map((it, i) => ({
        ...it,
        kind: i % 5 === 0 ? "图表" : "截图",
        refs: i < 3 ? ["快讯① · 画面槽位"] : [],
      }))
      setAssets(next)
      setCurrent(next[0] ?? null)
    }).catch(() => {})
  }, [])

  const kinds = ["全部", "截图", "录屏", "图表", "Logo", "音频", "字幕"]
  const countOf = (k: string) => (k === "全部" ? assets.length : assets.filter((a) => a.kind === k).length)
  const filtered = assets.filter((a) =>
    (tab === "全部" || a.kind === tab) &&
    (!query || a.title.toLowerCase().includes(query.toLowerCase()) || a.source.toLowerCase().includes(query.toLowerCase()))
  )

  return (
    <div className="flex-1 flex overflow-hidden">
      <div className="flex-1 flex flex-col overflow-hidden">
        <div className="px-4 py-2.5 border-b flex items-center gap-3 flex-wrap">
          <h2 className="font-semibold">素材库</h2>
          <span className="text-xs text-muted-foreground">AI 前线观察 · {assets.length} 个素材 · 18.6 GB</span>
          <Input className="max-w-52 h-8 ml-auto" placeholder="搜索素材 / 标签 / 来源…"
            value={query} onChange={(e) => setQuery(e.target.value)} />
          <Button size="sm" variant="ghost">自动抓取设置</Button>
          <Button size="sm" className="bg-violet-600 hover:bg-violet-500 text-white">
            <Upload className="size-3.5" /> 上传素材
          </Button>
        </div>

        <div className="px-4 py-2 border-b text-xs text-violet-300 bg-violet-950/20 flex items-center gap-2">
          <Link2 className="size-3.5" />
          已从今日选题自动抓取 8 个素材: 网页截图 ×3 · 基准图表 ×2 · 社区评论图 ×3
          <Button size="sm" variant="ghost" className="ml-auto text-xs text-violet-300">查看抓取记录</Button>
        </div>

        <div className="px-4 py-2 border-b flex items-center gap-1.5 flex-wrap">
          {kinds.map((k) => (
            <Button key={k} size="sm" variant={tab === k ? "secondary" : "ghost"}
              onClick={() => setTab(k)}>
              {k} <span className="ml-1 text-xs text-muted-foreground">{countOf(k)}</span>
            </Button>
          ))}
          <span className="ml-auto text-xs text-muted-foreground">最近添加</span>
        </div>

        <div className="flex-1 overflow-y-auto p-3 grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3 content-start">
          {filtered.map((a) => (
            <Card key={a.id} className="cursor-pointer hover:border-violet-500/50" onClick={() => setCurrent(a)}>
              <div className="h-28 rounded-t-lg bg-gradient-to-br from-violet-600/25 to-emerald-600/15 flex items-center justify-center text-xs relative">
                <Badge variant="secondary" className="absolute left-2 top-2">{a.kind}</Badge>
                {a.title.slice(0, 12)}
              </div>
              <CardContent className="p-2.5 flex flex-col gap-1">
                <div className="text-sm font-medium truncate">item-{a.id}.png</div>
                <div className="text-xs text-muted-foreground truncate">{a.title}</div>
                <div className="flex items-center gap-1.5 text-xs text-muted-foreground flex-wrap">
                  <span>{a.source}</span>
                  {a.published_at && <span>{new Date(a.published_at).toLocaleDateString()}</span>}
                </div>
                <div className="text-xs text-violet-300 truncate">
                  {a.refs.length > 0 ? a.refs.join("、") : "未引用"}
                </div>
              </CardContent>
            </Card>
          ))}
          {filtered.length === 0 && (
            <p className="text-sm text-muted-foreground col-span-full text-center mt-10">没有匹配的素材</p>
          )}
        </div>
      </div>

      {/* 素材详情 */}
      {current && (
        <aside className="w-80 shrink-0 border-l overflow-y-auto p-4 flex flex-col gap-3 relative">
          <Button variant="ghost" size="icon" className="absolute right-2 top-2 size-7" onClick={() => setCurrent(null)}>
            <X className="size-4" />
          </Button>
          <h3 className="font-semibold">素材详情</h3>
          <div className="rounded-lg bg-gradient-to-br from-violet-600/30 to-emerald-600/20 h-40 flex items-center justify-center text-sm">
            item-{current.id}.png
          </div>
          <div className="flex items-center justify-between">
            <span className="font-mono text-sm">item-{current.id}.png</span>
            <Badge variant="secondary">{current.kind}</Badge>
          </div>
          <dl className="text-sm flex flex-col gap-1.5">
            <div className="flex justify-between"><dt className="text-muted-foreground">来源</dt><dd>{current.source}</dd></div>
            <div className="flex justify-between"><dt className="text-muted-foreground">抓取时间</dt><dd>今天 08:02 · 自动</dd></div>
            <div className="flex justify-between"><dt className="text-muted-foreground">尺寸</dt><dd>2400 × 1350 · 1.8 MB</dd></div>
            <div className="flex justify-between"><dt className="text-muted-foreground">关联选题</dt><dd>#{current.id} {current.title.slice(0, 14)}</dd></div>
          </dl>
          <div className="flex gap-1.5 flex-wrap">
            <Badge variant="secondary">#GPT-5.1</Badge>
            <Badge variant="secondary">#官方博客</Badge>
            <Badge variant="secondary">#今日抓取</Badge>
          </div>
          <div className="text-sm">
            <div className="text-muted-foreground mb-1.5">引用位置 · {current.refs.length}</div>
            {current.refs.length > 0 ? current.refs.map((r) => (
              <Card key={r} className="mb-2"><CardContent className="p-2.5 text-sm flex items-center gap-2">
                <Link2 className="size-3.5 text-muted-foreground" /> {r}
                <span className="text-xs text-muted-foreground block">EP12 每日 AI 快讯</span>
              </CardContent></Card>
            )) : (
              <Card><CardContent className="p-2.5 text-sm text-muted-foreground">封面 · 背景图(EP12 三平台封面)</CardContent></Card>
            )}
          </div>
          <Button size="sm" className="bg-violet-600 hover:bg-violet-500 text-white">
            <Plus className="size-3.5" /> 插入当前槽位 · 快讯①
          </Button>
          <div className="flex justify-between text-sm">
            <Button size="sm" variant="ghost"><RefreshCw className="size-3.5" /> 重新抓取</Button>
            <Button size="sm" variant="ghost" className="text-destructive"><Trash2 className="size-3.5" /> 删除</Button>
          </div>
          <Button size="sm" variant="outline"><Download className="size-3.5" /> 下载原始素材</Button>
        </aside>
      )}
    </div>
  )
}
