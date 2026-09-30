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
- JS 脚本: 放入 `data/collectors/*.js`,feed 的 collector 填 `js:<脚本名>`,
  约定见 [packages/collectors/README.md](packages/collectors/README.md)
- Go 插件(go-plugin 子进程): roadmap

## 分支 / CI

| 分支 | 用途 | CI |
|---|---|---|
| `*-dev` | 日常开发(默认分支 `main-dev`) | push 即构建桌面三平台快照包(artifact) |
| `*-tags` | 发行(合并自对应 dev) | push 构建 + `v*` tag 发布 GitHub Release(含 Android debug APK) |

发版流程: `just release v0.1.0`(自动合并 main-dev → main-tags、打 tag、推送),GitHub Release 会附上三平台安装包 + Android APK。

## 制品下载

| 场景 | 位置 |
|---|---|
| dev 快照包 | Actions → 对应 run → Artifacts(90 天有效),或 `gh run download <run-id>` |
| release 分支构建(未打 tag) | Actions → Artifacts(安装包,非完整 bundle) |
| 正式发行 | Releases 页面(v* tag 触发): msi / dmg / AppImage / deb / apk |

## 环境要求

Go ≥ 1.26(或低版本 + `GOTOOLCHAIN=auto`)、Node 22、pnpm 12、Rust stable。
