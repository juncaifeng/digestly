import { useEffect, useState } from "react"
import { Download, Check, Store, RefreshCw } from "lucide-react"

import { marketApi, type MarketEntry } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"

export function MarketView({ onInstalled }: { onInstalled: () => void }) {
  const [entries, setEntries] = useState<MarketEntry[]>([])
  const [query, setQuery] = useState("")
  const [activeTag, setActiveTag] = useState<string | undefined>()
  const [installing, setInstalling] = useState<string | null>(null)
  const [error, setError] = useState("")
  const [stale, setStale] = useState(false)
  const [lastFetch, setLastFetch] = useState("")
  const [refreshing, setRefreshing] = useState(false)

  // 走本地缓存,瞬时返回
  const load = () => {
    setError("")
    marketApi
      .list()
      .then((r) => {
        setEntries(r.collectors)
        setStale(r.stale)
        setLastFetch(r.last_fetch)
      })
      .catch((e) => setError(`市场清单加载失败: ${e.message}`))
  }
  useEffect(() => { void load() }, [])

  // 显式回源远端拉取最新清单
  const refresh = async () => {
    setRefreshing(true)
    try {
      await marketApi.refresh()
      load()
    } catch (e) {
      setError(`刷新失败: ${(e as Error).message}`)
    } finally {
      setRefreshing(false)
    }
  }

  const install = async (name: string) => {
    setInstalling(name)
    try {
      await marketApi.install(name)
      load()
      onInstalled()
    } catch (e) {
      setError(`安装失败: ${(e as Error).message}`)
    } finally {
      setInstalling(null)
    }
  }

  const allTags = [...new Set(entries.flatMap((e) => e.tags))].sort()
  const filtered = entries.filter(
    (e) =>
      (!activeTag || e.tags.includes(activeTag)) &&
      (!query ||
        e.title.toLowerCase().includes(query.toLowerCase()) ||
        e.description.toLowerCase().includes(query.toLowerCase()))
  )

  return (
    <div className="flex-1 flex flex-col p-4 gap-3 overflow-hidden">
      <div className="flex items-center gap-2 flex-wrap">
        <h2 className="text-base font-semibold flex items-center gap-2 mr-2">
          <Store className="size-5" /> 订阅源市场
        </h2>
        <Input
          className="max-w-56"
          placeholder="搜索名称 / 描述…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <Button
          variant={activeTag === undefined ? "secondary" : "ghost"}
          size="sm"
          onClick={() => setActiveTag(undefined)}
        >
          全部
        </Button>
        {allTags.map((t) => (
          <Button
            key={t}
            variant={activeTag === t ? "secondary" : "ghost"}
            size="sm"
            onClick={() => setActiveTag(t)}
          >
            {t}
          </Button>
        ))}
        <Button
          variant="ghost"
          size="icon"
          className="ml-auto"
          disabled={refreshing}
          title="从远端拉取最新市场清单"
          onClick={refresh}
        >
          <RefreshCw className={"size-4 " + (refreshing ? "animate-spin" : "")} />
        </Button>
      </div>

      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        {lastFetch && <span>清单更新于 {new Date(lastFetch).toLocaleString()}</span>}
        {stale && (
          <span className="text-amber-600">
            清单可能已过期,点右上角刷新获取最新采集器
          </span>
        )}
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="flex-1 overflow-y-auto grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3 content-start pr-1">
        {filtered.map((e) => (
          <Card key={e.name}>
            <CardHeader>
              <CardTitle className="text-base flex items-center justify-between gap-2">
                <span className="truncate">{e.title}</span>
                {e.installed && !e.update_available && (
                  <Badge variant="secondary" className="shrink-0">
                    <Check className="size-3" /> 已安装
                  </Badge>
                )}
                {e.update_available && (
                  <Badge className="shrink-0 bg-amber-500 text-white">可更新 v{e.script_version}</Badge>
                )}
              </CardTitle>
              <CardDescription>{e.description}</CardDescription>
            </CardHeader>
            <CardContent className="flex items-center justify-between gap-2">
              <div className="flex gap-1 flex-wrap">
                {e.tags.map((t) => (
                  <Badge key={t} variant="outline">{t}</Badge>
                ))}
              </div>
              <Button
                size="sm"
                variant={e.installed ? "outline" : "default"}
                disabled={installing === e.name}
                onClick={() => install(e.name)}
              >
                <Download className="size-3.5" />
                {installing === e.name ? "安装中…" : e.update_available ? "更新" : e.installed ? "重装" : "安装"}
              </Button>
            </CardContent>
          </Card>
        ))}
        {filtered.length === 0 && !error && (
          <p className="text-muted-foreground text-sm col-span-full text-center mt-10">
            没有匹配的订阅源
          </p>
        )}
      </div>
    </div>
  )
}
