import { useState } from "react"
import {
  Activity, FolderOpen, LayoutTemplate, ListVideo, Settings as SettingsIcon,
  Store, User, Zap,
} from "lucide-react"

import { ReaderView } from "@/components/ReaderView"
import { SettingsView } from "@/components/SettingsView"
import { ShotFlowAnalytics } from "@/components/ShotFlowAnalytics"
import { ShotFlowAssembly } from "@/components/ShotFlowAssembly"
import { ShotFlowAssets } from "@/components/ShotFlowAssets"
import { ShotFlowRender } from "@/components/ShotFlowRender"
import { ShotFlowTopics } from "@/components/ShotFlowTopics"
import { MarketView } from "@/components/MarketView"
import { Button } from "@/components/ui/button"

type View = "topics" | "assembly" | "assets" | "render" | "analytics" | "reader" | "market" | "settings"

const navItems: { key: View; label: string; icon: React.ReactNode }[] = [
  { key: "topics", label: "选题 · 信息源", icon: <ListVideo className="size-4" /> },
  { key: "assembly", label: "模板装配", icon: <LayoutTemplate className="size-4" /> },
  { key: "assets", label: "素材库", icon: <FolderOpen className="size-4" /> },
  { key: "render", label: "渲染 · 发布", icon: <Zap className="size-4" /> },
  { key: "analytics", label: "数据复盘", icon: <Activity className="size-4" /> },
]

// ShotFlow 主壳: 选题 → 装配 → 素材 → 渲染发布 → 复盘;
// 保留 digestly 阅读器(阅读)与市场/设置作为辅助入口。
export default function App() {
  const [view, setView] = useState<View>("topics")
  const [topicId, setTopicId] = useState<number | undefined>()

  const openAssembly = (id: number) => {
    setTopicId(id)
    setView("assembly")
  }

  return (
    <div className="dark flex h-screen bg-background text-foreground">
      <aside className="w-64 shrink-0 border-r flex flex-col p-4 gap-3">
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
              variant={view === n.key ? "secondary" : "ghost"}
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

        <div className="border-t pt-2 flex flex-col gap-0.5">
          <Button variant={view === "reader" ? "secondary" : "ghost"} size="sm" className="justify-start"
            onClick={() => setView("reader")}>
            <ListVideo className="size-4" /> 阅读
          </Button>
          <Button variant={view === "market" ? "secondary" : "ghost"} size="sm" className="justify-start"
            onClick={() => setView("market")}>
            <Store className="size-4" /> 市场
          </Button>
        </div>

        <div className="mt-auto border-t pt-3 flex items-center gap-2">
          <span className="size-7 rounded-full bg-violet-600/30 text-violet-300 flex items-center justify-center text-xs">林</span>
          <div className="flex-1 min-w-0">
            <div className="text-sm truncate">林小河</div>
            <div className="text-xs text-muted-foreground">SRE · 平台组</div>
          </div>
          <Button variant="ghost" size="icon" className="size-7" title="设置" onClick={() => setView("settings")}>
            <SettingsIcon className="size-4" />
          </Button>
        </div>
        <div className="flex items-center gap-1 text-xs text-muted-foreground">
          <User className="size-3" /> 本地工作区 · 数据不出本机
        </div>
      </aside>

      {view === "topics" ? (
        <ShotFlowTopics onAssemble={openAssembly} />
      ) : view === "assembly" ? (
        <ShotFlowAssembly topicId={topicId} />
      ) : view === "assets" ? (
        <ShotFlowAssets />
      ) : view === "render" ? (
        <ShotFlowRender />
      ) : view === "analytics" ? (
        <ShotFlowAnalytics />
      ) : view === "reader" ? (
        <ReaderView />
      ) : view === "market" ? (
        <MarketView onInstalled={() => {}} />
      ) : (
        <SettingsView />
      )}
    </div>
  )
}
