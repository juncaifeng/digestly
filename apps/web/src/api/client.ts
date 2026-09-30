// 开发态: vite proxy /api -> :3210，用相对路径。
// 桌面生产态: Tauri webview 加载本地 dist，直连 sidecar 端口。
const BASE = import.meta.env.DEV ? "" : `http://127.0.0.1:${(window as any).__DIGESTLY_PORT__ ?? 3210}`

export interface Feed {
  id: number
  title: string
  url: string
  collector: string
  config: string
  interval_min: number
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
  tags?: string[]
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
  refreshFeed: (id: number) => req<void>(`/feeds/${id}/refresh`, { method: "POST" }),
  listItems: (params: Record<string, string | number | undefined> = {}) => {
    const qs = new URLSearchParams()
    for (const [k, v] of Object.entries(params))
      if (v !== undefined && v !== "") qs.set(k, String(v))
    return req<Item[]>(`/items?${qs}`)
  },
  markRead: (id: number, read: boolean) =>
    req<void>(`/items/${id}/read`, { method: "POST", body: JSON.stringify({ read }) }),
  addTag: (id: number, tag: string) =>
    req<void>(`/items/${id}/tags`, { method: "POST", body: JSON.stringify({ tag }) }),
  listCollectors: () => req<string[]>("/collectors"),
  runPipeline: () => req<{ processed: number }>("/pipeline/run", { method: "POST" }),
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

export const marketApi = {
  list: () => req<{ collectors: MarketEntry[] }>("/market"),
  install: (name: string) =>
    req<InstallResult>("/market/install", { method: "POST", body: JSON.stringify({ name }) }),
}
