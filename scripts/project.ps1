param([ValidateSet('run','fmt','check','test','build','lint','vuln','dev','migrate-status','migrate-up')][string]$Task = 'check')
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $projectRoot
try {
  switch ($Task) {
    'run' { & go run ./cmd/hadalbot -config configs/development.yaml }
    'migrate-status' { & go run ./cmd/migrate -command status }
    'migrate-up' { & go run ./cmd/migrate -command up }
    default { & go run ./cmd/devtools $Task }
  }
  if ($LASTEXITCODE -ne 0) { throw "任务 $Task 失败，退出码 $LASTEXITCODE" }
} finally { Pop-Location }
