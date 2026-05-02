# Probe each Codex account with each candidate model. For each account:
#   - try each model in order
#   - first model that warmup-fires successfully wins → PATCH it as warmup_model
#   - if all attempts return token_expired → record for deletion
#   - if all attempts return model_not_supported → record "needs manual override"

param(
  [string]$Base  = 'http://127.0.0.1:18317',
  [string]$Token = 'dashboard-dev-token'
)

$ErrorActionPreference = 'Stop'
$auth = @{ Authorization = "Bearer $Token" }

# Cheapest first; models the proxy exposes for OpenAI/Codex.
$candidates = @(
  'gpt-5.1-codex-mini',
  'gpt-5-codex-mini',
  'gpt-5.1',
  'gpt-5',
  'gpt-5.3-codex-spark',
  'gpt-5.2-codex',
  'gpt-5.1-codex',
  'gpt-5.2',
  'gpt-5.1-codex-max',
  'gpt-5-codex',
  'gpt-5.3-codex'
)

$resp = Invoke-RestMethod -Uri "$Base/v0/management/accounts" -Headers $auth
$codex = $resp.accounts | Where-Object { $_.provider -eq 'codex' }

$results = @()

foreach ($acct in $codex) {
  $id = $acct.id
  Write-Host ""
  Write-Host "=== $($acct.label)  [$id] ===" -ForegroundColor Cyan
  $winner = $null
  $expired = $false
  $lastErr = ''

  foreach ($m in $candidates) {
    Write-Host "  trying $m … " -NoNewline
    $patchBody = @{ warmup_model = $m } | ConvertTo-Json -Compress
    Invoke-RestMethod -Uri "$Base/v0/management/accounts/$([uri]::EscapeDataString($id))" `
      -Method Patch -Headers $auth -ContentType 'application/json' -Body $patchBody | Out-Null
    $r = Invoke-RestMethod -Uri "$Base/v0/management/accounts/$([uri]::EscapeDataString($id))/warmup" `
      -Method Post -Headers $auth
    $last = $r.last
    if ($last.ok) {
      Write-Host "OK" -ForegroundColor Green
      $winner = $m
      break
    }
    $lastErr = ($last.error -as [string])
    if ($lastErr -match 'token_expired|authentication token is expired') {
      Write-Host "TOKEN EXPIRED" -ForegroundColor Red
      $expired = $true
      break
    }
    if ($lastErr -match 'is not supported when using Codex') {
      Write-Host 'model not supported' -ForegroundColor Yellow
    } else {
      $short = $lastErr.Substring(0, [Math]::Min(80, $lastErr.Length))
      Write-Host "other: $short" -ForegroundColor Yellow
    }
  }

  $results += [pscustomobject]@{
    Id      = $id
    Email   = $acct.email
    Winner  = $winner
    Expired = $expired
    LastErr = if ($winner) { '' } else { $lastErr }
  }
}

Write-Host ""
Write-Host "============== summary ==============" -ForegroundColor Cyan
$results | Format-Table -AutoSize Email, Winner, Expired

# Persist for the caller to act on
$results | ConvertTo-Json -Depth 5 | Set-Content -Path "$env:TEMP\codex-probe.json"
Write-Host ""
Write-Host "wrote $env:TEMP\codex-probe.json"
