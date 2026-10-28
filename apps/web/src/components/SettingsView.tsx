import { useEffect, useState } from "react"
import { Check, Copy, Cpu, Download, Pencil, Plus, RefreshCw, Save, Star, Trash2, Zap } from "lucide-react"

import { ingestApi, llmApi, mcpApi, opmlApi, type EmbeddingProviderPreset, type LLMConfig, type ProviderPreset } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"

// 设置页: LLM 供应商配置(大纲模板管理已挪到创作中心子菜单)
export function SettingsView() {
  const [configs, setConfigs] = useState<LLMConfig[]>([])
  const [error, setError] = useState("")
  const [notice, setNotice] = useState("")

  // LLM 配置表单(editing.id > 0 为编辑)
  const emptyCfg = { name: "", kind: "chat" as "chat" | "embedding", base_url: "", api_key: "", model: "", temperature: 0.7, max_tokens: 4096 }
  const [form, setForm] = useState<typeof emptyCfg & { id?: number }>(emptyCfg)
  const [testing, setTesting] = useState(false)

  // 供应商预设(内置 YAML + data/llm_providers 用户覆盖)
  const [presets, setPresets] = useState<ProviderPreset[]>([])
  const [presetName, setPresetName] = useState("")
  // 向量模型预设(独立 schema,含本地运行时 Ollama / LM Studio 等)
  const [embedPresets, setEmbedPresets] = useState<EmbeddingProviderPreset[]>([])
  const [embedPresetName, setEmbedPresetName] = useState("")

  // 采集入库限制(全局默认)
  const [ingest, setIngest] = useState({ max_items: 50, max_age_days: 30 })

  // MCP 服务状态
  const [mcpState, setMcpState] = useState<{
    status: "checking" | "ok" | "error"
    tools: string[]
    protocol: string
    error: string
  }>({ status: "checking", tools: [], protocol: "", error: "" })

  const checkMcp = () => {
    setMcpState((s) => ({ ...s, status: "checking", error: "" }))
    mcpApi.status()
      .then((r) => setMcpState({ status: "ok", tools: r.tools, protocol: r.protocol, error: "" }))
      .catch((e) => setMcpState({ status: "error", tools: [], protocol: "", error: (e as Error).message }))
  }
  useEffect(() => { checkMcp() }, [])

  const load = () => {
    llmApi.list().then(setConfigs).catch((e) => setError(`LLM 配置加载失败: ${e.message}`))
    llmApi.presets().then(setPresets).catch(() => {})
    llmApi.embeddingPresets().then(setEmbedPresets).catch(() => {})
    ingestApi.get().then(setIngest).catch(() => {})
  }
  useEffect(() => { void load() }, [])

  const preset = presets.find((p) => p.name === presetName)
  const embedPreset = embedPresets.find((p) => p.name === embedPresetName)
  const presetModel = preset?.models.find((m) => m.id === form.model)

  const applyPreset = (name: string) => {
    setPresetName(name)
    const p = presets.find((x) => x.name === name)
    if (!p) return
    setForm((f) => ({
      ...f,
      kind: "chat",
      name: f.name || p.name,
      base_url: p.base_url,
      model: p.models[0]?.id ?? f.model,
    }))
  }

  const applyEmbedPreset = (name: string) => {
    setEmbedPresetName(name)
    const p = embedPresets.find((x) => x.name === name)
    if (!p) return
    setForm((f) => ({
      ...f,
      kind: "embedding",
      name: f.name || p.name,
      base_url: p.base_url,
      model: p.models[0]?.id ?? f.model,
    }))
  }

  const saveConfig = async () => {
    setError(""); setNotice("")
    try {
      if (form.id) {
        await llmApi.update(form.id, form)
      } else {
        await llmApi.create(form)
      }
      setForm(emptyCfg)
      load()
    } catch (e) {
      setError(`保存失败: ${(e as Error).message}`)
    }
  }

  const testConfig = async () => {
    setTesting(true)
    setError(""); setNotice("")
    try {
      const r = await llmApi.test({ id: form.id, kind: form.kind, base_url: form.base_url, api_key: form.api_key, model: form.model })
      setNotice(`连接成功,模型回复: ${r.reply}`)
    } catch (e) {
      setError(`连接失败: ${(e as Error).message}`)
    } finally {
      setTesting(false)
    }
  }

  const copyMcpUrl = async () => {
    try {
      await navigator.clipboard.writeText(mcpApi.url())
      setNotice("已复制 MCP 连接地址")
    } catch {
      setError("复制失败,请手动选中地址复制")
    }
  }

  // OPML 导出: 下载为文件
  const exportOpml = async () => {
    setError(""); setNotice("")
    try {
      const text = await opmlApi.export()
      const blob = new Blob([text], { type: "text/x-opml" })
      const a = document.createElement("a")
      a.href = URL.createObjectURL(blob)
      a.download = "digestly-subscriptions.opml"
      a.click()
      URL.revokeObjectURL(a.href)
      setNotice("已导出 OPML 文件")
    } catch (e) {
      setError(`导出失败: ${(e as Error).message}`)
    }
  }

  // OPML 导入: 选文件 -> 提交文本 -> 汇报结果
  const importOpml = async (file: File) => {
    setError(""); setNotice("")
    try {
      const text = await file.text()
      const r = await opmlApi.import(text)
      setNotice(`导入完成: 新增 ${r.added} 个订阅,跳过已订阅 ${r.skipped} 个,新建专栏 ${r.columns_created} 个(共解析 ${r.total} 条)`)
    } catch (e) {
      setError(`导入失败: ${(e as Error).message}`)
    }
  }

  return (
    <div className="flex-1 overflow-y-auto p-4">
      <div className="max-w-3xl flex flex-col gap-6">
        {error && <p className="text-sm text-destructive">{error}</p>}
        {notice && <p className="text-sm text-green-600">{notice}</p>}

        {/* MCP 服务 */}
        <section className="flex flex-col gap-3">
          <h2 className="text-base font-semibold flex items-center gap-2">
            <Zap className="size-5" /> MCP 服务
            <span className="ml-auto">
              {mcpState.status === "ok" ? (
                <Badge>正常 · {mcpState.tools.length} 个工具</Badge>
              ) : mcpState.status === "checking" ? (
                <Badge variant="secondary">检测中…</Badge>
              ) : (
                <Badge variant="destructive">不可用</Badge>
              )}
            </span>
          </h2>
          <Card>
            <CardContent className="flex flex-col gap-2 pt-4">
              <div className="flex items-center gap-2">
                <code className="flex-1 text-sm border rounded-md px-2 py-1.5 bg-muted/40 truncate">
                  {mcpApi.url()}
                </code>
                <Button size="sm" variant="outline" onClick={copyMcpUrl} title="复制连接地址">
                  <Copy className="size-3.5" /> 复制
                </Button>
                <Button size="sm" variant="ghost" onClick={checkMcp} disabled={mcpState.status === "checking"}
                  title="重新检测">
                  <RefreshCw className={"size-3.5 " + (mcpState.status === "checking" ? "animate-spin" : "")} />
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                MCP 客户端(Claude Code / Kimi Code / Cursor 等)把该地址添加为 MCP server,
                即可直接调用 digestly 的查询 / 阅读 / 创作工具(与 digestly-cli 同一套工具面)。
                {mcpState.protocol && ` 协议版本: ${mcpState.protocol}`}
              </p>
              {mcpState.status === "error" && (
                <p className="text-xs text-destructive">
                  探测失败: {mcpState.error}(确认 digestly 桌面端/服务在运行)
                </p>
              )}
              {mcpState.status === "ok" && mcpState.tools.length > 0 && (
                <details>
                  <summary className="text-xs text-muted-foreground cursor-pointer">
                    工具清单({mcpState.tools.length})
                  </summary>
                  <div className="flex flex-wrap gap-1 pt-2">
                    {mcpState.tools.map((t) => (
                      <Badge key={t} variant="secondary" className="font-mono text-xs">{t}</Badge>
                    ))}
                  </div>
                </details>
              )}
            </CardContent>
          </Card>
        </section>

        {/* 订阅导入导出(OPML) */}
        <section className="flex flex-col gap-3">
          <h2 className="text-base font-semibold flex items-center gap-2">
            <RefreshCw className="size-5" /> 订阅导入导出(OPML)
          </h2>
          <Card>
            <CardContent className="flex flex-col gap-2 pt-4">
              <div className="flex items-center gap-2">
                <Button size="sm" variant="outline" onClick={exportOpml}>
                  <Download className="size-3.5" /> 导出 OPML(备份 / 迁移)
                </Button>
                <label className="inline-flex">
                  <input
                    type="file"
                    accept=".opml,.xml,text/x-opml,application/xml"
                    className="hidden"
                    onChange={(e) => {
                      const f = e.target.files?.[0]
                      if (f) void importOpml(f)
                      e.target.value = ""
                    }}
                  />
                  <span className="inline-flex items-center gap-1.5 h-8 px-3 rounded-md border text-sm font-medium cursor-pointer hover:bg-accent">
                    <Plus className="size-3.5" /> 导入 OPML 文件
                  </span>
                </label>
              </div>
              <p className="text-xs text-muted-foreground">
                导出: 全部订阅源按专栏分组生成 OPML,可备份或迁移到其他阅读器。
                导入: OPML 里的分组映射为专栏(不存在自动创建),已订阅的源(按 URL)自动跳过。
              </p>
            </CardContent>
          </Card>
        </section>

        {/* 采集限制(入库前截断) */}
        <section className="flex flex-col gap-3">
          <h2 className="text-base font-semibold flex items-center gap-2">
            <RefreshCw className="size-5" /> 采集限制
            <Badge variant="secondary" className="ml-auto">入库前截断</Badge>
          </h2>
          <Card>
            <CardContent className="flex flex-col gap-2 pt-4">
              <div className="grid grid-cols-2 gap-2">
                <div className="flex flex-col gap-1">
                  <span className="text-xs text-muted-foreground">每源最多收最近 N 条(0 = 不限)</span>
                  <Input type="number" min={0} value={ingest.max_items}
                    onChange={(e) => setIngest({ ...ingest, max_items: parseInt(e.target.value, 10) || 0 })} />
                </div>
                <div className="flex flex-col gap-1">
                  <span className="text-xs text-muted-foreground">只收最近 X 天发布的(0 = 不限)</span>
                  <Input type="number" min={0} value={ingest.max_age_days}
                    onChange={(e) => setIngest({ ...ingest, max_age_days: parseInt(e.target.value, 10) || 0 })} />
                </div>
              </div>
              <div>
                <Button size="sm" onClick={() => ingestApi.save(ingest.max_items, ingest.max_age_days)
                  .then(() => setNotice("采集限制已保存,下次拉取生效"))
                  .catch((e) => setError(`保存失败: ${(e as Error).message}`))}>
                  <Save className="size-3.5" /> 保存
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                拉取时超过天数或超出条数(取最近)的不再入库——防止首次订阅整站 RSS 一次灌入上千条历史。
                只影响新拉取,已入库不动(用归档管理存量);订阅行上的「历史上限」优先于全局条数。
              </p>
            </CardContent>
          </Card>
        </section>

        {/* LLM 配置 */}
        <section className="flex flex-col gap-3">
          <h2 className="text-base font-semibold flex items-center gap-2">
            <Cpu className="size-5" /> LLM 配置
          </h2>
          {configs.map((c) => (
            <Card key={c.id}>
              <CardHeader className="flex-row items-center justify-between gap-2 space-y-0">
                <div className="flex items-center gap-2 min-w-0">
                  <CardTitle className="text-base truncate">{c.name}</CardTitle>
                  {c.kind === "embedding" && <Badge variant="secondary">向量</Badge>}
                  {c.is_default && <Badge>默认</Badge>}
                </div>
                <div className="flex gap-1 shrink-0">
                  {!c.is_default && (
                    <Button variant="ghost" size="icon" className="size-7" title="设为默认"
                      onClick={() => llmApi.setDefault(c.id).then(load)}>
                      <Star className="size-3.5" />
                    </Button>
                  )}
                  <Button variant="ghost" size="icon" className="size-7" title="编辑"
                    onClick={() => setForm({ id: c.id, name: c.name, kind: (c.kind === "embedding" ? "embedding" : "chat"), base_url: c.base_url, api_key: "", model: c.model, temperature: c.temperature, max_tokens: c.max_tokens })}>
                    <Pencil className="size-3.5" />
                  </Button>
                  <Button variant="ghost" size="icon" className="size-7" title="删除"
                    onClick={() => window.confirm(`删除配置「${c.name}」?`) && llmApi.remove(c.id).then(load)}>
                    <Trash2 className="size-3.5" />
                  </Button>
                </div>
              </CardHeader>
              <CardContent className="text-sm text-muted-foreground flex flex-col gap-0.5">
                <span>{c.base_url} · {c.model}</span>
                <span>temperature {c.temperature} · max_tokens {c.max_tokens} · key {c.api_key || "(未设置)"}</span>
              </CardContent>
            </Card>
          ))}

          <Card>
            <CardHeader>
              <CardTitle className="text-base">{form.id ? "编辑配置" : "新增配置"}</CardTitle>
              <CardDescription>OpenAI 兼容接口;api_key 留空表示不修改已有密钥</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-2">
              <select
                className="border rounded-md h-9 px-2 text-sm bg-transparent"
                value={form.kind}
                onChange={(e) => setForm({ ...form, kind: e.target.value as "chat" | "embedding", model: "" })}
              >
                <option value="chat">对话 / 生成(chat)</option>
                <option value="embedding">向量模型(embedding)</option>
              </select>
              {form.kind === "chat" && presets.length > 0 && (
                <select
                  className="border rounded-md h-9 px-2 text-sm bg-transparent"
                  value={presetName}
                  onChange={(e) => applyPreset(e.target.value)}
                >
                  <option value="">从 LLM 供应商预设填充(DeepSeek / Kimi / 智谱 / 通义…)…</option>
                  {presets.map((p) => (
                    <option key={p.name} value={p.name}>{p.name}</option>
                  ))}
                </select>
              )}
              {form.kind === "embedding" && embedPresets.length > 0 && (
                <select
                  className="border rounded-md h-9 px-2 text-sm bg-transparent"
                  value={embedPresetName}
                  onChange={(e) => applyEmbedPreset(e.target.value)}
                >
                  <option value="">从向量模型预设填充(OpenAI / 硅基流动 / Ollama 本地…)…</option>
                  {embedPresets.map((p) => (
                    <option key={p.name} value={p.name}>{p.name}{p.local ? " · 本地" : ""}</option>
                  ))}
                </select>
              )}
              {form.kind === "embedding" && embedPreset?.note && (
                <p className="text-xs text-muted-foreground">{embedPreset.note}</p>
              )}
              <div className="grid grid-cols-2 gap-2">
                <Input placeholder="名称,如 DeepSeek" value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })} />
                <Input placeholder="模型,如 deepseek-flash" value={form.model}
                  onChange={(e) => setForm({ ...form, model: e.target.value })} />
              </div>
              {form.kind === "embedding" && embedPreset && embedPreset.models.length > 0 && (
                <select
                  className="border rounded-md h-9 px-2 text-sm bg-transparent"
                  value={embedPreset.models.some((m) => m.id === form.model) ? form.model : ""}
                  onChange={(e) => e.target.value && setForm({ ...form, model: e.target.value })}
                >
                  <option value="">选择 {embedPreset.name} 的向量模型…</option>
                  {embedPreset.models.map((m) => (
                    <option key={m.id} value={m.id}>{m.label || m.id}{m.dimensions ? ` · ${m.dimensions} 维` : ""}</option>
                  ))}
                </select>
              )}
              {form.kind === "chat" && preset && preset.models.length > 0 && (
                <select
                  className="border rounded-md h-9 px-2 text-sm bg-transparent"
                  value={preset.models.some((m) => m.id === form.model) ? form.model : ""}
                  onChange={(e) => e.target.value && setForm({ ...form, model: e.target.value })}
                >
                  <option value="">选择 {preset.name} 的模型…</option>
                  {preset.models.map((m) => (
                    <option key={m.id} value={m.id}>{m.label || m.id}</option>
                  ))}
                </select>
              )}
              {form.kind === "chat" && presetModel && (
                <div className="text-xs text-muted-foreground border rounded-md p-2 flex flex-col gap-0.5">
                  {presetModel.label && <span>版本: {presetModel.label}</span>}
                  <span>
                    {presetModel.context && `上下文 ${presetModel.context}`}
                    {presetModel.max_output && ` · 最大输出 ${presetModel.max_output}`}
                    {presetModel.concurrency > 0 && ` · 并发 ${presetModel.concurrency}`}
                    {presetModel.vision && " · 支持图像理解"}
                  </span>
                  {presetModel.thinking && <span>思考模式: {presetModel.thinking}</span>}
                  {presetModel.pricing && <pre className="whitespace-pre-wrap">{presetModel.pricing}</pre>}
                </div>
              )}
              {form.kind === "chat" && preset?.note && <p className="text-xs text-muted-foreground">{preset.note}</p>}
              <Input placeholder="Base URL,如 https://api.deepseek.com/v1" value={form.base_url}
                onChange={(e) => setForm({ ...form, base_url: e.target.value })} />
              <Input placeholder="API Key" type="password" value={form.api_key}
                onChange={(e) => setForm({ ...form, api_key: e.target.value })} />
              {form.kind === "chat" && (
                <div className="grid grid-cols-2 gap-2">
                  <Input placeholder="temperature,如 0.7" type="number" step="0.1" value={form.temperature}
                    onChange={(e) => setForm({ ...form, temperature: parseFloat(e.target.value) || 0 })} />
                  <Input placeholder="max_tokens,如 4096" type="number" value={form.max_tokens}
                    onChange={(e) => setForm({ ...form, max_tokens: parseInt(e.target.value, 10) || 0 })} />
                </div>
              )}
              <div className="flex gap-2">
                <Button size="sm" onClick={saveConfig}
                  disabled={!form.name.trim() || !form.base_url.trim() || !form.model.trim()}>
                  {form.id ? <><Check className="size-3.5" /> 保存</> : <><Plus className="size-3.5" /> 添加</>}
                </Button>
                <Button size="sm" variant="outline" onClick={testConfig}
                  disabled={testing || !form.base_url.trim() || !form.model.trim()}>
                  <Zap className="size-3.5" /> {testing ? "测试中…" : "测试连接"}
                </Button>
                {form.id && (
                  <Button size="sm" variant="ghost" onClick={() => setForm(emptyCfg)}>取消编辑</Button>
                )}
              </div>
            </CardContent>
          </Card>
        </section>
      </div>
    </div>
  )
}
