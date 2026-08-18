<#
.SYNOPSIS
Test-only fixture: asserts the version contract and runs the documented
first-use workflow (open -> note -> status -> handoff) in a temporary git
repository against a devlog.exe binary. With -RunInstallerHarness, also runs
the existing 30-scenario installer harness (scripts/install.tests.ps1) in a
fresh PowerShell process and reports its result. Used by
.github/workflows/preflight.yml (archive-matrix and powershell-installer
jobs). Not shipped; not part of the devlog binary.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string]$Binary,
    [switch]$RunInstallerHarness
)

$ErrorActionPreference = 'Stop'

function Step { param([string]$Name) Write-Host "== $Name" }

if (-not (Test-Path -LiteralPath $Binary -PathType Leaf)) {
    Write-Host "FAIL: binary not found: $Binary"
    exit 1
}
$Binary = (Resolve-Path -LiteralPath $Binary).Path

Step "version output"
$ver = & $Binary --version 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host 'FAIL: --version exited non-zero'
    exit 1
}
$verText = ($ver -join "`n")
if ($verText -notmatch 'version:\s*[0-9]+\.[0-9]+\.[0-9]+') {
    Write-Host 'FAIL: --version did not report a semantic version:'
    Write-Host $verText
    exit 1
}
if ($verText -match 'version:\s*v[0-9]') {
    Write-Host 'FAIL: --version reports a leading v:'
    Write-Host $verText
    exit 1
}
Write-Host ("OK: " + [regex]::Match($verText, 'version:\s*\S+').Value)

Step "documented workflow"
$repo = Join-Path ([System.IO.Path]::GetTempPath()) ('devlog-preflight-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $repo | Out-Null
$marker = 'PREFLIGHT-NOTE-' + [guid]::NewGuid().ToString('N')
try {
    Push-Location $repo
    git init -q -b main
    git config user.name 'Preflight Agent'
    git config user.email 'preflight@devlog.local'

    & $Binary open 'Preflight workflow test' *> $null
    if ($LASTEXITCODE -ne 0) { throw 'devlog open failed' }
    & $Binary note -m $marker *> $null
    if ($LASTEXITCODE -ne 0) { throw 'devlog note failed' }

    $status = & $Binary status 2>&1
    if ($LASTEXITCODE -ne 0) { throw 'devlog status failed' }
    $statusText = $status -join "`n"
    if ($statusText -notmatch '\(active\)') { throw 'status does not report an active session' }
    if ($statusText -notmatch [regex]::Escape($marker)) { throw 'status does not show the note' }

    $handoff = & $Binary handoff 2>&1
    if ($LASTEXITCODE -ne 0) { throw 'devlog handoff failed' }
    $pathMatch = [regex]::Match(($handoff -join "`n"), '\.devlog/handoffs/[0-9TZ-]+\.md')
    if (-not $pathMatch.Success) { throw 'handoff output has no file path' }
    $path = Join-Path $repo $pathMatch.Value
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "handoff file not found: $path" }
    $handoffText = Get-Content -LiteralPath $path -Raw
    if ($handoffText -notmatch [regex]::Escape($marker)) { throw 'handoff does not contain the note' }

    $sessions = @(Get-ChildItem -LiteralPath (Join-Path $repo '.devlog\sessions') -Filter '*.md' -File -ErrorAction Stop)
    if ($sessions.Count -lt 1) { throw 'no session file written' }
}
finally {
    Pop-Location
    Remove-Item -LiteralPath $repo -Recurse -Force -ErrorAction SilentlyContinue
}
Write-Host 'OK: open -> note -> status -> handoff complete in temp repo'

if ($RunInstallerHarness) {
    Step "installer harness (30 scenarios)"
    $harness = Join-Path (Split-Path -Parent $PSScriptRoot) 'install.tests.ps1'
    $pwshPath = (Get-Process -Id $PID).Path
    $proc = Start-Process -FilePath $pwshPath -ArgumentList @('-NoProfile', '-File', $harness) -Wait -PassThru
    if ($proc.ExitCode -ne 0) {
        Write-Host 'FAIL: installer harness reported failures'
        exit 1
    }
    Write-Host 'OK: installer harness passed'
}

Write-Host 'PASS: documented workflow verified'