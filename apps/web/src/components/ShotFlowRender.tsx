import { useEffect, useState } from "react"
import { Check, Cloud, Cpu, Pause, Play } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Input } from "@/components/ui/input"

// ShotFlow · 渲染·发布: 渲染队列 / 预览 / 节点 / 导出设置。
export function ShotFlowRender() {
  const [progress, setProgress] = useState(64)
  const [playing, setPlaying] = useState(false)
  const [platforms, setPlatforms] = useState<Record<string, boolean>>({ douyin: true, shipinhao: true, bili: true })
  const [title, setTitle] = useState("GPT-5.1 深夜发布！多模态推理全面屠榜 #AI #GPT5 #科技快讯")

  useEffect(() => {
    if (!playing) return undefined
    const t = setInterval(() => {
      setProgress((p) => (p >= 100 ? 64 : p + 1))
    }, 1500)
    return () => clearInterval(t)
  }, [playing])

  const jobs = [
    { name: "EP12 · 每日 AI 快讯 <MainVideo/>", state: "渲染中", desc: `${progress}% · ${Math.round(progress * 15.9)} / 1590f · 剩余 ~52s · 完成后自动发布`, active: true },
    { name: "EP12 · 封面 ×3 尺寸 <CoverStill/>", state: "排队中", desc: "抖音 9:16 · 视频号 9:16 · B站 16:9" },
    { name: "EP11 · 每日 AI 快讯 <MainVideo/>", state: "已完成", desc: "昨天 18:28 已发布三平台 · 41 MB · 看数据" },
    { name: "技术精讲 EP07 <Explainer/>", state: "失败", desc: "缺少基准图表素材 · 重试" },
  ]

  return (
    <div className="flex-1 flex overflow-hidden">
      {/* 左: 队列 + 节点 */}
      <div className="w-96 shrink-0 border-r overflow-y-auto p-3 flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <h2 className="font-semibold">渲染队列 · 4</h2>
          <Button size="sm" variant="ghost" className="text-xs">清除已完成</Button>
        </div>
        {jobs.map((j) => (
          <Card key={j.name} className={j.active ? "border-violet-500/60" : ""}>
            <CardContent className="p-3 flex flex-col gap-2">
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-medium truncate">{j.name}</span>
                <Badge variant={j.state === "失败" ? "destructive" : j.state === "已完成" ? "secondary" : "secondary"}
                  className={j.state === "渲染中" ? "text-violet-300 border-violet-500/30" : j.state === "已完成" ? "text-emerald-400 border-emerald-500/30" : ""}>
                  {j.state}
                </Badge>
              </div>
              {j.active && <div className="h-1 rounded bg-secondary overflow-hidden"><div className="h-full bg-violet-500" style={{ width: `${progress}%` }} /></div>}
              <div className="text-xs text-muted-foreground">{j.desc}</div>
            </CardContent>
          </Card>
        ))}

        <div className="text-sm font-medium mt-2">本周发布日历 <span className="text-xs text-muted-foreground ml-2">6月 · 第2周</span></div>
        <div className="grid grid-cols-7 gap-1">
          {["一", "二", "三", "四", "五", "六", "日"].map((d, i) => (
            <div key={d} className={"rounded border text-center py-1.5 text-xs " + (i === 4 ? "border-violet-500 text-violet-300" : "border-border/60 text-muted-foreground")}>
              <div>{d}</div>
              <div className="font-mono">{[2, 2, 2, 2, 1, 1, 0][i]}</div>
            </div>
          ))}
        </div>

        <div className="text-sm font-medium mt-2">渲染节点</div>
        {[
          { name: "本机 · RTX 4070", desc: "12 线程 · 忙碌", busy: true },
          { name: "云端节点 A", desc: "16 线程 · 空闲", busy: false },
        ].map((n) => (
          <Card key={n.name}>
            <CardContent className="p-3 flex items-center gap-2">
              {n.busy ? <Cpu className="size-4 text-violet-300" /> : <Cloud className="size-4 text-emerald-400" />}
              <div className="flex-1">
                <div className="text-sm">{n.name}</div>
                <div className="text-xs text-muted-foreground">{n.desc}</div>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* 中: 预览 */}
      <div className="flex-1 flex flex-col items-center justify-center gap-4 p-6 overflow-y-auto">
        <div className="flex items-center gap-2 text-sm">
          <span className="font-medium">EP12 · 每日 AI 快讯</span>
          <Badge variant="secondary" className="text-violet-300 border-violet-500/30">渲染中 {progress}%</Badge>
        </div>
        <div className="w-64 rounded-2xl border border-violet-500/40 bg-gradient-to-b from-violet-950/40 to-background p-4 flex flex-col gap-2">
          <div className="text-[10px] text-muted-foreground flex justify-between"><span>AI 前线观察</span><span>01 / 03</span></div>
          <div className="font-bold">GPT-5.1 深夜发布</div>
          <div className="text-xs text-violet-300">多模态推理全面屠榜</div>
          <div className="rounded-lg bg-gradient-to-br from-violet-600/40 to-emerald-500/30 h-56 flex items-center justify-center text-xs">画面截图</div>
          <div className="text-xs space-y-1">
            <div className="flex items-center gap-1.5 text-emerald-400"><Check className="size-3" /> MMMU 基准刷新纪录</div>
            <div className="flex items-center gap-1.5 text-emerald-400"><Check className="size-3" /> API 价格下降 30%</div>
          </div>
          <div className="text-[10px] text-muted-foreground border-t pt-1">OpenAI 这次没有预热…</div>
          <div className="h-1 rounded bg-violet-500" style={{ width: `${progress}%` }} />
        </div>
        <div className="w-64">
          <div className="h-1 rounded bg-secondary overflow-hidden"><div className="h-full bg-violet-500" style={{ width: `${progress}%` }} /></div>
          <div className="flex items-center justify-center gap-4 mt-3 text-xs text-muted-foreground">
            <span>00:33.9</span>
            <Button variant="secondary" size="icon" className="rounded-full size-9 bg-violet-600 hover:bg-violet-500 text-white"
              onClick={() => setPlaying(!playing)}>
              {playing ? <Pause className="size-4" /> : <Play className="size-4" />}
            </Button>
            <span>00:53.0 · 1590f</span>
          </div>
        </div>
      </div>

      {/* 右: 渲染设置 + 发布设置 */}
      <aside className="w-80 shrink-0 border-l overflow-y-auto p-4 flex flex-col gap-4">
        <section className="flex flex-col gap-2">
          <h3 className="font-semibold">渲染设置</h3>
          <label className="text-xs text-muted-foreground">分辨率</label>
          <select className="border rounded-md h-9 px-2 text-sm bg-transparent"><option>1080 × 1920 (9:16)</option></select>
          <label className="text-xs text-muted-foreground">帧率 / 编码</label>
          <select className="border rounded-md h-9 px-2 text-sm bg-transparent"><option>30 fps · H.264 MP4</option></select>
        </section>
        <section className="flex flex-col gap-2">
          <h3 className="font-semibold">发布设置</h3>
          {[
            { key: "douyin", label: "抖音", ratio: "9:16" },
            { key: "shipinhao", label: "视频号", ratio: "9:16" },
            { key: "bili", label: "哔哩哔哩", ratio: "自动转码 16:9" },
          ].map((p) => (
            <label key={p.key} className="flex items-center gap-2 border rounded-md px-2.5 py-2 text-sm cursor-pointer">
              <input type="checkbox" checked={!!platforms[p.key]}
                onChange={(e) => setPlatforms({ ...platforms, [p.key]: e.target.checked })} />
              <span className="flex-1">{p.label}</span>
              <span className="text-xs text-amber-400">{p.ratio}</span>
            </label>
          ))}
          <label className="text-xs text-muted-foreground">发布时间</label>
          <select className="border rounded-md h-9 px-2 text-sm bg-transparent"><option>今天 18:30 · 渲染完成后</option></select>
          <label className="text-xs text-muted-foreground">标题 / 话题</label>
          <Input value={title} onChange={(e) => setTitle(e.target.value)} />
          <div className="flex items-center gap-2 text-xs text-violet-300 border border-violet-500/30 rounded-md px-2.5 py-2 bg-violet-950/20">
            预计 2m40s · 输出 48 MB · 发布 3 平台
          </div>
          <Button className="bg-violet-600 hover:bg-violet-500 text-white">渲染并发布</Button>
          <Button variant="outline">仅渲染，不发布</Button>
        </section>
      </aside>
    </div>
  )
}
