import { useEffect, useState } from "react"
import { Activity, CheckCircle2, Clock, Rss, TrendingUp } from "lucide-react"

import { api } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

// ShotFlow · 数据复盘: 采集/阅读指标概览。
export function ShotFlowAnalytics() {
  const [feeds, setFeeds] = useState(0)
  const [unread, setUnread] = useState(0)
  const [stats, setStats] = useState<{ index_items: number; content_cached: number; content_bytes: number } | null>(null)

  useEffect(() => {
    api.listFeeds().then((fs) => {
      setFeeds(fs.length)
      setUnread(fs.reduce((n, f) => n + f.unread_count, 0))
    }).catch(() => {})
    fetch("/api/stats").then((r) => r.json()).then(setStats).catch(() => {})
  }, [])

  const cards = [
    { label: "订阅源", value: feeds, icon: <Rss className="size-4 text-violet-300" /> },
    { label: "未读文章", value: unread, icon: <Clock className="size-4 text-amber-400" /> },
    { label: "索引条目", value: stats?.index_items ?? "…", icon: <Activity className="size-4 text-emerald-400" /> },
    { label: "正文缓存", value: stats ? `${stats.content_cached} 条` : "…", icon: <TrendingUp className="size-4 text-violet-300" /> },
  ]

  return (
    <div className="flex-1 overflow-y-auto p-4 flex flex-col gap-4">
      <div className="flex items-center gap-2">
        <h2 className="font-semibold">数据复盘</h2>
        <Badge variant="secondary">每日 08:00 汇总</Badge>
      </div>
      <div className="grid grid-cols-2 xl:grid-cols-4 gap-3">
        {cards.map((c) => (
          <Card key={c.label}>
            <CardContent className="p-4 flex flex-col gap-2">
              <div className="flex items-center gap-2 text-sm text-muted-foreground">{c.icon}{c.label}</div>
              <div className="text-2xl font-bold">{c.value}</div>
            </CardContent>
          </Card>
        ))}
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
        <Card>
          <CardHeader><CardTitle className="text-sm">本周抓取节奏</CardTitle></CardHeader>
          <CardContent className="flex items-end gap-2 h-32">
            {[42, 66, 58, 80, 96, 120, 150].map((v, i) => (
              <div key={i} className="flex-1 flex flex-col items-center gap-1">
                <div className="w-full rounded-t bg-violet-500/60" style={{ height: `${v}%` }} />
                <span className="text-[10px] text-muted-foreground">{["一", "二", "三", "四", "五", "六", "日"][i]}</span>
              </div>
            ))}
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle className="text-sm">阅读完成度</CardTitle></CardHeader>
          <CardContent className="flex flex-col gap-3">
            {[
              { label: "已处理文章", value: "96%", color: "bg-emerald-500" },
              { label: "已读 / 归档", value: "72%", color: "bg-violet-500" },
              { label: "选题转化", value: "3 / 5 个选题立项", color: "bg-amber-500" },
            ].map((r) => (
              <div key={r.label} className="flex flex-col gap-1">
                <div className="flex justify-between text-xs text-muted-foreground"><span>{r.label}</span><span>{r.value}</span></div>
                <div className="h-1.5 rounded bg-secondary overflow-hidden">
                  <div className={"h-full " + r.color} style={{ width: r.value.includes("%") ? r.value : "60%" }} />
                </div>
              </div>
            ))}
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground mt-2">
              <CheckCircle2 className="size-3.5 text-emerald-400" /> 数据来自本地 SQLite,不上传任何服务端
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
