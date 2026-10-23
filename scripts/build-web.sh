#!/bin/sh
# Builds Lexington's web version into dist/web (or $1): the WebAssembly,
# Go's wasm_exec.js and the page in web/. Serve the folder over http(s).
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
out=${1:-$root/dist/web}
mkdir -p "$out"
GOOS=js GOARCH=wasm go build -C "$root" -ldflags="-s -w" -o "$out/lexington.wasm" ./cmd/lexington-web
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$out/"
cp -r "$root"/web/. "$out/"
echo "web version in $out ($(du -h "$out/lexington.wasm" | cut -f1) of WebAssembly)"
