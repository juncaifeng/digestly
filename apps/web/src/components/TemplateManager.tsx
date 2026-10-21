import { useEffect, useState } from "react"
import { BookTemplate, Check, Pencil, Plus, Trash2 } from "lucide-react"

import { templateApi, type ArticleTemplate } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"

// 大纲模板管理: 创作中心子菜单复用;onChanged 在模板增删改后回调
export function TemplateManager({ onChanged }: { onChanged?: () => void }) {
  const [templates, setTemplates] = useState<ArticleTemplate[]>([])
  const [error, setError] = useState("")
  const [tplForm, setTplForm] = useState<{ id?: number; name: string; prompt: string }>({ name: "", prompt: "" })

  const load = () => {
    templateApi.list()
      .then((ts) => { setTemplates(ts); onChanged?.() })
      .catch((e) => setError(`模板加载失败: ${e.message}`))
  }
  useEffect(() => { templateApi.list().then(setTemplates).catch((e) => setError(`模板加载失败: ${e.message}`)) }, [])

  const saveTemplate = async () => {
    setError("")
    try {
      if (tplForm.id) {
        await templateApi.update(tplForm.id, tplForm)
      } else {
        await templateApi.create(tplForm)
      }
      setTplForm({ name: "", prompt: "" })
      load()
    } catch (e) {
      setError(`模板保存失败: ${(e as Error).message}`)
    }
  }

  return (
    <div className="flex flex-col gap-3">
      <h2 className="text-base font-semibold flex items-center gap-2">
        <BookTemplate className="size-5" /> 大纲模板管理
      </h2>
      {error && <p className="text-sm text-destructive">{error}</p>}
      {templates.map((t) => (
        <Card key={t.id}>
          <CardHeader className="flex-row items-center justify-between gap-2 space-y-0">
            <div className="flex items-center gap-2 min-w-0">
              <CardTitle className="text-base truncate">{t.name}</CardTitle>
              {t.built_in && <Badge variant="secondary">内置</Badge>}
            </div>
            <div className="flex gap-1 shrink-0">
              <Button variant="ghost" size="icon" className="size-7" title="编辑"
                onClick={() => setTplForm({ id: t.id, name: t.name, prompt: t.prompt })}>
                <Pencil className="size-3.5" />
              </Button>
              {!t.built_in && (
                <Button variant="ghost" size="icon" className="size-7" title="删除"
                  onClick={() => window.confirm(`删除模板「${t.name}」?`) && templateApi.remove(t.id).then(load)}>
                  <Trash2 className="size-3.5" />
                </Button>
              )}
            </div>
          </CardHeader>
          <CardContent>
            <pre className="text-xs text-muted-foreground whitespace-pre-wrap line-clamp-4">{t.prompt}</pre>
          </CardContent>
        </Card>
      ))}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{tplForm.id ? "编辑模板" : "新增模板"}</CardTitle>
          <CardDescription>prompt 中的 {"{{items}}"} 会被替换为素材清单</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-2">
          <Input placeholder="模板名称,如 话题综述" value={tplForm.name}
            onChange={(e) => setTplForm({ ...tplForm, name: e.target.value })} />
          <textarea
            className="border rounded-md p-2 text-sm min-h-40 bg-transparent"
            placeholder="生成 prompt…"
            value={tplForm.prompt}
            onChange={(e) => setTplForm({ ...tplForm, prompt: e.target.value })}
          />
          <div className="flex gap-2">
            <Button size="sm" onClick={saveTemplate}
              disabled={!tplForm.name.trim() || !tplForm.prompt.trim()}>
              {tplForm.id ? <><Check className="size-3.5" /> 保存</> : <><Plus className="size-3.5" /> 添加</>}
            </Button>
            {tplForm.id && (
              <Button size="sm" variant="ghost" onClick={() => setTplForm({ name: "", prompt: "" })}>取消编辑</Button>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
