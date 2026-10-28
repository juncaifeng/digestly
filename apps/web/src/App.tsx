import { useCallback, useEffect, useState } from "react"
import {
  Archive, ChevronDown, ChevronRight, FolderOpen, FolderPlus, LayoutList, ListChecks,
  Pencil, RefreshCw, Rss, Search, Settings, Settings2, Sparkles, Store, Trash2,
} from "lucide-react"

import { api, columnApi, type Column, type Feed, type Item, type ItemStatus } from "@/api/client"
import { CreationCenterView } from "@/components/CreationCenterView"
import { ItemDetail } from "@/components/ItemDetail"
import { MarketView } from "@/components/MarketView"
import { SettingsView } from "@/components/SettingsView"
import { TasksView } from "@/components/TasksView"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"

const statusVariant: Record<ItemStatus, "default" | "secondary" | "destructive" | "outline"> = {
  pending: "secondary",
  processing: "outline",
  processed: "default",
  failed: "destructive",
}

export default function App() {
  const [feeds, setFeeds] = useState<Feed[]>([])
  const [columns, setColumns] = useState<Column[]>([])
  const [items, setItems] = useState<Item[]>([])
  const [activeFeed, setActiveFeed] = useState<number | undefined>()
  const [activeColumn, setActiveColumn] = useState<number | undefined>()
  const [query, setQuery] = useState("")
  const [newFeed, setNewFeed] = useState({ title: "", url: "" })
  const [view, setView] = useState<"articles" | "market" | "creation" | "tasks" | "settings">("articles")
  const [selected, setSelected] = useState<Item | null>(null)
  const [newColumn, setNewColumn] = useState("")
  const [collapsed, setCollapsed] = useState<Record<number, boolean>>({})
  const [renaming, setRenaming] = useState<{ id: number; name: string } | null>(null)
  const [dragOver, setDragOver] = useState<number | "ungrouped" | null>(null)
  const [showArchived, setShowArchived] = useState(false)

  const loadFeeds = useCallback(() => api.listFeeds().then(setFeeds).catch(console.error), [])
  const loadColumns = useCallback(() => columnApi.list().then(setColumns).catch(console.error), [])
  const loadItems = useCallback(() => {
    api
      .listItems({ feed_id: activeFeed, column_id: activeColumn, q: query, limit: 100, archived: showArchived ? 1 : undefined })
      .then(setItems)
      .catch(console.error)
  }, [activeFeed, activeColumn, query, showArchived])

  useEffect(() => { void loadFeeds(); void loadColumns() }, [loadFeeds, loadColumns])
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
    setTimeout(() => { loadItems(); loadFeeds() }, 1500)
  }

  const addColumn = async () => {
    if (!newColumn.trim()) return
    try {
      await columnApi.create(newColumn.trim())
      setNewColumn("")
      loadColumns()
    } catch (e) {
      alert((e as Error).message)
    }
  }

  const removeColumn = async (c: Column) => {
    if (!window.confirm("删除专栏「" + c.name + "」? 组内订阅将移回未分组,手动收藏将清除。")) return
    await columnApi.remove(c.id)
    if (activeColumn === c.id) setActiveColumn(undefined)
    loadColumns(); loadFeeds(); loadItems()
  }

  const commitRename = async () => {
    if (!renaming) return
    const { id, name } = renaming
    setRenaming(null)
    if (!name.trim()) return
    await columnApi.rename(id, name.trim())
    loadColumns()
  }

  // 拖拽移动订阅到专栏(columnId 为 null 表示移回未分组)
  const moveFeed = async (feedId: number, columnId: number | null) => {
    await columnApi.setFeedColumn(feedId, columnId)
    loadFeeds(); loadColumns(); loadItems()
  }

  const selectColumn = (id: number | undefined) => {
    setActiveColumn(id); setActiveFeed(undefined); setSelected(null)
  }
  const selectFeed = (id: number | undefined) => {
    setActiveFeed(id); setActiveColumn(undefined); setSelected(null)
  }

  const dropProps = (key: number | "ungrouped", columnId: number | null) => ({
    onDragOver: (e: React.DragEvent) => { e.preventDefault(); setDragOver(key) },
    onDragLeave: () => setDragOver((d) => (d === key ? null : d)),
    onDrop: (e: React.DragEvent) => {
      e.preventDefault(); setDragOver(null)
      const fid = Number(e.dataTransfer.getData("text/feed-id"))
      if (fid) void moveFeed(fid, columnId)
    },
  })

  const feedRow = (f: Feed) => (
    <div
      key={f.id}
      className="flex items-center gap-1 pl-6"
      draggable
      onDragStart={(e) => e.dataTransfer.setData("text/feed-id", String(f.id))}
      title="可拖拽到专栏"
    >
      <Button
        variant={activeFeed === f.id ? "secondary" : "ghost"}
        size="sm"
        className="flex-1 justify-start"
        onClick={() => selectFeed(f.id)}
      >
        <span className="truncate">{f.title}</span>
        {f.unread_count > 0 && (
          <span className="ml-auto shrink-0 text-xs text-muted-foreground">{f.unread_count}</span>
        )}
      </Button>
      <Button
        variant="ghost" size="icon" className="size-7"
        onClick={() => api.refreshFeed(f.id).then(() => setTimeout(() => { loadItems(); loadFeeds() }, 1500))}
      >
        <RefreshCw className="size-3.5" />
      </Button>
      <Button
        variant="ghost" size="icon" className="size-7"
        title={"历史上限: 仅保留最近 N 条,0 = 不限" + (f.max_items > 0 ? `(当前 ${f.max_items})` : "(当前不限)")}
        onClick={() => {
          const v = window.prompt("历史上限: 仅保留最近 N 条,0 = 不限", String(f.max_items || 0))
          if (v == null) return
          const n = parseInt(v, 10)
          if (isNaN(n) || n < 0) { alert("请输入非负整数"); return }
          api.updateFeed(f.id, { max_items: n }).then(() => { loadFeeds(); loadItems() })
        }}
      >
        <Settings2 className="size-3.5" />
      </Button>
      <Button
        variant="ghost" size="icon" className="size-7"
        onClick={() => api.deleteFeed(f.id).then(() => { loadFeeds(); loadItems(); loadColumns() })}
      >
        <Trash2 className="size-3.5" />
      </Button>
    </div>
  )

  const ungrouped = feeds.filter((f) => f.column_id == null)
  const totalUnread = feeds.reduce((n, f) => n + f.unread_count, 0)
  const columnUnread = (cid: number) =>
    feeds.filter((f) => f.column_id === cid).reduce((n, f) => n + f.unread_count, 0)

  return (
    <div className="flex h-screen">
      <aside className="w-64 shrink-0 border-r flex flex-col p-4 gap-3">
        <h1 className="text-lg font-bold flex items-center gap-2">
          <Rss className="size-5" /> digestly
        </h1>
        <div className="grid grid-cols-2 gap-1">
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
          <Button
            variant={view === "creation" ? "secondary" : "ghost"}
            size="sm"
            className="flex-1"
            onClick={() => setView("creation")}
          >
            <Sparkles className="size-4" /> 创作
          </Button>
          <Button
            variant={view === "tasks" ? "secondary" : "ghost"}
            size="sm"
            className="flex-1"
            onClick={() => setView("tasks")}
          >
            <ListChecks className="size-4" /> 任务
          </Button>
          <Button
            variant={view === "settings" ? "secondary" : "ghost"}
            size="sm"
            className="flex-1"
            onClick={() => setView("settings")}
          >
            <Settings className="size-4" /> 设置
          </Button>
        </div>
        {view === "creation" && (
          <div id="creation-tree-slot" className="flex-1 flex flex-col gap-2 overflow-hidden" />
        )}
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
        <div className="flex gap-1">
          <Input
            className="h-8"
            placeholder="新建专栏…"
            value={newColumn}
            onChange={(e) => setNewColumn(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && addColumn()}
          />
          <Button size="sm" variant="outline" onClick={addColumn}>
            <FolderPlus className="size-4" />
          </Button>
        </div>
        <nav className="flex flex-col gap-1 overflow-y-auto">
          <Button
            variant={activeFeed === undefined && activeColumn === undefined ? "secondary" : "ghost"}
            size="sm"
            className="justify-start"
            onClick={() => { selectFeed(undefined); setActiveColumn(undefined) }}
          >
            全部文章
            {totalUnread > 0 && (
              <span className="ml-auto text-xs text-muted-foreground">{totalUnread}</span>
            )}
          </Button>

          {columns.map((c) => (
            <div key={c.id} className="flex flex-col gap-0.5">
              <div
                className={"flex items-center gap-0.5 rounded-md " +
                  (dragOver === c.id ? "bg-accent ring-1 ring-ring " : "")}
                {...dropProps(c.id, c.id)}
              >
                <Button
                  variant="ghost" size="icon" className="size-7 shrink-0"
                  onClick={() => setCollapsed({ ...collapsed, [c.id]: !collapsed[c.id] })}
                >
                  {collapsed[c.id] ? <ChevronRight className="size-3.5" /> : <ChevronDown className="size-3.5" />}
                </Button>
                {renaming?.id === c.id ? (
                  <Input
                    className="h-7 flex-1"
                    autoFocus
                    value={renaming.name}
                    onChange={(e) => setRenaming({ id: c.id, name: e.target.value })}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") void commitRename()
                      if (e.key === "Escape") setRenaming(null)
                    }}
                    onBlur={() => void commitRename()}
                  />
                ) : (
                  <Button
                    variant={activeColumn === c.id ? "secondary" : "ghost"}
                    size="sm"
                    className="flex-1 justify-start truncate"
                    onClick={() => selectColumn(c.id)}
                  >
                    <FolderOpen className="size-3.5 shrink-0" />
                    <span className="truncate">{c.name}</span>
                    {columnUnread(c.id) > 0 && (
                      <span className="ml-auto text-xs font-medium">{columnUnread(c.id)}</span>
                    )}
                    <span className={(columnUnread(c.id) > 0 ? "" : "ml-auto ") + "text-xs text-muted-foreground"}>
                      {c.feed_count}
                    </span>
                  </Button>
                )}
                <Button
                  variant="ghost" size="icon" className="size-7 shrink-0"
                  onClick={() => setRenaming({ id: c.id, name: c.name })}
                >
                  <Pencil className="size-3" />
                </Button>
                <Button
                  variant="ghost" size="icon" className="size-7 shrink-0"
                  onClick={() => removeColumn(c)}
                >
                  <Trash2 className="size-3" />
                </Button>
              </div>
              {!collapsed[c.id] && feeds.filter((f) => f.column_id === c.id).map(feedRow)}
            </div>
          ))}

          {columns.length > 0 && (
            <div
              className={"flex flex-col gap-0.5 rounded-md " +
                (dragOver === "ungrouped" ? "bg-accent ring-1 ring-ring " : "")}
              {...dropProps("ungrouped", null)}
            >
              <div className="px-2 pt-2 text-xs text-muted-foreground">未分组(拖到此处移出专栏)</div>
              {ungrouped.map(feedRow)}
            </div>
          )}
          {columns.length === 0 && feeds.map(feedRow)}
        </nav>
        </>)}
      </aside>

      {view === "market" ? (
        <MarketView onInstalled={() => { loadFeeds(); loadColumns() }} />
      ) : view === "settings" ? (
        <SettingsView />
      ) : view === "tasks" ? (
        <TasksView />
      ) : view === "creation" ? (
        <CreationCenterView onGenerated={() => { loadItems(); loadFeeds() }} />
      ) : (
      <main className="flex-1 flex overflow-hidden">
      <div className="flex-1 flex flex-col p-4 gap-3 overflow-hidden">
        <div className="flex items-center gap-2">
          <div className="relative flex-1">
            <Search className="absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
            <Input
              className="pl-8"
              placeholder="全文搜索(标题 / 正文 / 摘要 / 关键词)…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>
          {activeFeed && !showArchived && (<>
            <Button size="sm" variant="outline" title="归档该源全部已读文章(隐藏但不删除)"
              onClick={() => {
                if (!window.confirm("归档该源全部已读文章? 可在归档箱中查看/恢复")) return
                api.archiveFeed(activeFeed, "read").then(() => { loadItems(); loadFeeds() })
              }}>
              归档已读
            </Button>
            <Button size="sm" variant="outline" title="归档该源全部文章(隐藏但不删除)"
              onClick={() => {
                if (!window.confirm("归档该源全部文章(含未读)? 可在归档箱中查看/恢复")) return
                api.archiveFeed(activeFeed, "all").then(() => { loadItems(); loadFeeds() })
              }}>
              归档全部
            </Button>
          </>)}
          <Button
            size="sm" variant={showArchived ? "secondary" : "ghost"}
            title="查看已归档文章"
            onClick={() => { setShowArchived(!showArchived); setSelected(null) }}
          >
            <Archive className="size-4" /> 归档箱
          </Button>
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
                if (!it.read) api.markRead(it.id, true).then(() => { loadItems(); loadFeeds() })
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
              {showArchived ? "归档箱是空的" : "暂无文章，先在左侧添加一个订阅源试试"}
            </p>
          )}
        </div>
      </div>
      {selected && (
        <ItemDetail
          item={selected}
          onClose={() => setSelected(null)}
          onTagged={() => { loadItems(); loadFeeds() }}
        />
      )}
      </main>
      )}
    </div>
  )
}
