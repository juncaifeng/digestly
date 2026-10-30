import { useEffect, useState } from "react"
import { Copy, Download, Plus, RefreshCw, Save, Zap } from "lucide-react"

import { ingestApi, mcpApi, opmlApi } from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Input } from "@/components/ui/input"

// 设置页: MCP 服务 / 订阅导入导出 / 采集限制。
export function SettingsView() {
  const [error, setError] = useState("")
  const [notice, setNotice] = useState("")

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
    ingestApi.get().then(setIngest).catch(() => {})
  }
  useEffect(() => { void load() }, [])

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
                即可直接读取 digestly 的订阅源与文章(id + 原文),后续加工交给你的专门工具。
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
      </div>
    </div>
  )
}
