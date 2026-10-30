import { useEffect, useState } from "react"
import { Download, Check, Globe, Plus, RefreshCw, Trash2 } from "lucide-react"

import { api, marketApi, type MountedSource, type SourceFeed } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"

// 订阅源市场: 挂载第三方市场源(OPML / feeds.json 清单),其订阅源进市场浏览与订阅。
export function SourceMarket({ onSubscribed }: { onSubscribed: () => void }) {
  const [sources, setSources] = useState<MountedSource[]>([])
  const [url, setUrl] = useState("")
  const [title, setTitle] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")
  const [query, setQuery] = useState("")
  const [activeTag, setActiveTag] = useState<string | undefined>()
  const [activeSource, setActiveSource] = useState<number | undefined>()
  // 已订阅状态: 本地标记 + 已有订阅源 URL(刷新后依然正确显示)
  const [subscribed, setSubscribed] = useState<Record<string, boolean>>({})

  const load = () => {
    marketApi
      .listSources()
      .then((r) => setSources(r.sources))
      .catch((e) => setError(`市场源加载失败: ${e.message}`))
    api
      .listFeeds()
      .then((feeds) =>
        setSubscribed(Object.fromEntries(feeds.map((f) => [f.url, true])))
      )
      .catch(() => {})
  }
  useEffect(() => { void load() }, [])

  // 挂载市场源: 解析成功才入库,源里的订阅源即刻进市场
  const add = async () => {
    if (!url.trim()) return
    setLoading(true)
    setError("")
    try {
      await marketApi.addSource(url.trim(), title.trim())
      setUrl("")
      setTitle("")
      load()
    } catch (e) {
      setError(`挂载失败: ${(e as Error).message}`)
    } finally {
      setLoading(false)
    }
  }

  // 导入本地 OPML 文件作为市场源
  const importLocal = async (file: File) => {
    setLoading(true)
    setError("")
    try {
      const text = await file.text()
      const r = await marketApi.importSource(text, file.name.replace(/\.(opml|xml)$/i, ""))
      load()
      setActiveSource(r.id)
    } catch (e) {
      setError(`导入失败: ${(e as Error).message}`)
    } finally {
      setLoading(false)
    }
  }

  // 导出某市场源目录为 OPML 文件
  const exportSource = async (id: number) => {
    try {
      const text = await marketApi.exportSource(id)
      const blob = new Blob([text], { type: "text/x-opml" })
      const a = document.createElement("a")
      a.href = URL.createObjectURL(blob)
      a.download = `market-${id}.opml`
      a.click()
      URL.revokeObjectURL(a.href)
    } catch (e) {
      setError(`导出失败: ${(e as Error).message}`)
    }
  }

  const subscribe = async (f: SourceFeed) => {
    try {
      await api.addFeed({ title: f.title, url: f.feed_url, collector: "rss" })
      onSubscribed()
    } catch (e) {
      // feeds.url 有 UNIQUE 约束: 重复订阅直接视为已订阅,不报错
      if (!(e as Error).message.includes("UNIQUE")) {
        setError(`订阅失败: ${(e as Error).message}`)
        return
      }
    }
    setSubscribed((m) => ({ ...m, [f.feed_url]: true }))
  }

  const feeds = sources.flatMap((s) =>
    (s.feeds ?? []).map((f) => ({ ...f, tags: f.tags ?? [], source_id: s.id, source_title: s.title }))
  )
  const allTags = [...new Set(feeds.flatMap((f) => f.tags))].sort()
  const filtered = feeds.filter(
    (f) =>
      (!activeSource || f.source_id === activeSource) &&
      (!activeTag || f.tags.includes(activeTag)) &&
      (!query ||
        f.title.toLowerCase().includes(query.toLowerCase()) ||
        f.feed_url.toLowerCase().includes(query.toLowerCase()))
  )

  return (
    <div className="flex-1 flex flex-col p-4 gap-3 overflow-hidden">
      <div className="flex items-center gap-2 flex-wrap">
        <h2 className="text-base font-semibold flex items-center gap-2 mr-2">
          <Globe className="size-5" /> 订阅源市场
        </h2>
        <Input
          className="max-w-72"
          placeholder="市场源地址(含 feeds.opml / feeds.json 的仓库或文件)"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && add()}
        />
        <Input
          className="max-w-32"
          placeholder="名称(可选)"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && add()}
        />
        <Button size="sm" onClick={add} disabled={loading || !url.trim()}>
          <Plus className="size-3.5" /> {loading ? "解析中…" : "挂载"}
        </Button>
        <label className="inline-flex">
          <input
            type="file"
            accept=".opml,.xml,text/x-opml,application/xml"
            className="hidden"
            onChange={(e) => {
              const f = e.target.files?.[0]
              if (f) void importLocal(f)
              e.target.value = ""
            }}
          />
          <span className="inline-flex items-center gap-1.5 h-8 px-3 rounded-md border text-sm font-medium cursor-pointer hover:bg-accent">
            <Plus className="size-3.5" /> 导入本地 OPML
          </span>
        </label>
        <Input
          className="max-w-44 ml-auto"
          placeholder="搜索名称 / 地址…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {/* 已挂载市场源 */}
      {sources.length > 0 && (
        <div className="flex items-center gap-2 flex-wrap text-xs">
          {sources.map((s) => (
            <div
              key={s.id}
              className={"flex items-center gap-1.5 border rounded-md px-2 py-1 cursor-pointer " +
                (activeSource === s.id ? "bg-secondary ring-1 ring-ring" : "")}
              title="点击只看这个市场源,再点取消"
              onClick={() => setActiveSource(activeSource === s.id ? undefined : s.id)}
            >
              <span className="font-medium">{s.title || s.url}</span>
              <Badge variant="secondary">{s.feeds?.length ?? 0} 源</Badge>
              {s.parse_err && (
                <Badge variant="destructive" title={s.parse_err}>
                  {(s.feeds?.length ?? 0) > 0 ? "刷新失败(仍在用上次清单)" : "解析失败"}
                </Badge>
              )}
              <Button variant="ghost" size="icon" className="size-5"
                title="刷新该市场源"
                onClick={(e) => { e.stopPropagation(); marketApi.refreshSource(s.id).then(load) }}>
                <RefreshCw className="size-3" />
              </Button>
              <Button variant="ghost" size="icon" className="size-5"
                title="导出该市场源目录为 OPML"
                onClick={(e) => { e.stopPropagation(); void exportSource(s.id) }}>
                <Download className="size-3" />
              </Button>
              <Button variant="ghost" size="icon" className="size-5"
                title="卸载"
                onClick={(e) => {
                  e.stopPropagation()
                  if (activeSource === s.id) setActiveSource(undefined)
                  marketApi.removeSource(s.id).then(load)
                }}>
                <Trash2 className="size-3" />
              </Button>
            </div>
          ))}
          {allTags.length > 0 && <span className="text-muted-foreground mx-1">|</span>}
          {allTags.map((t) => (
            <Button key={t} variant={activeTag === t ? "secondary" : "ghost"} size="sm"
              onClick={() => setActiveTag(activeTag === t ? undefined : t)}>
              {t}
            </Button>
          ))}
        </div>
      )}

      <div className="flex-1 overflow-y-auto grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3 content-start pr-1">
        {filtered.map((f) => (
          <Card key={f.feed_url}>
            <CardHeader>
              <CardTitle className="text-base">{f.title}</CardTitle>
              <CardDescription>
                {f.description || f.feed_url}
                {f.source_title && <span className="block mt-1 opacity-70">来自 {f.source_title}</span>}
              </CardDescription>
            </CardHeader>
            <CardContent className="flex items-center justify-between gap-2">
              <div className="flex gap-1 flex-wrap">
                {f.tags.slice(0, 3).map((t) => (
                  <Badge key={t} variant="outline">{t}</Badge>
                ))}
              </div>
              <Button size="sm" variant={subscribed[f.feed_url] ? "outline" : "default"}
                disabled={!!subscribed[f.feed_url]}
                onClick={() => subscribe(f)}>
                {subscribed[f.feed_url] ? <><Check className="size-3.5" /> 已订阅</> :
                  <><Download className="size-3.5" /> 订阅</>}
              </Button>
            </CardContent>
          </Card>
        ))}
        {filtered.length === 0 && (
          <p className="text-muted-foreground text-sm col-span-full text-center mt-10">
            {sources.length === 0
              ? "还没有市场源 — 粘贴一个含 feeds.opml / feeds.json 的地址挂载试试"
              : "没有匹配的订阅源"}
          </p>
        )}
      </div>
    </div>
  )
}
