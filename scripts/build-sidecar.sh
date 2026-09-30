#!/usr/bin/env bash
# 按当前 Rust target triple 构建 Go sidecar 并放入 Tauri binaries/
set -euo pipefail
cd "$(dirname "$0")/../apps/server"
TRIPLE=$(rustc -vV | sed -n 's|host: ||p')
EXT=""
[[ "$TRIPLE" == *windows* ]] && EXT=".exe"
mkdir -p ../desktop/src-tauri/binaries
go build -ldflags "-s -w" -o "../desktop/src-tauri/binaries/digestly-server-${TRIPLE}${EXT}" .
echo "sidecar: ../desktop/src-tauri/binaries/digestly-server-${TRIPLE}${EXT}"
