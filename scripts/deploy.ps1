param(
    [string]$Addr = ":8080",
    [string]$MemoryDir = "data/memory",
    [string]$OutDir = "dist/relationship-agent-runtime"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$out = Join-Path $root $OutDir
$goExe = Get-Command go -ErrorAction SilentlyContinue

if ($goExe) {
    $go = $goExe.Source
} elseif (Test-Path "C:\Program Files\Go\bin\go.exe") {
    $go = "C:\Program Files\Go\bin\go.exe"
} else {
    throw "Go executable was not found. Install Go 1.22+ or add go.exe to PATH."
}

New-Item -ItemType Directory -Force -Path $out | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $out "data") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $out "logs") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $out "web") | Out-Null

$env:GOCACHE = Join-Path $root ".gocache"
& $go test ./...
& $go build -o (Join-Path $out "relationship-agent-runtime.exe") ./cmd/server
& $go build -o (Join-Path $out "relationship-agent-cli.exe") ./cmd/cli

Copy-Item (Join-Path $root "README.md") (Join-Path $out "README.md") -Force
Copy-Item (Join-Path $root "docs\ARCHITECTURE.md") (Join-Path $out "ARCHITECTURE.md") -Force
Copy-Item (Join-Path $root "web\index.html") (Join-Path $out "web\index.html") -Force
Copy-Item (Join-Path $root "scripts\setup-openai-key.ps1") (Join-Path $out "setup-openai-key.ps1") -Force

@"
ADDR=$Addr
MEMORY_DIR=$MemoryDir
"@ | Set-Content -Encoding UTF8 (Join-Path $out ".env")

@"
`$ErrorActionPreference = "Stop"
`$here = Split-Path -Parent `$MyInvocation.MyCommand.Path
Set-Location `$here

`$env:ADDR = "$Addr"
`$env:MEMORY_DIR = "$MemoryDir"
New-Item -ItemType Directory -Force -Path `$env:MEMORY_DIR | Out-Null

Write-Host "Relationship Agent Runtime listening on `$env:ADDR"
Write-Host "Memory directory: `$env:MEMORY_DIR"
Write-Host "Health: http://localhost$Addr/health"
Write-Host "Web UI: http://localhost$Addr/"
Write-Host ""
Write-Host "This is the API server window. Do NOT type chat messages here."
Write-Host "For direct chat, open another PowerShell and run: .\chat.ps1"
Write-Host ""
Write-Host "Press Ctrl+C to stop."
& "`$here\relationship-agent-runtime.exe"
"@ | Set-Content -Encoding UTF8 (Join-Path $out "run-server.ps1")

@"
`$ErrorActionPreference = "Stop"
`$here = Split-Path -Parent `$MyInvocation.MyCommand.Path
Set-Location `$here
`$env:ADDR = ":8081"
`$env:MEMORY_DIR = "data/memory-api-8081"
New-Item -ItemType Directory -Force -Path `$env:MEMORY_DIR | Out-Null
Write-Host "Relationship Agent Runtime API server on :8081"
Write-Host "Health: http://localhost:8081/health"
Write-Host "Web UI: http://localhost:8081/"
Write-Host "This is the API server window. Do NOT type chat messages here."
Write-Host "For direct chat, open another PowerShell and run: .\chat.ps1"
Write-Host ""
& "`$here\relationship-agent-runtime.exe"
"@ | Set-Content -Encoding UTF8 (Join-Path $out "api-8081.ps1")

@"
`$ErrorActionPreference = "Stop"
`$here = Split-Path -Parent `$MyInvocation.MyCommand.Path
Set-Location `$here
chcp 65001 | Out-Null
[Console]::InputEncoding = [System.Text.UTF8Encoding]::new()
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new()
Write-Host "Relationship Agent Runtime Chat"
Write-Host "This is the direct chat window. Type Chinese messages here."
Write-Host "Tips: /trace toggles runtime trace, /exit quits."
Write-Host ""
& "`$here\relationship-agent-cli.exe" --user interactive-user --memory data\memory
"@ | Set-Content -Encoding UTF8 (Join-Path $out "chat.ps1")

@"
`$ErrorActionPreference = "Stop"
Invoke-RestMethod -Uri "http://localhost$Addr/health" -Method GET | ConvertTo-Json -Compress
"@ | Set-Content -Encoding UTF8 (Join-Path $out "health.ps1")

@"
`$ErrorActionPreference = "Stop"
`$OutputEncoding = [System.Text.UTF8Encoding]::new()
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new()

function Invoke-Chat([string]`$message) {
    `$body = @{
        user_id = "review-demo"
        message = `$message
    } | ConvertTo-Json

    Invoke-RestMethod -Uri "http://localhost$Addr/chat" -Method POST -ContentType "application/json; charset=utf-8" -Body `$body |
        ConvertTo-Json -Depth 10
}

function From-Utf8Base64([string]`$value) {
    [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String(`$value))
}

Invoke-Chat (From-Utf8Base64 "5oiR5Y+r5p6X5aSP77yM5oiR5Zyo5LiK5rW377yM5piv5Lqn5ZOB57uP55CG44CC5oiR5Zac5qyi5aSc6LeR77yM5LiN5Zac5qyi5aSq5Ya35Yaw5Yaw55qE5Zue5aSN44CC")
Invoke-Chat (From-Utf8Base64 "5pyA6L+R5LiL5ZGo6KaB6Z2i6K+V77yM5oiR5pyJ54K554Sm6JmR77yM5biM5pyb5L2g5rip5p+U5LiA54K577yM5L2G5Lmf57uZ5oiR55u05o6l55qE5bu66K6u44CC")
Invoke-Chat (From-Utf8Base64 "5YW25a6e5oiR5bey57uP5pCs5Yiw5rex5Zyz5LqG77yM5pyA6L+R5L2c5oGv5LiA6Iis5Lya54as5aSc44CC")
Invoke-RestMethod -Uri "http://localhost$Addr/profile/review-demo" -Method GET | ConvertTo-Json -Depth 10
"@ | Set-Content -Encoding UTF8 (Join-Path $out "demo.ps1")

Write-Host "Deployment package created:"
Write-Host $out
Write-Host ""
Write-Host "Start it with:"
Write-Host "powershell -ExecutionPolicy Bypass -File `"$out\run-server.ps1`""
