import { useEffect, useState } from "react"
import { Check, ChevronRight, Flame, Info, Plus } from "lucide-react"

import { api, type Item } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"

// ShotFlow · 选题·信息源: 今日抓取聚合 + AI 聚类选题卡 + EP 阶段流。
export function ShotFlowTopics({ onAssemble }: { onAssemble: (id: number) => void }) {
  const [items, setItems] = useState<Item[]>([])
  const [hidden, setHidden] = useState<Set<number>>(new Set())

  useEffect(() => {
    api.listItems({ limit: 5 }).then(setItems).catch(() => {})
  }, [])

  const cards = items.filter((it) => !hidden.has(it.id))
  const stage = [
    { name: "选题", desc: "3 条入选 · 08:10", active: true },
    { name: "文案", desc: "AI 初稿已生成 · 待审核" },
    { name: "装配", desc: "模板 NewsFlash v3" },
    { name: "渲染", desc: "预计 2m40s" },
    { name: "发布", desc: "今天 18:30 · 三平台" },
  ]

  return (
    <div className="flex-1 flex overflow-hidden">
      <div className="flex-1 overflow-y-auto p-4 flex flex-col gap-3">
        <div className="flex items-center gap-3 flex-wrap">
          <span className="text-sm text-muted-foreground">
            今日抓取 150 条 · AI 聚类为 5 个选题 · 08:00 已更新
          </span>
          <div className="ml-auto flex gap-2">
            <Button size="sm" variant="ghost">忽略低分</Button>
            <Button size="sm" className="bg-violet-600 hover:bg-violet-500 text-white">
              <Plus className="size-3.5" /> 手动新建选题
            </Button>
          </div>
        </div>

        {cards.map((it, i) => (
          <Card key={it.id} className="border-border/60 hover:border-violet-500/50 transition-colors">
            <CardContent className="p-4 flex gap-3">
              <div className="flex flex-col items-center gap-1 pt-1 text-muted-foreground">
                <Flame className={"size-4 " + (i === 0 ? "text-orange-400" : "text-muted-foreground/60")} />
                <span className="text-xs font-mono">#{i + 1}</span>
              </div>
              <div className="flex-1 min-w-0 flex flex-col gap-2">
                <div className="flex items-start gap-2 flex-wrap">
                  <h3 className="font-semibold leading-snug flex-1 min-w-0">{it.title}</h3>
                  {i === 0 ? (
                    <Badge className="bg-emerald-600/20 text-emerald-400 border-emerald-600/30 shrink-0">
                      已立项 → EP12
                    </Badge>
                  ) : i === cards.length - 1 ? (
                    <Badge variant="secondary" className="shrink-0">忽略</Badge>
                  ) : (
                    <Badge variant="secondary" className="text-violet-300 border-violet-500/30 shrink-0">立项成期</Badge>
                  )}
                  {i === 0 && (
                    <Button size="sm" className="bg-emerald-600 hover:bg-emerald-500 text-white shrink-0"
                      onClick={() => onAssemble(it.id)}>
                      去装配 <ChevronRight className="size-3.5" />
                    </Button>
                  )}
                </div>
                <p className="text-sm text-muted-foreground line-clamp-2">
                  {it.link}
                </p>
                <div className="flex items-center gap-3 text-xs text-muted-foreground flex-wrap">
                  <span>{it.source}</span>
                  {it.published_at && <span>{new Date(it.published_at).toLocaleString()}</span>}
                  <span>#{it.id}</span>
                </div>
              </div>
              <div className="flex flex-col gap-1 shrink-0">
                {i !== 0 && i !== cards.length - 1 && (
                  <Button size="sm" variant="outline" onClick={() => onAssemble(it.id)}>去装配</Button>
                )}
                {i === cards.length - 1 && (
                  <Button size="sm" variant="ghost" onClick={() => setHidden(new Set(hidden).add(it.id))}>忽略</Button>
                )}
                <Button size="sm" variant="ghost" className="text-muted-foreground">存入选题池</Button>
              </div>
            </CardContent>
          </Card>
        ))}
        {cards.length === 0 && (
          <p className="text-sm text-muted-foreground text-center mt-10">
            今日抓取已完成,暂无新选题 — 去阅读页看看原始文章
          </p>
        )}
      </div>

      {/* 右栏: EP 阶段流 + 技术拆解 + AI 运营建议 */}
      <aside className="w-80 shrink-0 border-l overflow-y-auto p-4 flex flex-col gap-4">
        <Card>
          <CardContent className="p-4 flex flex-col gap-3">
            <div className="flex items-center justify-between">
              <h3 className="font-semibold">每日 AI 快讯 · EP12</h3>
              <Badge variant="secondary" className="text-violet-300 border-violet-500/30">进行中</Badge>
            </div>
            {stage.map((s) => (
              <div key={s.name} className="flex items-start gap-2 text-sm">
                {s.active ? (
                  <Check className="size-4 text-emerald-400 mt-0.5" />
                ) : (
                  <span className="size-4 rounded-full border border-muted-foreground/40 mt-0.5" />
                )}
                <div className="flex-1">
                  <div className="font-medium">{s.name}</div>
                  <div className="text-xs text-muted-foreground">{s.desc}</div>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-4 flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <h3 className="font-semibold">GPT-5.1 技术拆解</h3>
              <Badge variant="secondary">周六</Badge>
            </div>
            <p className="text-sm text-muted-foreground">
              问题引入 → 架构图解 → 基准实测 → 上手建议
            </p>
            <div className="flex items-center gap-1.5 text-xs text-amber-400">
              <Info className="size-3.5" /> 已攒 6 个素材 · 缺基准图表
            </div>
          </CardContent>
        </Card>

        <Card className="border-violet-500/30 bg-violet-950/20">
          <CardContent className="p-4 flex flex-col gap-2">
            <h3 className="font-semibold text-violet-200">AI 运营建议</h3>
            <p className="text-sm text-violet-200/70 leading-relaxed">
              GPT-5.1 当前热度 #1,建议将「GPT-5.1 深夜发布」保留为快讯①;
              Claude API 与 Qwen3 相关度同档,可做对照卡。
              英伟达市值与账号定位相关度低,建议忽略或并入快讯尾条。
            </p>
          </CardContent>
        </Card>
      </aside>
    </div>
  )
}
