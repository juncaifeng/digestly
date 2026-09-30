import { useCallback, useEffect, useState } from "react"
import { LayoutList, RefreshCw, Rss, Search, Store, Trash2 } from "lucide-react"

import { api, type Feed, type Item, type ItemStatus } from "@/api/client"
import { ItemDetail } from "@/components/ItemDetail"
import { MarketView } from "@/components/MarketView"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"

const statusVariant: Record<ItemStatus, "default" | "secondary" | "destructive"> = {
  pending: "secondary",
  processed: "default",
  failed: "destructive",
}

export default function App() {
  const [feeds, setFeeds] = useState<Feed[]>([])
  const [items, setItems] = useState<Item[]>([])
  const [activeFeed, setActiveFeed] = useState<number | undefined>()
  const [query, setQuery] = useState("")
  const [newFeed, setNewFeed] = useState({ title: "", url: "" })
  const [view, setView] = useState<"articles" | "market">("articles")
  const [selected, setSelected] = useState<Item | null>(null)

  const loadFeeds = useCallback(() => api.listFeeds().then(setFeeds).catch(console.error), [])
  const loadItems = useCallback(() => {
    api
      .listItems({ feed_id: activeFeed, q: query, limit: 100 })
      .then(setItems)
      .catch(console.error)
  }, [activeFeed, query])

  useEffect(() => { void loadFeeds() }, [loadFeeds])
  useEffect(() => { void loadItems() }, [loadItems])
  useEffect(() => {
    if (selected) {
      const fresh = items.find((i) => i.id === selected.id)
      if (fresh && fresh !== selected) setSelected(fresh)
    }
  }, [items])

  const addFeed = async () => {
    if (!newFeed.title || !newFeed.url) return
    const f = await api.addFeed(newFeed)
    setNewFeed({ title: "", url: "" })
    loadFeeds()
    await api.refreshFeed(f.id)
    setTimeout(loadItems, 1500)
  }

  return (
    <div className="flex h-screen">
      <aside className="w-64 shrink-0 border-r flex flex-col p-4 gap-3">
        <h1 className="text-lg font-bold flex items-center gap-2">
          <Rss className="size-5" /> digestly
        </h1>
        <div className="flex gap-1">
          <Button
            variant={view === "articles" ? "secondary" : "ghost"}
            size="sm"
            className="flex-1"
            onClick={() => setView("articles")}
          >
            <LayoutList className="size-4" /> 文章
          </Button>
          <Button
            variant={view === "market" ? "secondary" : "ghost"}
            size="sm"
            className="flex-1"
            onClick={() => setView("market")}
          >
            <Store className="size-4" /> 市场
          </Button>
        </div>
        {view === "articles" && (<>
        <div className="flex flex-col gap-2">
          <Input
            placeholder="订阅源名称"
            value={newFeed.title}
            onChange={(e) => setNewFeed({ ...newFeed, title: e.target.value })}
          />
          <Input
            placeholder="RSS / 采集 URL"
            value={newFeed.url}
            onChange={(e) => setNewFeed({ ...newFeed, url: e.target.value })}
          />
          <Button size="sm" onClick={addFeed}>添加订阅</Button>
        </div>
        <nav className="flex flex-col gap-1 overflow-y-auto">
          <Button
            variant={activeFeed === undefined ? "secondary" : "ghost"}
            size="sm"
            className="justify-start"
            onClick={() => { setActiveFeed(undefined); setSelected(null) }}
          >
            全部文章
          </Button>
          {feeds.map((f) => (
            <div key={f.id} className="flex items-center gap-1">
              <Button
                variant={activeFeed === f.id ? "secondary" : "ghost"}
                size="sm"
                className="flex-1 justify-start truncate"
                onClick={() => { setActiveFeed(f.id); setSelected(null) }}
              >
                {f.title}
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className="size-7"
                onClick={() => api.refreshFeed(f.id).then(() => setTimeout(loadItems, 1500))}
              >
                <RefreshCw className="size-3.5" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className="size-7"
                onClick={() => api.deleteFeed(f.id).then(() => { loadFeeds(); loadItems() })}
              >
                <Trash2 className="size-3.5" />
              </Button>
            </div>
          ))}
        </nav>
        </>)}
      </aside>

      {view === "market" ? (
        <MarketView onInstalled={loadFeeds} />
      ) : (
      <main className="flex-1 flex overflow-hidden">
      <div className="flex-1 flex flex-col p-4 gap-3 overflow-hidden">
        <div className="relative">
          <Search className="absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
          <Input
            className="pl-8"
            placeholder="全文搜索(标题 / 正文 / 摘要 / 关键词)…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </div>
        <div className={"flex-1 overflow-y-auto flex flex-col gap-3 pr-1 " +
          (selected ? "max-w-md" : "")}>
          {items.map((it) => (
            <Card
              key={it.id}
              className={(it.read ? "opacity-60 " : "") +
                (selected?.id === it.id ? "ring-2 ring-ring " : "") +
                "cursor-pointer"}
              onClick={() => {
                setSelected(it)
                if (!it.read) api.markRead(it.id, true).then(loadItems)
              }}
            >
              <CardHeader>
                <div className="flex items-center gap-2 flex-wrap">
                  <CardTitle className="text-base">
                    <a
                      href={it.link}
                      target="_blank"
                      rel="noreferrer"
                      className="hover:underline"
                      onClick={(e) => e.stopPropagation()}
                    >
                      {it.title || "(无标题)"}
                    </a>
                  </CardTitle>
                  <Badge variant={statusVariant[it.status]}>{it.status}</Badge>
                  {it.source && <Badge variant="outline">{it.source}</Badge>}
                </div>
                <CardDescription>
                  {it.author && <span className="mr-2">{it.author}</span>}
                  {it.published_at && new Date(it.published_at).toLocaleString()}
                </CardDescription>
              </CardHeader>
              <CardContent className="flex flex-col gap-2">
                {it.summary && <p className="text-sm text-muted-foreground">{it.summary}</p>}
                {it.keywords && (
                  <div className="flex gap-1 flex-wrap">
                    {it.keywords.split(",").filter(Boolean).map((k) => (
                      <Badge key={k} variant="secondary">{k}</Badge>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          ))}
          {items.length === 0 && (
            <p className="text-muted-foreground text-sm text-center mt-10">
              暂无文章，先在左侧添加一个订阅源试试
            </p>
          )}
        </div>
      </div>
      {selected && (
        <ItemDetail
          item={selected}
          onClose={() => setSelected(null)}
          onTagged={loadItems}
        />
      )}
      </main>
      )}
    </div>
  )
}
