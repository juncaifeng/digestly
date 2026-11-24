import { useMemo, useState } from "react"
import { ChevronRight, Clock3, Layers3, LayoutGrid, List, Search } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Input } from "@/components/ui/input"

export type ShotFlowTemplate = {
  id: string
  name: string
  version: string
  composition: string
  category: string
  description: string
  structure: string
  slotPlan: { label: string; secs: number; kind: "intro" | "item" | "outro" }[]
  duration: number
  slots: number
  uses: number
  tags: string[]
  accent: string
}

const templates: ShotFlowTemplate[] = [
  {
    id: "newsflash-v3",
    name: "NewsFlash",
    version: "v3",
    composition: "NewsFlash",
    category: "快讯",
    description: "多条新闻快速串联，自动带出标题、核心要点、口播与来源。",
    structure: "开场 → N 条快讯 → 观点 → CTA",
    slotPlan: [
      { label: "Hook 开场", secs: 5, kind: "intro" },
      { label: "快讯① GPT-5.1", secs: 12, kind: "item" },
      { label: "快讯② Claude API", secs: 12, kind: "item" },
      { label: "快讯③ Qwen3", secs: 12, kind: "item" },
      { label: "观点收尾", secs: 8, kind: "outro" },
      { label: "CTA 关注", secs: 4, kind: "outro" },
    ],
    duration: 53,
    slots: 6,
    uses: 12,
    tags: ["9:16", "AI 文案", "自动截图"],
    accent: "from-violet-600/45 via-purple-950/50 to-emerald-600/20",
  },
  {
    id: "dailybrief-v2",
    name: "DailyBrief",
    version: "v2",
    composition: "DailyBrief",
    category: "日报",
    description: "适合每日信息汇总，强化期次、日期和多条摘要的阅读节奏。",
    structure: "日期 → 今日概览 → N 条摘要 → 收尾",
    slotPlan: [
      { label: "日期开场", secs: 5, kind: "intro" },
      { label: "今日概览", secs: 10, kind: "intro" },
      { label: "摘要①", secs: 12, kind: "item" },
      { label: "摘要②", secs: 12, kind: "item" },
      { label: "摘要③", secs: 12, kind: "item" },
      { label: "本周观点", secs: 10, kind: "outro" },
      { label: "CTA 关注", secs: 4, kind: "outro" },
    ],
    duration: 65,
    slots: 7,
    uses: 8,
    tags: ["日更", "摘要", "多来源"],
    accent: "from-blue-600/40 via-slate-950/60 to-cyan-500/20",
  },
  {
    id: "techbrief-v1",
    name: "TechBrief",
    version: "v1",
    composition: "TechBrief",
    category: "技术",
    description: "面向技术发布与版本更新，突出基准数据、代码链接和关键变化。",
    structure: "版本引入 → 核心变化 → 数据对比 → 建议",
    slotPlan: [
      { label: "版本引入", secs: 6, kind: "intro" },
      { label: "核心变化", secs: 16, kind: "item" },
      { label: "数据对比", secs: 18, kind: "item" },
      { label: "实测结果", secs: 14, kind: "item" },
      { label: "上手建议", secs: 12, kind: "outro" },
      { label: "CTA 关注", secs: 6, kind: "outro" },
    ],
    duration: 72,
    slots: 6,
    uses: 6,
    tags: ["基准图表", "代码链接", "技术"],
    accent: "from-emerald-600/35 via-slate-950/60 to-violet-600/20",
  },
  {
    id: "newsflash-compact-v2",
    name: "NewsFlash Compact",
    version: "v2",
    composition: "NewsFlashCompact",
    category: "快讯",
    description: "更紧凑的竖屏快讯版式，减少装饰元素，适合高频短内容。",
    structure: "短开场 → N 条快讯 → 关注",
    slotPlan: [
      { label: "短开场", secs: 4, kind: "intro" },
      { label: "快讯①", secs: 10, kind: "item" },
      { label: "快讯②", secs: 10, kind: "item" },
      { label: "快讯③", secs: 10, kind: "item" },
      { label: "关注", secs: 4, kind: "outro" },
    ],
    duration: 38,
    slots: 5,
    uses: 4,
    tags: ["短视频", "紧凑", "9:16"],
    accent: "from-orange-500/30 via-slate-950/60 to-violet-600/25",
  },
]

const categories = ["全部", "快讯", "日报", "技术"]

export function ShotFlowTemplates({ topicId, onAssemble }: {
  topicId?: number
  onAssemble: (template: ShotFlowTemplate) => void
}) {
  const [query, setQuery] = useState("")
  const [category, setCategory] = useState("全部")
  const [mode, setMode] = useState<"grid" | "list">("grid")

  const filtered = useMemo(() => templates.filter((template) =>
    (category === "全部" || template.category === category) &&
    (!query || `${template.name} ${template.description} ${template.tags.join(" ")}`.toLowerCase().includes(query.toLowerCase()))
  ), [category, query])

  return (
    <main className="flex-1 min-w-0 flex flex-col overflow-hidden">
      <header className="border-b px-5 py-4 flex items-center gap-4 flex-wrap">
        <div>
          <h2 className="text-lg font-semibold">模板库</h2>
          <p className="text-xs text-muted-foreground mt-1">
            先选择模板，再进入内容装配
            {topicId && <span className="text-violet-300"> · 当前选题 #{topicId}</span>}
          </p>
        </div>
        <div className="ml-auto flex items-center gap-2">
          <div className="relative">
            <Search className="absolute left-2.5 top-2.5 size-3.5 text-muted-foreground" />
            <Input className="h-8 w-56 pl-8" placeholder="搜索模板…" value={query}
              onChange={(event) => setQuery(event.target.value)} />
          </div>
          <div className="flex rounded-md border p-0.5">
            <Button size="icon" variant={mode === "grid" ? "secondary" : "ghost"}
              className="size-7" title="卡片视图" onClick={() => setMode("grid")}>
              <LayoutGrid className="size-3.5" />
            </Button>
            <Button size="icon" variant={mode === "list" ? "secondary" : "ghost"}
              className="size-7" title="列表视图" onClick={() => setMode("list")}>
              <List className="size-3.5" />
            </Button>
          </div>
        </div>
      </header>

      <div className="border-b px-5 py-2.5 flex items-center gap-2">
        {categories.map((item) => (
          <Button key={item} size="sm" variant={category === item ? "secondary" : "ghost"}
            onClick={() => setCategory(item)}>
            {item}
            <span className="text-xs text-muted-foreground">
              {item === "全部" ? templates.length : templates.filter((template) => template.category === item).length}
            </span>
          </Button>
        ))}
        <span className="ml-auto text-xs text-muted-foreground">{filtered.length} 个可用模板</span>
      </div>

      <div className="flex-1 overflow-y-auto p-5">
        {filtered.length === 0 ? (
          <div className="h-full flex items-center justify-center text-sm text-muted-foreground">
            没有匹配的模板
          </div>
        ) : mode === "grid" ? (
          <div className="grid grid-cols-1 lg:grid-cols-2 2xl:grid-cols-3 gap-4 content-start">
            {filtered.map((template) => (
              <Card key={template.id} className="min-w-0 overflow-hidden border-border/70 hover:border-violet-500/50 transition-colors">
                <div className={`h-44 bg-gradient-to-br ${template.accent} p-5 flex items-center justify-center relative`}>
                  <Badge variant="secondary" className="absolute left-3 top-3">{template.category}</Badge>
                  <span className="absolute right-3 top-3 text-xs text-muted-foreground">9:16</span>
                  <div className="w-32 rounded-lg border border-white/15 bg-background/75 shadow-2xl p-2.5 backdrop-blur-sm">
                    <div className="h-2 w-12 rounded-full bg-violet-400/70 mb-2" />
                    <div className="h-3 w-20 rounded-sm bg-foreground/75 mb-2" />
                    <div className="h-14 rounded bg-gradient-to-br from-violet-500/35 to-emerald-400/20 mb-2" />
                    <div className="space-y-1">
                      <div className="h-1.5 rounded bg-foreground/25" />
                      <div className="h-1.5 w-3/4 rounded bg-foreground/20" />
                    </div>
                  </div>
                </div>
                <CardContent className="p-4 flex flex-col gap-3">
                  <div>
                    <div className="flex items-center gap-2">
                      <h3 className="font-semibold">{template.name}</h3>
                      <Badge variant="outline" className="text-violet-300">{template.version}</Badge>
                    </div>
                    <p className="text-sm text-muted-foreground mt-1.5 leading-relaxed">{template.description}</p>
                  </div>
                  <div className="flex gap-1.5 flex-wrap">
                    {template.tags.map((tag) => <Badge key={tag} variant="secondary">{tag}</Badge>)}
                  </div>
                  <div className="flex items-center gap-4 text-xs text-muted-foreground border-t pt-3">
                    <span className="flex items-center gap-1"><Clock3 className="size-3.5" />约 {template.duration}s</span>
                    <span className="flex items-center gap-1"><Layers3 className="size-3.5" />{template.slots} 个槽位</span>
                    <span className="ml-auto">{template.uses} 期使用</span>
                  </div>
                  <Button className="w-full bg-violet-600 hover:bg-violet-500 text-white"
                    onClick={() => onAssemble(template)}>
                    使用模板装配 <ChevronRight className="size-4" />
                  </Button>
                </CardContent>
              </Card>
            ))}
          </div>
        ) : (
          <div className="flex flex-col gap-2">
            {filtered.map((template) => (
              <Card key={template.id} className="border-border/70 hover:border-violet-500/50 transition-colors">
                <CardContent className="p-3 flex items-center gap-4">
                  <div className={`w-28 h-20 shrink-0 rounded-md bg-gradient-to-br ${template.accent} border border-white/10 flex items-center justify-center`}>
                    <Layers3 className="size-6 text-white/70" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <h3 className="font-semibold">{template.name}</h3>
                      <Badge variant="outline" className="text-violet-300">{template.version}</Badge>
                      <Badge variant="secondary">{template.category}</Badge>
                    </div>
                    <p className="text-sm text-muted-foreground truncate mt-1">{template.description}</p>
                    <div className="flex items-center gap-4 text-xs text-muted-foreground mt-2">
                      <span>约 {template.duration}s</span>
                      <span>{template.slots} 个槽位</span>
                      <span>{template.uses} 期使用</span>
                    </div>
                  </div>
                  <Button className="shrink-0 bg-violet-600 hover:bg-violet-500 text-white"
                    onClick={() => onAssemble(template)}>
                    使用模板装配 <ChevronRight className="size-4" />
                  </Button>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>
    </main>
  )
}
