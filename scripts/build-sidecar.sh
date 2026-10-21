#!/usr/bin/env bash
# 按当前 Rust target triple 构建 Go sidecar(server + cli)并放入 Tauri binaries/
set -euo pipefail
cd "$(dirname "$0")/.."
TRIPLE=$(rustc -vV | sed -n 's|host: ||p')
EXT=""
[[ "$TRIPLE" == *windows* ]] && EXT=".exe"
mkdir -p apps/desktop/src-tauri/binaries
go build -C apps/server -ldflags "-s -w" -o "../../apps/desktop/src-tauri/binaries/digestly-server-${TRIPLE}${EXT}" .
go build -C apps/cli -ldflags "-s -w" -o "../../apps/desktop/src-tauri/binaries/digestly-cli-${TRIPLE}${EXT}" .
echo "sidecar: apps/desktop/src-tauri/binaries/digestly-{server,cli}-${TRIPLE}${EXT}"
