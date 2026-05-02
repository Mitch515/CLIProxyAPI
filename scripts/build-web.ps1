# Build the SvelteKit dashboard and copy it to internal/web/dist so the Go
# binary can pick it up via go:embed on the next `go build`.
$ErrorActionPreference = 'Stop'

$root = Resolve-Path (Join-Path $PSScriptRoot '..')
Set-Location (Join-Path $root 'web')

if (-not (Test-Path 'node_modules')) {
  pnpm install --frozen-lockfile
}
pnpm build

$dest = Join-Path $root 'internal\web\dist'
if (Test-Path $dest) { Remove-Item -Recurse -Force $dest }
New-Item -ItemType Directory -Path $dest | Out-Null
Copy-Item -Recurse (Join-Path $root 'web\build\*') $dest

Write-Output "Dashboard built into $dest"
