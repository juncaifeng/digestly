import { useEffect, useRef, useState } from "react"
import DOMPurify from "dompurify"
import { Archive, ArchiveRestore, ExternalLink, X } from "lucide-react"

import { api, columnApi, type Column, type Item } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

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
  const articleRef = useRef<HTMLElement>(null)

  // 切换文章: 回到顶部 + 单独拉取正文(列表接口不携带 content,详情走 L2 LRU)
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
            {item.source && <Badge variant="outline">{item.source}</Badge>}
            <span className="text-xs text-muted-foreground self-center">#{item.id}</span>
          </div>
        </div>
        <div className="flex gap-1 shrink-0">
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

      <article
        ref={articleRef}
        className="flex-1 overflow-y-auto p-4 prose prose-sm dark:prose-invert max-w-none
          [&_img]:max-w-full [&_a]:text-blue-500 [&_pre]:bg-muted [&_pre]:p-2 [&_pre]:rounded"
        dangerouslySetInnerHTML={{
          __html: DOMPurify.sanitize(content || "(无正文)"),
        }}
      />
    </div>
  )
}
