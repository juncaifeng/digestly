import { useState } from "react"
import {
  Activity, Database, FolderOpen, LayoutTemplate, ListVideo, Zap,
} from "lucide-react"
import { ShotFlowAnalytics } from "@/components/ShotFlowAnalytics"
import { ShotFlowAssembly } from "@/components/ShotFlowAssembly"
import { ShotFlowAssets } from "@/components/ShotFlowAssets"
import { ShotFlowRender } from "@/components/ShotFlowRender"
import { ShotFlowTemplates, type ShotFlowTemplate } from "@/components/ShotFlowTemplates"
import { ShotFlowTopics } from "@/components/ShotFlowTopics"
import { Button } from "@/components/ui/button"

type View = "topics" | "templates" | "assembly" | "assets" | "render" | "analytics"

const navItems: { key: View; label: string; icon: React.ReactNode }[] = [
  { key: "topics", label: "选题 · 信息源", icon: <ListVideo className="size-4" /> },
  { key: "templates", label: "模板库", icon: <LayoutTemplate className="size-4" /> },
  { key: "assets", label: "素材库", icon: <FolderOpen className="size-4" /> },
  { key: "render", label: "渲染 · 发布", icon: <Zap className="size-4" /> },
  { key: "analytics", label: "数据复盘", icon: <Activity className="size-4" /> },
]

// ShotFlow 主壳: 选题 → 模板库 → 装配 → 素材 → 渲染发布 → 复盘;
// digestly 仅作为只读内容来源,订阅管理留在独立产品内。
export default function App() {
  const [view, setView] = useState<View>("topics")
  const [topicId, setTopicId] = useState<number | undefined>()
  const [template, setTemplate] = useState<ShotFlowTemplate | null>(null)

  const openTemplates = (id: number) => {
    setTopicId(id)
    setView("templates")
  }

  const openAssembly = (selected: ShotFlowTemplate) => {
    setTemplate(selected)
    setView("assembly")
  }

  return (
    <div className="dark shotflow-shell flex h-screen bg-background text-foreground">
      <aside className="shotflow-sidebar w-64 shrink-0 border-r flex flex-col p-4 gap-3">
        <div className="flex items-center gap-2 text-violet-400">
          <Zap className="size-5" />
          <h1 className="text-lg font-bold">ShotFlow</h1>
        </div>

        <div className="border rounded-md px-2.5 py-2 text-sm flex items-center gap-2">
          <span className="text-xs text-muted-foreground">运营账号</span>
          <select className="flex-1 bg-transparent text-sm outline-none">
            <option>AI 前线观察</option>
          </select>
        </div>

        <nav className="flex flex-col gap-0.5">
          {navItems.map((n) => (
            <Button
              key={n.key}
              variant={view === n.key || (n.key === "templates" && view === "assembly") ? "secondary" : "ghost"}
              size="sm"
              className="justify-start"
              onClick={() => setView(n.key)}
            >
              {n.icon} {n.label}
            </Button>
          ))}
        </nav>

        <div className="border-t pt-3 flex flex-col gap-1 text-xs">
          <div className="text-muted-foreground mb-1">栏目</div>
          <div className="flex justify-between"><span className="text-violet-300">每日 AI 快讯</span><span className="text-muted-foreground">日更</span></div>
          <div className="flex justify-between"><span>技术精讲</span><span className="text-muted-foreground">周更</span></div>
          <div className="flex justify-between"><span>干货清单</span><span className="text-muted-foreground">不定期</span></div>
        </div>

        <div className="mt-auto border-t pt-3 flex items-center gap-2">
          <span className="size-7 rounded-full bg-violet-600/30 text-violet-300 flex items-center justify-center text-xs">林</span>
          <div className="flex-1 min-w-0">
            <div className="text-sm truncate">林小河</div>
            <div className="text-xs text-muted-foreground">SRE · 平台组</div>
          </div>
        </div>
        <div className="flex items-center gap-1 text-xs text-muted-foreground" title="ShotFlow 只读取 digestly 已采集的文章，不包含订阅管理">
          <Database className="size-3" /> 内容来源 · digestly（只读）
        </div>
      </aside>

      {view === "topics" ? (
        <ShotFlowTopics onAssemble={openTemplates} />
      ) : view === "templates" ? (
        <ShotFlowTemplates topicId={topicId} onAssemble={openAssembly} />
      ) : view === "assembly" && template ? (
        <ShotFlowAssembly topicId={topicId} template={template} onBack={() => setView("templates")} />
      ) : view === "assets" ? (
        <ShotFlowAssets />
      ) : view === "render" ? (
        <ShotFlowRender />
      ) : (
        <ShotFlowAnalytics />
      )}
    </div>
  )
}
