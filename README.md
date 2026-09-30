# digestly

订阅 + 采集 + 标注 一体化阅读器。Go monorepo,桌面端 Tauri v2 壳,移动端 gomobile(规划中)。

## 架构

```
┌────────────────────────────────────────────┐
│ apps/web     React + Vite + shadcn/ui      │  纯前端，只走 HTTP API
├────────────────────────────────────────────┤
│ apps/server  Go HTTP 外壳 (:3210)          │  开发期独立跑 / 桌面端 sidecar
├────────────────────────────────────────────┤
│ core/        业务核心(gomobile 就绪)        │
│  ├─ collector  RSS/Atom/JSONFeed + JS 脚本  │
│  ├─ store      SQLite(FTS5 全文 + 标签)     │
│  ├─ pipeline   标注管线(来源/摘要/关键词)    │
│  └─ scheduler  周期采集调度                  │
├────────────────────────────────────────────┤
│ apps/desktop Tauri v2(仅壳 + sidecar 编排) │
└────────────────────────────────────────────┘
```

文章状态机: `pending`(已采集) → 标注管线 → `processed` / `failed`。

## 开发(不碰 Tauri)

```bash
# 终端 1: Go 后端
just dev-server        # = go run ./apps/server -addr 127.0.0.1:3210 -data data

# 终端 2: 前端(vite proxy /api -> :3210)
just dev-web           # = pnpm -C apps/web dev
```

浏览器打开 vite 输出的地址即可完整联调。

## 桌面端

```bash
just build-desktop     # web build -> Go sidecar -> tauri build
just dev-desktop       # tauri dev(会同时拉起前端 dev server)
```

## 采集器扩展

- 内置: `rss` / `atom` / `jsonfeed`
- **订阅源市场**: [digestly-collectors](https://github.com/juncaifeng/digestly-collectors),
  `GET /api/market` 浏览、`POST /api/market/install` 一键安装(下载脚本→热重载→自动建 feed)
- JS 脚本: 手动放入 `data/collectors/*.js` 亦可,约定见市场仓库 README
- Go 插件(go-plugin 子进程): roadmap

## 分支 / CI

与 mihoyo-sub 同一套 release 管理(单个 .github/workflows/build.yml):

| 触发 | 产出(Releases 页直接可见) |
|---|---|
| 任意分支 push | 滚动 Pre-release `<分支名>-latest`,覆盖更新,随时取最新包 |
| `v*` tag push | 正式 Release `<分支名>-<tag>`(Latest) |

每次 Release 附带: Windows msi / macOS dmg / Linux AppImage+deb / Android debug APK。
滚动 tag 带 `-latest` 后缀是为了避免与同名分支产生 git ref 歧义。

发版流程: `just release v0.1.0`(自动合并 main-dev → main-tags、打 tag、推送)。

## 环境要求

Go ≥ 1.26(或低版本 + `GOTOOLCHAIN=auto`)、Node 22、pnpm 12、Rust stable。
