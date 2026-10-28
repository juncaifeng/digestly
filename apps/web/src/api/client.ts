// 开发态: vite proxy /api -> :3210，用相对路径。
// 桌面生产态: Tauri webview 加载本地 dist，直连 sidecar 端口。
const BASE = import.meta.env.DEV ? "" : `http://127.0.0.1:${(window as any).__DIGESTLY_PORT__ ?? 3210}`

export interface Feed {
  id: number
  title: string
  column_id: number | null
  url: string
  collector: string
  config: string
  interval_min: number
  max_items: number
  unread_count: number
}

export type ItemStatus = "pending" | "processed" | "failed"

export interface Item {
  id: number
  feed_id: number
  guid: string
  title: string
  link: string
  author: string
  content: string
  summary: string
  keywords: string
  source: string
  published_at: string | null
  status: ItemStatus
  read: boolean
  archived: boolean
  tags?: string[]
  column_ids?: number[]
}

export interface Column {
  id: number
  name: string
  sort: number
  template_id: number
  feed_count: number
  item_count: number
  created_at: string
}

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}/api${path}`, {
    headers: { "Content-Type": "application/json" },
    ...init,
  })
  if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`)
  if (res.status === 204) return undefined as T
  return res.json()
}

export const api = {
  listFeeds: () => req<Feed[]>("/feeds"),
  addFeed: (f: { title: string; url: string; collector?: string }) =>
    req<Feed>("/feeds", { method: "POST", body: JSON.stringify(f) }),
  deleteFeed: (id: number) => req<void>(`/feeds/${id}`, { method: "DELETE" }),
  updateFeed: (id: number, patch: { max_items?: number }) =>
    req<{ max_items: number }>(`/feeds/${id}`, {
      method: "PATCH",
      body: JSON.stringify(patch),
    }),
  refreshFeed: (id: number) => req<void>(`/feeds/${id}/refresh`, { method: "POST" }),
  listItems: (params: Record<string, string | number | undefined> = {}) => {
    const qs = new URLSearchParams()
    for (const [k, v] of Object.entries(params))
      if (v !== undefined && v !== "") qs.set(k, String(v))
    return req<Item[]>(`/items?${qs}`)
  },
  getItem: (id: number) => req<Item>(`/items/${id}`),
  markRead: (id: number, read: boolean) =>
    req<void>(`/items/${id}/read`, { method: "POST", body: JSON.stringify({ read }) }),
  archiveItem: (id: number, archived: boolean) =>
    req<void>(`/items/${id}/archive`, { method: "POST", body: JSON.stringify({ archived }) }),
  archiveFeed: (id: number, scope: "read" | "all") =>
    req<{ archived: number }>(`/feeds/${id}/archive`, { method: "POST", body: JSON.stringify({ scope }) }),
  addTag: (id: number, tag: string) =>
    req<void>(`/items/${id}/tags`, { method: "POST", body: JSON.stringify({ tag }) }),
  listCollectors: () => req<string[]>("/collectors"),
  runPipeline: () => req<{ processed: number }>("/pipeline/run", { method: "POST" }),
  retryItem: (id: number) =>
    req<{ id: number; status: ItemStatus }>(`/items/${id}/retry`, { method: "POST" }),
  retryPipeline: () =>
    req<{ retried: number; processed: number }>("/pipeline/retry", { method: "POST" }),
}

export interface MarketEntry {
  name: string
  title: string
  description: string
  url: string
  source_url: string
  tags: string[]
  script_version: number
  config_schema: Record<string, { type: string; default: unknown }>
  installed: boolean
  installed_version: number
  update_available: boolean
}

export interface InstallResult {
  installed: string
  feed_created: boolean
  feed_id?: number
}

export interface MarketListResult {
  collectors: MarketEntry[]
  stale: boolean
  last_fetch: string
}

export interface SourceFeed {
  name: string
  title: string
  description: string
  feed_url: string
  site_url: string
  tags: string[]
  catalog: string
}

export interface MountedSource {
  id: number
  url: string
  title: string
  enabled: boolean
  feed_count: number
  last_fetch: string
  feeds: SourceFeed[]
  parse_err?: string
  fetched_at: string
}

export const columnApi = {
  list: () => req<Column[]>("/columns"),
  create: (name: string) =>
    req<{ id: number; name: string }>("/columns", { method: "POST", body: JSON.stringify({ name }) }),
  update: (id: number, patch: { name?: string; template_id?: number }) =>
    req<void>(`/columns/${id}`, { method: "PATCH", body: JSON.stringify(patch) }),
  rename: (id: number, name: string) =>
    req<void>(`/columns/${id}`, { method: "PATCH", body: JSON.stringify({ name }) }),
  remove: (id: number) => req<void>(`/columns/${id}`, { method: "DELETE" }),
  setFeedColumn: (feedId: number, columnId: number | null) =>
    req<void>(`/feeds/${feedId}/column`, { method: "POST", body: JSON.stringify({ column_id: columnId }) }),
  favorite: (itemId: number, columnId: number) =>
    req<void>(`/items/${itemId}/columns`, { method: "POST", body: JSON.stringify({ column_id: columnId }) }),
  unfavorite: (itemId: number, columnId: number) =>
    req<void>(`/items/${itemId}/columns/${columnId}`, { method: "DELETE" }),
}

export interface LLMConfig {
  id: number
  name: string
  base_url: string
  api_key: string // 列表接口返回掩码;提交时空串 = 不修改
  model: string
  temperature: number
  max_tokens: number
  is_default: boolean
  created_at: string
  updated_at: string
}

export interface ArticleTemplate {
  id: number
  name: string
  prompt: string
  built_in: boolean
  created_at: string
}

export interface Article {
  id: number
  creation_id: number
  template_id: number
  title: string
  content: string
  item_ids?: number[]
  created_at: string
  updated_at: string
}

export interface CreationSource {
  kind: "column" | "feed"
  ref_id: number
}

export interface Creation {
  id: number
  name: string
  group_name: string
  template_id: number
  sources?: CreationSource[]
  created_at: string
}

export interface ModelPreset {
  id: string
  label: string
  context: string
  max_output: string
  thinking: string
  vision: boolean
  pricing: string
  concurrency: number
}

export interface ProviderPreset {
  name: string
  base_url: string
  docs: string
  note: string
  models: ModelPreset[]
}

// MCP 入口地址(与 web 同一 server 的 /mcp; dev 下 vite 只代理 /api,故用绝对地址)
export const mcpUrl = () => `http://127.0.0.1:${(window as any).__DIGESTLY_PORT__ ?? 3210}/mcp`

export const mcpApi = {
  url: mcpUrl,
  // 探测 MCP 服务状态: initialize + tools/list
  status: async () => {
    const url = mcpUrl()
    const rpc = async (id: number, method: string, params: unknown = {}) => {
      const res = await fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ jsonrpc: "2.0", id, method, params }),
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      return res.json()
    }
    const init = await rpc(1, "initialize", {
      protocolVersion: "2025-06-18", capabilities: {},
      clientInfo: { name: "digestly-web", version: "0" },
    })
    const list = await rpc(2, "tools/list")
    const tools: string[] = (list.result?.tools ?? []).map((t: { name: string }) => t.name)
    return { protocol: (init.result?.protocolVersion ?? "") as string, tools }
  },
}

export const llmApi = {
  presets: () => req<ProviderPreset[]>("/llm/presets"),
  list: () => req<LLMConfig[]>("/llm/configs"),
  create: (c: Partial<LLMConfig>) =>
    req<LLMConfig>("/llm/configs", { method: "POST", body: JSON.stringify(c) }),
  update: (id: number, c: Partial<LLMConfig>) =>
    req<void>(`/llm/configs/${id}`, { method: "PATCH", body: JSON.stringify(c) }),
  remove: (id: number) => req<void>(`/llm/configs/${id}`, { method: "DELETE" }),
  setDefault: (id: number) =>
    req<void>(`/llm/configs/${id}/default`, { method: "POST" }),
  test: (body: { id?: number; base_url?: string; api_key?: string; model?: string }) =>
    req<{ reply: string }>("/llm/test", { method: "POST", body: JSON.stringify(body) }),
}

export const templateApi = {
  list: () => req<ArticleTemplate[]>("/templates"),
  create: (t: { name: string; prompt: string }) =>
    req<ArticleTemplate>("/templates", { method: "POST", body: JSON.stringify(t) }),
  update: (id: number, t: { name: string; prompt: string }) =>
    req<void>(`/templates/${id}`, { method: "PATCH", body: JSON.stringify(t) }),
  remove: (id: number) => req<void>(`/templates/${id}`, { method: "DELETE" }),
}

export const articleApi = {
  list: (creation_id: number) => req<Article[]>(`/articles?creation_id=${creation_id}`),
  get: (id: number) => req<Article>(`/articles/${id}`),
  update: (id: number, a: { title: string; content: string }) =>
    req<void>(`/articles/${id}`, { method: "PATCH", body: JSON.stringify(a) }),
  remove: (id: number) => req<void>(`/articles/${id}`, { method: "DELETE" }),
}

export interface CreationPlan {
  creation: Creation
  template: ArticleTemplate
  feed_ids: number[]
  force: boolean
  item_ids: number[]
  items: Item[]
  history_titles: string[]
  prompt: string
}

export const creationApi = {
  list: () => req<Creation[]>("/creations"),
  create: (c: { name: string; group_name?: string }) =>
    req<Creation>("/creations", { method: "POST", body: JSON.stringify(c) }),
  update: (id: number, c: Partial<Creation>) =>
    req<void>(`/creations/${id}`, { method: "PATCH", body: JSON.stringify(c) }),
  remove: (id: number) => req<void>(`/creations/${id}`, { method: "DELETE" }),
  generate: (id: number, template_id?: number, force?: boolean) =>
    req<Article>(`/creations/${id}/generate`, {
      method: "POST",
      body: JSON.stringify({ template_id, force }),
    }),
  plan: (id: number, template_id?: number, force?: boolean) =>
    req<CreationPlan>(`/creations/${id}/plan`, {
      method: "POST",
      body: JSON.stringify({ template_id, force }),
    }),
}

export const marketApi = {
  // 第三方市场源(订阅源清单)
  listSources: () => req<{ sources: MountedSource[] }>("/market/sources"),
  addSource: (url: string, title = "") =>
    req<{ id: number; source: string; count: number; feeds: SourceFeed[] }>("/market/sources", {
      method: "POST",
      body: JSON.stringify({ url, title }),
    }),
  // 只解析不入库: 预览第三方源里的订阅源清单
  parseSource: (url: string) =>
    req<{ source: string; count: number; feeds: SourceFeed[] }>("/market/sources/parse", {
      method: "POST",
      body: JSON.stringify({ url }),
    }),
  removeSource: (id: number) => req<void>(`/market/sources/${id}`, { method: "DELETE" }),
  refreshSource: (id: number) =>
    req<{ count: number }>(`/market/sources/${id}/refresh`, { method: "POST" }),
  // waitReady=true 时首轮加载挂起等后台预热完成(上限 5s)
  list: (waitReady = false) => req<MarketListResult>(`/market${waitReady ? "?wait_ready=1" : ""}`),
  refresh: () => req<{ refreshed: boolean; collectors: number }>("/market/refresh", { method: "POST" }),
  install: (name: string) =>
    req<InstallResult>("/market/install", { method: "POST", body: JSON.stringify({ name }) }),
}
