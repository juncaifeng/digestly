import { useState } from "react"
import DOMPurify from "dompurify"
import { ExternalLink, X } from "lucide-react"

import { api, type Item, type ItemStatus } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

const statusVariant: Record<ItemStatus, "default" | "secondary" | "destructive"> = {
  pending: "secondary",
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

      <article
        className="flex-1 overflow-y-auto p-4 prose prose-sm dark:prose-invert max-w-none
          [&_img]:max-w-full [&_a]:text-blue-500 [&_pre]:bg-muted [&_pre]:p-2 [&_pre]:rounded"
        dangerouslySetInnerHTML={{
          __html: DOMPurify.sanitize(item.content || item.summary || "(无正文)"),
        }}
      />
    </div>
  )
}
