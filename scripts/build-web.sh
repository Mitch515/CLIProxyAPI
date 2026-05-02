#!/usr/bin/env bash
# Build the SvelteKit dashboard and copy it to internal/web/dist so the Go
# binary can pick it up via go:embed on the next `go build`.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/web"

if [ ! -d node_modules ]; then
  pnpm install --frozen-lockfile
fi
pnpm build

DEST="$ROOT/internal/web/dist"
rm -rf "$DEST"
mkdir -p "$DEST"
cp -r "$ROOT/web/build/." "$DEST/"
echo "Dashboard built into $DEST"
