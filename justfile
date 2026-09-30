# digestly 日常开发命令(需要 just: https://github.com/casey/just)

# 前后端联调(不碰 Tauri): 开两个终端分别跑 dev-server / dev-web
dev-server:
    go run ./apps/server -addr 127.0.0.1:3210 -data data

dev-web:
    pnpm -C apps/web dev

# 桌面端(Tauri dev，会自动拉起前端 dev server)
dev-desktop:
    pnpm -C apps/desktop tauri dev

# 产出桌面安装包: 前端 build -> sidecar -> tauri build
build-desktop:
    pnpm -C apps/web build
    ./scripts/build-sidecar.sh
    pnpm -C apps/desktop tauri build

# Go 编译检查
check:
    cd core && go build ./...
    cd apps/server && go build ./...
