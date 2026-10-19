import { useEffect, useState } from "react"
import { Download, Check, Store, RefreshCw } from "lucide-react"

import { api, marketApi, type MarketEntry, type SourceFeed } from "@/api/client"
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
  const [srcURL, setSrcURL] = useState("")
  const [srcFeeds, setSrcFeeds] = useState<SourceFeed[]>([])
  const [srcLoading, setSrcLoading] = useState(false)
  const [subscribed, setSubscribed] = useState<Record<string, boolean>>({})

  // 解析第三方市场源: 只认仓库里明确声明的订阅源清单,没有就报"没有"
  const parseSource = async () => {
    if (!srcURL.trim()) return
    setSrcLoading(true)
    setSrcFeeds([])
    try {
      const r = await marketApi.parseSource(srcURL.trim())
      setSrcFeeds(r.feeds)
    } catch (e) {
      setError(`解析失败: ${(e as Error).message}`)
    } finally {
      setSrcLoading(false)
    }
  }

  // 订阅一条源(下一步才是采集,走既有 feed 流程)
  const subscribe = async (f: SourceFeed) => {
    try {
      await api.addFeed({ title: f.title, url: f.feed_url, collector: "rss" })
      setSubscribed((m) => ({ ...m, [f.feed_url]: true }))
      onInstalled()
    } catch (e) {
      setError(`订阅失败: ${(e as Error).message}`)
    }
  }

  // 走本地缓存,瞬时返回。首次(本地无清单)带 wait_ready 等后台预热,之后纯缓存。
  const load = (waitReady = false) => {
    setError("")
    marketApi
      .list(waitReady)
      .then((r) => {
        setEntries(r.collectors)
        setStale(r.stale)
        setLastFetch(r.last_fetch)
      })
      .catch((e) => setError(`市场清单加载失败: ${e.message}`))
  }
  useEffect(() => { void load(true) }, [])

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

      <div className="border rounded-lg p-3 flex flex-col gap-2">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-sm font-medium">第三方市场源</span>
          <Input
            className="max-w-md flex-1"
            placeholder="GitHub 仓库地址(仓库内需有 feeds.opml / feeds.json 清单)"
            value={srcURL}
            onChange={(e) => setSrcURL(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && parseSource()}
          />
          <Button size="sm" onClick={parseSource} disabled={srcLoading}>
            {srcLoading ? "解析中…" : "解析订阅源"}
          </Button>
        </div>
        {srcFeeds.length > 0 && (
          <div className="flex flex-col gap-1.5 max-h-56 overflow-y-auto pr-1">
            {srcFeeds.map((f) => (
              <div key={f.feed_url} className="flex items-center gap-2 text-sm">
                <div className="flex-1 min-w-0">
                  <div className="truncate">{f.title}</div>
                  <div className="text-xs text-muted-foreground truncate">{f.feed_url}</div>
                </div>
                <div className="flex gap-1 shrink-0">
                  {f.tags.slice(0, 2).map((t) => (
                    <Badge key={t} variant="outline">{t}</Badge>
                  ))}
                </div>
                <Button
                  size="sm"
                  variant={subscribed[f.feed_url] ? "outline" : "default"}
                  className="shrink-0"
                  onClick={() => subscribe(f)}
                >
                  {subscribed[f.feed_url] ? "已订阅" : "订阅"}
                </Button>
              </div>
            ))}
          </div>
        )}
      </div>

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
        {filtered.length === 0 && !error && entries.length === 0 && (
          <p className="text-muted-foreground text-sm col-span-full text-center mt-10">
            正在获取市场清单…
          </p>
        )}
        {filtered.length === 0 && !error && entries.length > 0 && (
          <p className="text-muted-foreground text-sm col-span-full text-center mt-10">
            没有匹配的订阅源
          </p>
        )}
      </div>
    </div>
  )
}
