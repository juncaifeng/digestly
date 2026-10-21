import { useEffect, useState } from "react"
import { Check, Cpu, Pencil, Plus, Star, Trash2, Zap } from "lucide-react"

import { llmApi, type AgentPreset, type LLMConfig, type ProviderPreset } from "@/api/client"
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
  const emptyCfg = { name: "", kind: "chat" as "chat" | "agent", command: "", base_url: "", api_key: "", model: "", temperature: 0.7, max_tokens: 4096 }
  const [form, setForm] = useState<typeof emptyCfg & { id?: number }>(emptyCfg)
  const [testing, setTesting] = useState(false)

  // 供应商预设(内置 YAML + data/llm_providers 用户覆盖)
  const [presets, setPresets] = useState<ProviderPreset[]>([])
  const [presetName, setPresetName] = useState("")
  // 外部 Agent 运行时预设(内置 YAML + data/llm_agents 用户覆盖)
  const [agentPresets, setAgentPresets] = useState<AgentPreset[]>([])
  const [agentPresetName, setAgentPresetName] = useState("")

  const load = () => {
    llmApi.list().then(setConfigs).catch((e) => setError(`LLM 配置加载失败: ${e.message}`))
    llmApi.presets().then(setPresets).catch(() => {})
    llmApi.agentPresets().then(setAgentPresets).catch(() => {})
  }
  useEffect(() => { void load() }, [])

  const preset = presets.find((p) => p.name === presetName)
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

  const applyAgentPreset = (name: string) => {
    setAgentPresetName(name)
    const p = agentPresets.find((x) => x.name === name)
    if (!p) return
    setForm((f) => ({
      ...f,
      kind: "agent",
      name: f.name || p.name,
      command: p.command,
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
      const r = await llmApi.test({ id: form.id, kind: form.kind, command: form.command, base_url: form.base_url, api_key: form.api_key, model: form.model })
      setNotice(`连接成功,模型回复: ${r.reply}`)
    } catch (e) {
      setError(`连接失败: ${(e as Error).message}`)
    } finally {
      setTesting(false)
    }
  }

  return (
    <div className="flex-1 overflow-y-auto p-4">
      <div className="max-w-3xl flex flex-col gap-6">
        {error && <p className="text-sm text-destructive">{error}</p>}
        {notice && <p className="text-sm text-green-600">{notice}</p>}

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
                    onClick={() => setForm({ id: c.id, name: c.name, kind: c.kind ?? "chat", command: c.command ?? "", base_url: c.base_url, api_key: "", model: c.model, temperature: c.temperature, max_tokens: c.max_tokens })}>
                    <Pencil className="size-3.5" />
                  </Button>
                  <Button variant="ghost" size="icon" className="size-7" title="删除"
                    onClick={() => window.confirm(`删除配置「${c.name}」?`) && llmApi.remove(c.id).then(load)}>
                    <Trash2 className="size-3.5" />
                  </Button>
                </div>
              </CardHeader>
              <CardContent className="text-sm text-muted-foreground flex flex-col gap-0.5">
                {c.kind === "agent" ? (
                  <>
                    <span>外部 Agent · <code className="text-xs">{c.command}</code></span>
                    <span>prompt 经 {c.command.includes("{prompt_file}") ? "临时文件" : "stdin"} 传入 · 输出取 stdout</span>
                  </>
                ) : (
                  <>
                    <span>{c.base_url} · {c.model}</span>
                    <span>temperature {c.temperature} · max_tokens {c.max_tokens} · key {c.api_key || "(未设置)"}</span>
                  </>
                )}
              </CardContent>
            </Card>
          ))}

          <Card>
            <CardHeader>
              <CardTitle className="text-base">{form.id ? "编辑配置" : "新增配置"}</CardTitle>
              <CardDescription>
                生成后端二选一:直连 OpenAI 兼容 API,或调用本地外部 Agent(dsh / OpenClaw / Claude Code…)
              </CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-2">
              <select
                className="border rounded-md h-9 px-2 text-sm bg-transparent"
                value={form.kind}
                onChange={(e) => setForm({ ...form, kind: e.target.value as "chat" | "agent" })}
              >
                <option value="chat">直连 API(OpenAI 兼容)</option>
                <option value="agent">外部 Agent(dsh / OpenClaw / Claude Code…)</option>
              </select>

              {form.kind === "agent" ? (
                <>
                  {agentPresets.length > 0 && (
                    <select
                      className="border rounded-md h-9 px-2 text-sm bg-transparent"
                      value={agentPresetName}
                      onChange={(e) => applyAgentPreset(e.target.value)}
                    >
                      <option value="">从 Agent 预设填充(DSH / OpenClaw / Claude Code)…</option>
                      {agentPresets.map((p) => (
                        <option key={p.name} value={p.name}>{p.name}</option>
                      ))}
                    </select>
                  )}
                  {agentPresets.find((p) => p.name === agentPresetName)?.note && (
                    <p className="text-xs text-muted-foreground">
                      {agentPresets.find((p) => p.name === agentPresetName)!.note}
                    </p>
                  )}
                  <Input placeholder="命令模板,如 dsh --profile headless(prompt 走 stdin)" value={form.command}
                    onChange={(e) => setForm({ ...form, command: e.target.value })} />
                  <p className="text-xs text-muted-foreground">
                    含 <code>{"{prompt_file}"}</code> 占位时 prompt 写入临时文件替换之(如 openclaw agent exec --message-file {"{prompt_file}"});
                    否则 prompt 走 stdin。生成文本从 stdout 读取。
                  </p>
                </>
              ) : (
                <>
                  {presets.length > 0 && (
                    <select
                      className="border rounded-md h-9 px-2 text-sm bg-transparent"
                      value={presetName}
                      onChange={(e) => applyPreset(e.target.value)}
                    >
                      <option value="">从供应商预设填充(DeepSeek / Kimi / 智谱 / 通义…)…</option>
                      {presets.map((p) => (
                        <option key={p.name} value={p.name}>{p.name}</option>
                      ))}
                    </select>
                  )}
                  {preset?.note && <p className="text-xs text-muted-foreground">{preset.note}</p>}
                </>
              )}

              <div className="grid grid-cols-2 gap-2">
                <Input placeholder="名称,如 DeepSeek" value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })} />
                {form.kind === "chat" && (
                  <Input placeholder="模型,如 deepseek-flash" value={form.model}
                    onChange={(e) => setForm({ ...form, model: e.target.value })} />
                )}
              </div>
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
              {form.kind === "chat" && (
                <>
                  <Input placeholder="Base URL,如 https://api.deepseek.com/v1" value={form.base_url}
                    onChange={(e) => setForm({ ...form, base_url: e.target.value })} />
                  <Input placeholder="API Key" type="password" value={form.api_key}
                    onChange={(e) => setForm({ ...form, api_key: e.target.value })} />
                  <div className="grid grid-cols-2 gap-2">
                    <Input placeholder="temperature,如 0.7" type="number" step="0.1" value={form.temperature}
                      onChange={(e) => setForm({ ...form, temperature: parseFloat(e.target.value) || 0 })} />
                    <Input placeholder="max_tokens,如 4096" type="number" value={form.max_tokens}
                      onChange={(e) => setForm({ ...form, max_tokens: parseInt(e.target.value, 10) || 0 })} />
                  </div>
                </>
              )}
              <div className="flex gap-2">
                <Button size="sm" onClick={saveConfig}
                  disabled={!form.name.trim() || (form.kind === "agent" ? !form.command.trim() : !form.base_url.trim() || !form.model.trim())}>
                  {form.id ? <><Check className="size-3.5" /> 保存</> : <><Plus className="size-3.5" /> 添加</>}
                </Button>
                <Button size="sm" variant="outline" onClick={testConfig}
                  disabled={testing || (form.kind === "agent" ? !form.command.trim() : !form.base_url.trim() || !form.model.trim())}>
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
