import { useEffect, useRef, useState } from "react"
import DOMPurify from "dompurify"
import { Archive, ArchiveRestore, ExternalLink, RotateCcw, X } from "lucide-react"

import { api, columnApi, type Column, type Item, type ItemChunk, type ItemStatus } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

const statusVariant: Record<ItemStatus, "default" | "secondary" | "destructive" | "outline"> = {
  pending: "secondary",
  processing: "outline",
  processed: "default",
  failed: "destructive",
}

export function ItemDetail({
  item,
  onClose,
  onTagged,
}: {
  item: Item
  onClose: () => void
  onTagged: () => void
}) {
  const [tag, setTag] = useState("")
  const [content, setContent] = useState("")
  const [columns, setColumns] = useState<Column[]>([])
  const [favs, setFavs] = useState<number[]>([])
  // 查看方式: 阅读(整篇) / 滑窗切块(逐块原文 + 块摘要)
  const [view, setView] = useState<"read" | "chunks">("read")
  const [chunks, setChunks] = useState<ItemChunk[] | null>(null)
  const articleRef = useRef<HTMLElement>(null)
  const chunksRef = useRef<HTMLDivElement>(null)

  // 切块数据: 切到「滑窗切块」时才拉
  useEffect(() => {
    if (view !== "chunks") return
    let cancelled = false
    setChunks(null)
    api.getItemChunks(item.id)
      .then((cs) => { if (!cancelled) setChunks(cs) })
      .catch(console.error)
    return () => { cancelled = true }
  }, [view, item.id])

  // 切换文章: 回到顶部 + 单独拉取正文(列表接口不再携带 content,详情走 L2 LRU)
  useEffect(() => {
    articleRef.current?.scrollTo({ top: 0 })
    setContent("")
    setFavs([])
    let cancelled = false
    api.getItem(item.id)
      .then((full) => {
        if (cancelled) return
        setContent(full.content)
        setFavs(full.column_ids ?? [])
      })
      .catch(console.error)
    return () => { cancelled = true }
  }, [item.id])

  useEffect(() => {
    columnApi.list().then(setColumns).catch(console.error)
  }, [])

  const toggleFav = async (columnId: number) => {
    if (favs.includes(columnId)) {
      await columnApi.unfavorite(item.id, columnId)
      setFavs(favs.filter((c) => c !== columnId))
    } else {
      await columnApi.favorite(item.id, columnId)
      setFavs([...favs, columnId])
    }
  }

  const addTag = async () => {
    if (!tag.trim()) return
    await api.addTag(item.id, tag.trim())
    setTag("")
    onTagged()
  }

  return (
    <div className="flex-1 flex flex-col border-l overflow-hidden">
      <div className="flex items-start gap-2 p-4 border-b">
        <div className="flex-1 min-w-0">
          <h2 className="text-lg font-semibold leading-snug">{item.title || "(无标题)"}</h2>
          <p className="text-sm text-muted-foreground mt-1">
            {item.author && <span className="mr-2">{item.author}</span>}
            {item.published_at && new Date(item.published_at).toLocaleString()}
          </p>
          <div className="flex gap-1 flex-wrap mt-2">
            <Badge variant={statusVariant[item.status]}>{item.status}</Badge>
            {item.source && <Badge variant="outline">{item.source}</Badge>}
            {item.keywords.split(",").filter(Boolean).map((k) => (
              <Badge key={k} variant="secondary">{k}</Badge>
            ))}
          </div>
        </div>
        <div className="flex gap-1 shrink-0">
          {item.status === "failed" && (
            <Button
              variant="ghost" size="icon"
              title="标注失败,重试"
              onClick={() => api.retryItem(item.id).then(() => onTagged()).catch(console.error)}
            >
              <RotateCcw className="size-4" />
            </Button>
          )}
          <Button
            variant="ghost" size="icon"
            title={item.archived ? "取消归档" : "归档(从列表隐藏,不删除)"}
            onClick={() => api.archiveItem(item.id, !item.archived).then(() => { onTagged(); onClose() })}
          >
            {item.archived ? <ArchiveRestore className="size-4" /> : <Archive className="size-4" />}
          </Button>
          <Button variant="ghost" size="icon" asChild>
            <a href={item.link} target="_blank" rel="noreferrer">
              <ExternalLink className="size-4" />
            </a>
          </Button>
          <Button variant="ghost" size="icon" onClick={onClose}>
            <X className="size-4" />
          </Button>
        </div>
      </div>

      <div className="flex items-center gap-2 px-4 py-2 border-b">
        <div className="flex gap-1 flex-wrap flex-1">
          {(item.tags ?? []).map((t) => (
            <Badge key={t} variant="default">{t}</Badge>
          ))}
        </div>
        <Input
          className="max-w-36 h-8"
          placeholder="打标签…"
          value={tag}
          onChange={(e) => setTag(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && addTag()}
        />
        <Button size="sm" variant="outline" onClick={addTag}>添加</Button>
      </div>

      {columns.length > 0 && (
        <div className="flex items-center gap-2 px-4 py-2 border-b flex-wrap">
          <span className="text-xs text-muted-foreground shrink-0">收藏到专栏:</span>
          {columns.map((c) => (
            <Badge
              key={c.id}
              variant={favs.includes(c.id) ? "default" : "outline"}
              className="cursor-pointer"
              onClick={() => void toggleFav(c.id)}
            >
              {c.name}
            </Badge>
          ))}
        </div>
      )}

      <div className="flex items-center gap-2 px-4 py-2 border-b">
        <div className="flex rounded-md border overflow-hidden">
          <button
            className={"px-3 h-7 text-xs font-medium " + (view === "read" ? "bg-accent" : "hover:bg-accent/50")}
            onClick={() => setView("read")}
          >
            阅读
          </button>
          <button
            className={"px-3 h-7 text-xs font-medium border-l " + (view === "chunks" ? "bg-accent" : "hover:bg-accent/50")}
            onClick={() => setView("chunks")}
            title="滑动窗口切块: 逐块原文 + 块摘要(总摘要见顶部)"
          >
            滑窗切块
          </button>
        </div>
        {view === "chunks" && (
          <span className="text-xs text-muted-foreground">
            {chunks ? `${chunks.length} 块` : "加载中…"} · 总摘要见文章顶部
          </span>
        )}
      </div>

      {view === "read" ? (
        <article
          ref={articleRef}
          className="flex-1 overflow-y-auto p-4 prose prose-sm dark:prose-invert max-w-none
            [&_img]:max-w-full [&_a]:text-blue-500 [&_pre]:bg-muted [&_pre]:p-2 [&_pre]:rounded"
          dangerouslySetInnerHTML={{
            __html: DOMPurify.sanitize(content || item.summary || "(无正文)"),
          }}
        />
      ) : (
        <div ref={chunksRef} className="flex-1 overflow-y-auto p-4 flex flex-col gap-3">
          {(chunks ?? []).length === 0 && (
            <p className="text-sm text-muted-foreground text-center mt-8">
              {chunks === null ? "加载中…" : "还没有切块数据 — 配置 chat 模型后由标注管线生成"}
            </p>
          )}
          {(chunks ?? []).map((c) => (
            <div key={c.id} className="border rounded-lg overflow-hidden">
              <div className="px-3 py-2 bg-muted/40 flex items-start gap-2">
                <Badge variant="secondary" className="shrink-0">块 {c.index + 1}</Badge>
                <p className="text-sm font-medium flex-1">{c.summary || "(无块摘要)"}</p>
              </div>
              <details>
                <summary className="px-3 py-1.5 text-xs text-muted-foreground cursor-pointer hover:bg-accent/50">
                  块原文({c.text.length} 字)
                </summary>
                <p className="px-3 pb-3 text-sm whitespace-pre-wrap leading-relaxed">{c.text}</p>
              </details>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
