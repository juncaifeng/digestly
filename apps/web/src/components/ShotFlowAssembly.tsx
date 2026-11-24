import { useEffect, useMemo, useState } from "react"
import { ArrowLeft, Check, Pencil, Play, Sparkles } from "lucide-react"

import { api, type Item } from "@/api/client"
import type { ShotFlowTemplate } from "@/components/ShotFlowTemplates"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Input } from "@/components/ui/input"

type Slot = { key: string; label: string; secs: number; state: "done" | "editing" | "todo"; item?: Item }
type Draft = { copy: string; title: string; points: string }

const draftFor = (item: Item): Draft => ({
  title: item.title,
  points: "多模态推理屠榜 · API 降价 30%",
  copy: `昨夜 OpenAI 毫无预热地上线了 ${item.title.split("，")[0]}：多模态推理基准全面刷新纪录，视频理解首次超过人类专家水平，而 API 价格反而下降三成。评论区已经有人用它一夜生成整部短剧。`,
})

// ShotFlow · 模板装配: 槽位 → 文案 → 画面 → 配音 → 预览。
export function ShotFlowAssembly({ topicId, template, onBack }: {
  topicId?: number
  template: ShotFlowTemplate
  onBack: () => void
}) {
  const [items, setItems] = useState<Item[]>([])
  const [slots, setSlots] = useState<Slot[]>([])
  const [current, setCurrent] = useState("s1")
  const [reviewed, setReviewed] = useState(false)

  useEffect(() => {
    Promise.all([
      api.listItems({ limit: 3 }),
      topicId ? api.getItem(topicId).catch(() => undefined) : Promise.resolve(undefined),
    ]).then(([its, selected]) => {
      const picked = selected ?? its[0]
      const rest = its.filter((it) => it.id !== picked?.id)
      const contentItems = picked ? [picked, ...rest] : its
      let itemIndex = 0
      let editingAssigned = false
      let todoAssigned = false
      const nextSlots = template.slotPlan.map((plan, index): Slot => {
        const slotItem = plan.kind === "item" ? contentItems[itemIndex++] : undefined
        let state: Slot["state"] = "done"
        if (plan.kind === "item" && !editingAssigned) {
          state = "editing"
          editingAssigned = true
        } else if (plan.kind === "outro" && !todoAssigned) {
          state = "todo"
          todoAssigned = true
        }
        return {
          key: `slot-${index}`,
          label: `${String(index).padStart(2, "0")} ${plan.label}`,
          secs: plan.secs,
          state,
          item: slotItem,
        }
      })
      setItems(contentItems)
      setSlots(nextSlots)
      setCurrent(nextSlots.find((slot) => slot.state === "editing")?.key ?? nextSlots[0]?.key ?? "")
      setReviewed(false)
    }).catch(() => {})
  }, [topicId, template])

  const slot = slots.find((s) => s.key === current)
  const item = slot?.item ?? items[0]
  const [drafts, setDrafts] = useState<Record<number, Draft>>({})
  const draft: Draft = item ? drafts[item.id] ?? draftFor(item) : { copy: "", title: "", points: "" }
  const updateDraft = (patch: Partial<Draft>) => {
    if (!item) return
    setDrafts((all) => ({
      ...all,
      [item.id]: { ...(all[item.id] ?? draftFor(item)), ...patch },
    }))
  }

  const totalSecs = useMemo(() => slots.reduce((n, s) => n + s.secs, 0), [slots])

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      <div className="flex items-center gap-3 px-4 py-2.5 border-b flex-wrap">
        <Button size="sm" variant="ghost" onClick={onBack}>
          <ArrowLeft className="size-3.5" /> 模板库
        </Button>
        <h2 className="font-semibold">每日 AI 快讯 · EP12</h2>
        <Badge variant="secondary" className="text-violet-300 border-violet-500/30">装配中</Badge>
        <span className="text-xs text-muted-foreground">
          {template.name} {template.version} · {slots.length} 个槽位 · {totalSecs.toFixed(1)}s · 文案审校 2 / {slots.length}
        </span>
        <div className="ml-auto flex gap-2">
          <Button size="sm" variant="outline"><Sparkles className="size-3.5" /> AI 重写本段</Button>
          <Button size="sm" className="bg-emerald-600 hover:bg-emerald-500 text-white" onClick={() => setReviewed(true)}>
            <Check className="size-3.5" /> {reviewed ? "已审" : "标记已审"}
          </Button>
        </div>
      </div>

      <div className="flex-1 flex overflow-hidden">
        {/* 左: 本期结构槽位 */}
        <aside className="w-64 shrink-0 border-r overflow-y-auto p-3 flex flex-col gap-2">
          <div className="border rounded-md p-2.5">
            <div className="text-sm font-medium">{template.name} {template.version}</div>
            <div className="text-xs text-violet-300 mt-1 font-mono">&lt;{template.composition}&gt;</div>
            <div className="text-xs text-muted-foreground mt-1">{template.structure}</div>
          </div>
          <div className="text-xs text-muted-foreground text-right">调整顺序</div>
          {slots.map((s) => (
            <button
              key={s.key}
              onClick={() => setCurrent(s.key)}
              className={"flex items-center gap-2 border rounded-md px-2.5 py-2 text-left text-sm " +
                (current === s.key ? "border-violet-500 bg-violet-950/30" : "border-border/60 hover:bg-accent/50")}
            >
              <span className="text-xs text-muted-foreground font-mono">{s.label.split(" ")[0]}</span>
              <span className="flex-1 truncate">{s.label.split(" ").slice(1).join(" ")}</span>
              <span className="text-xs text-muted-foreground">{s.secs}s</span>
              {s.state === "done" ? (
                <span className="size-3.5 rounded-full bg-emerald-500/20 text-emerald-400 text-[10px] flex items-center justify-center">✓</span>
              ) : s.state === "editing" ? (
                <Pencil className="size-3 text-violet-300" />
              ) : (
                <span className="size-3.5 rounded-full border border-muted-foreground/40" />
              )}
            </button>
          ))}
          <Button size="sm" variant="ghost" className="justify-start text-muted-foreground">+ 添加快讯槽位</Button>
          <Card className="mt-auto border-violet-500/30 bg-violet-950/20">
            <CardContent className="p-3">
              <div className="flex items-center gap-1.5 text-xs text-violet-200">
                <Sparkles className="size-3.5" /> AI 自动装配
              </div>
              <p className="text-xs text-violet-200/60 mt-1">审校完成后，自动生成画面、字幕与配音</p>
            </CardContent>
          </Card>
        </aside>

        {/* 中: 当前段编辑 */}
        <div className="flex-1 min-w-0 overflow-y-auto p-4 flex flex-col gap-4">
          {slot && (
            <>
              <div className="flex items-center gap-2 flex-wrap">
                <h3 className="font-semibold">{slot.label.replace(/^\d+\s*/, "")}</h3>
                <Badge variant="secondary" className="text-amber-400 border-amber-500/30">文案待审</Badge>
                <span className="ml-auto text-xs text-muted-foreground">
                  {topicId ? `选题 #${topicId}` : "未关联选题"} · {item?.source ?? "OpenAI Blog"}
                </span>
              </div>

              <Card>
                <CardContent className="p-4 flex flex-col gap-2">
                  <div className="flex items-center gap-2 text-sm">
                    <span className="font-medium">口播文案</span>
                    <Badge variant="secondary" className="text-violet-300 border-violet-500/30">AI 初稿</Badge>
                    <Button size="sm" variant="ghost" className="ml-auto text-xs">TTS 试听</Button>
                  </div>
                  <textarea
                    className="w-full min-h-24 rounded-md border bg-transparent p-3 text-sm leading-relaxed"
                    value={draft.copy}
                    onChange={(e) => updateDraft({ copy: e.target.value })}
                  />
                  <div className="flex items-center gap-3 text-xs text-muted-foreground">
                    <span>{draft.copy.length} 字 · 预计 {(draft.copy.length / 6).toFixed(1)}s · 语速 1.1x</span>
                    <Button size="sm" variant="ghost" className="ml-auto text-xs">编辑</Button>
                    <Button size="sm" variant="ghost" className="text-xs">重新生成</Button>
                  </div>
                </CardContent>
              </Card>

              <Card className="min-w-0">
                <CardContent className="min-w-0 p-4 flex flex-col gap-3">
                  <div className="text-sm font-medium">画面内容 · 模板槽位</div>
                  <div className="min-w-0 flex gap-3">
                    <div className="w-44 shrink-0 rounded-md bg-gradient-to-br from-violet-600/30 to-emerald-600/20 border border-border/60 flex items-center justify-center text-xs text-center p-2">
                      自动截图 · 08:02
                    </div>
                    <div className="flex-1 min-w-0 flex flex-col gap-2">
                      <label className="text-xs text-muted-foreground">画面标题</label>
                      <Input value={draft.title} onChange={(e) => updateDraft({ title: e.target.value })} />
                      <label className="text-xs text-muted-foreground">核心要点</label>
                      <Input value={draft.points} onChange={(e) => updateDraft({ points: e.target.value })} />
                      <div className="min-w-0 flex items-center gap-2 text-xs">
                        <a className="min-w-0 flex-1 truncate text-violet-300" href={item?.link} target="_blank" rel="noreferrer" title={item?.link}>
                          {item?.link}
                        </a>
                        <Button size="sm" variant="ghost" className="ml-auto text-xs shrink-0">替换截图</Button>
                      </div>
                    </div>
                  </div>
                  <div className="flex gap-2 flex-wrap text-xs">
                    <Badge variant="secondary">热度 #1</Badge>
                    <Badge variant="secondary">发布 2h 前</Badge>
                    <Badge variant="secondary">基准图表 ×2</Badge>
                    <Badge variant="secondary">社区评论 ×3</Badge>
                  </div>
                </CardContent>
              </Card>
            </>
          )}
        </div>

        {/* 右: 实时预览 + Props + 配音 */}
        <aside className="w-72 shrink-0 border-l overflow-y-auto p-3 flex flex-col gap-3">
          <div className="text-xs text-muted-foreground flex items-center justify-between">
            <span>实时预览</span><span>9:16</span>
          </div>
          <div className="rounded-xl border border-violet-500/40 bg-gradient-to-b from-violet-950/40 to-background p-3 flex flex-col gap-2">
            <div className="text-[10px] text-muted-foreground">AI 前线观察</div>
            <div className="font-bold text-sm leading-snug">{draft.title}</div>
            <div className="text-xs text-violet-300">多模态推理全面屠榜</div>
            <div className="rounded-md bg-gradient-to-br from-violet-600/40 to-emerald-500/30 h-32 flex items-center justify-center text-xs text-center p-2">
              画面截图
            </div>
            <div className="text-xs space-y-1">
              <div className="flex items-center gap-1.5 text-emerald-400"><Check className="size-3" /> MMMU 基准刷新纪录</div>
              <div className="flex items-center gap-1.5 text-emerald-400"><Check className="size-3" /> API 价格下降 30%</div>
            </div>
            <div className="text-[10px] text-muted-foreground border-t pt-1">OpenAI 这次没有预热…</div>
            <div className="h-1 rounded bg-violet-500/50" />
          </div>

          <Card>
            <CardContent className="p-3 text-xs flex flex-col gap-1">
              <div className="flex justify-between font-medium"><span>模板 Props</span><Pencil className="size-3" /></div>
              <div className="flex justify-between text-muted-foreground"><span>composition</span><span className="font-mono">&lt;{template.composition}/&gt;</span></div>
              <div className="flex justify-between text-muted-foreground"><span>theme</span><span>电光紫</span></div>
              <div className="flex justify-between text-muted-foreground"><span>font</span><span>思源黑体</span></div>
              <div className="flex justify-between text-muted-foreground"><span>transition</span><span className="font-mono">slide(8f)</span></div>
            </CardContent>
          </Card>

          <Card>
            <CardContent className="p-3 flex flex-col gap-2">
              <div className="flex items-center justify-between text-xs">
                <span>云希 · 男声 · 1.1x</span>
                <Badge className="bg-emerald-600/20 text-emerald-400 border-emerald-600/30">已生成</Badge>
              </div>
              <div className="flex items-end gap-0.5 h-8">
                {Array.from({ length: 40 }, (_, i) => (
                  <span key={i} className="flex-1 rounded-sm bg-violet-400/70"
                    style={{ height: `${20 + Math.abs(Math.sin(i * 1.7)) * 80}%` }} />
                ))}
              </div>
            </CardContent>
          </Card>
        </aside>
      </div>

      {/* 底部时间线 */}
      <div className="border-t px-4 py-3 flex items-center gap-3 flex-wrap">
        <span className="text-xs text-muted-foreground">Sequence · 本期时间线</span>
        <span className="text-xs text-muted-foreground ml-auto">{totalSecs.toFixed(1)}s · {totalSecs * 30}f @ 30fps</span>
        <div className="w-full flex gap-1">
          {slots.map((s) => (
            <button key={s.key} onClick={() => setCurrent(s.key)}
              className={"flex-1 h-7 rounded text-xs " +
                (current === s.key ? "bg-violet-600 text-white" : "bg-secondary text-secondary-foreground")}>
              {s.label.split(" ").slice(1).join(" ")}
            </button>
          ))}
        </div>
        <Button size="sm" className="bg-violet-600 hover:bg-violet-500 text-white w-full">
          <Play className="size-3.5" /> 预览整期 · {totalSecs.toFixed(0)}s
        </Button>
      </div>
    </div>
  )
}
