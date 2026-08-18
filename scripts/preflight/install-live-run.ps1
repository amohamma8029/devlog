<#
.SYNOPSIS
Test-only fixture: performs one end-to-end live run of scripts/install.ps1
against a locally built fixture release — the real devlog binary built with a
pinned version (the same ldflag goreleaser uses), packaged as zip + checksums,
served over a loopback HTTP listener. No real registry, PATH, install
directory, or GitHub is touched. Used by .github/workflows/preflight.yml
(powershell-installer job) and local preflight runs.
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = (Split-Path -Parent (Split-Path -Parent $PSScriptRoot)),
    [string]$FixtureVersion = '1.0.0',
    [string]$Architecture = 'amd64'
)

$ErrorActionPreference = 'Stop'

$installScript = Join-Path $RepoRoot 'scripts\install.ps1'
if (-not (Test-Path -LiteralPath $installScript -PathType Leaf)) {
    throw "install.ps1 not found at $installScript"
}

function Get-FreePort {
    $tcp = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
    $tcp.Start()
    try { return ([System.Net.IPEndPoint]$tcp.LocalEndpoint).Port }
    finally { $tcp.Stop() }
}

$work = Join-Path ([System.IO.Path]::GetTempPath()) ('devlog-live-run-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $work | Out-Null

try {
    Write-Host '== build fixture binary'
    $exe = Join-Path $work 'devlog.exe'
    & go build -ldflags "-X main.version=$FixtureVersion" -o $exe ./cmd/devlog
    if ($LASTEXITCODE -ne 0) { throw 'fixture build failed' }
    $versionOut = & $exe --version 2>&1
    if (($versionOut -join "`n") -notmatch [regex]::Escape($FixtureVersion)) {
        throw "fixture binary does not report version $FixtureVersion"
    }

    Write-Host '== package fixture release (zip + checksums)'
    $zipName = "devlog_${FixtureVersion}_windows_${Architecture}.zip"
    $checksumName = "devlog_${FixtureVersion}_checksums.txt"
    $fixtureDir = Join-Path $work 'release'
    $releaseDir = Join-Path $fixtureDir ("v$FixtureVersion")
    New-Item -ItemType Directory -Path $releaseDir | Out-Null
    $zipPath = Join-Path $releaseDir $zipName
    Compress-Archive -LiteralPath $exe -DestinationPath $zipPath -CompressionLevel Optimal
    $hash = (Get-FileHash -LiteralPath $zipPath -Algorithm SHA256).Hash.ToLowerInvariant()
    Set-Content -LiteralPath (Join-Path $releaseDir $checksumName) -Value "$hash  $zipName" -Encoding ascii

    Write-Host '== serve fixture release over loopback HTTP'
    $port = Get-FreePort
    $listener = [System.Net.HttpListener]::new()
    $listener.Prefixes.Add("http://127.0.0.1:$port/")
    $listener.Start()
    $handler = {
        param($Listener, $Root)
        while ($Listener.IsListening) {
            try { $ctx = $Listener.GetContext() } catch { break }
            $path = $ctx.Request.Url.AbsolutePath.TrimStart('/')
            $rel = $path.Replace('/', [IO.Path]::DirectorySeparatorChar)
            $file = Join-Path $Root $rel
            try {
                if (Test-Path -LiteralPath $file -PathType Leaf) {
                    $bytes = [IO.File]::ReadAllBytes($file)
                    $ctx.Response.StatusCode = 200
                    $ctx.Response.ContentType = if ($file -like '*.txt') { 'text/plain; charset=utf-8' } else { 'application/octet-stream' }
                }
                else {
                    $bytes = [byte[]]@()
                    $ctx.Response.StatusCode = 404
                    $ctx.Response.ContentType = 'text/plain; charset=utf-8'
                }
                $ctx.Response.ContentLength64 = $bytes.Length
                if ($bytes.Length -gt 0) { $ctx.Response.OutputStream.Write($bytes, 0, $bytes.Length) }
            }
            catch { }
            finally { try { $ctx.Response.Close() } catch { } }
        }
    }
    $ps = [System.Management.Automation.PowerShell]::Create()
    $null = $ps.AddScript($handler.ToString())
    $null = $ps.AddParameter('Listener', $listener)
    $null = $ps.AddParameter('Root', $fixtureDir)
    $handle = $ps.BeginInvoke()

    try {
        Write-Host '== run the real installer against the loopback fixture'
        $installDir = Join-Path $work 'installed'
        $pathStore = Join-Path $work 'path.json'
        $out = & $installScript -Version $FixtureVersion -ReleaseBaseUrl "http://127.0.0.1:$port" -InstallDir $installDir -PathStore $pathStore -Architecture $Architecture 2>&1
        $exit = $LASTEXITCODE
        if ($exit -ne 0) {
            throw "installer exited ${exit}: $($out -join "`n")"
        }
        $final = Join-Path $installDir 'devlog.exe'
        if (-not (Test-Path -LiteralPath $final -PathType Leaf)) { throw 'installed devlog.exe missing' }
        $installed = & $final --version 2>&1
        if (($installed -join "`n") -notmatch [regex]::Escape($FixtureVersion)) {
            throw "installed binary does not report $FixtureVersion"
        }
        if (-not (Test-Path -LiteralPath $pathStore -PathType Leaf)) { throw 'PATH store was not written' }
        $pathJson = Get-Content -LiteralPath $pathStore -Raw | ConvertFrom-Json
        if ($pathJson.Value -notmatch [regex]::Escape($installDir)) { throw 'PATH store does not contain the install dir' }
        Write-Host 'OK: download -> checksum verify -> stage -> version verify -> install -> final verify -> PATH entry'
    }
    finally {
        try { $listener.Stop() } catch { }
        try { if (-not $handle.IsCompleted) { $ps.Stop() } } catch { }
        try { $ps.EndInvoke($handle) } catch { }
        try { $ps.Dispose() } catch { }
        try { $listener.Close() } catch { }
    }
}
finally {
    Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host 'PASS: live installer run verified'