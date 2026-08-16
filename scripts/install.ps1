<#
.SYNOPSIS
Installs devlog for the current user on Windows without administrator rights.

.DESCRIPTION
Downloads the release ZIP and checksum file for the selected version, verifies
the ZIP SHA-256 against exactly one matching checksum entry, stages and runs
the candidate before touching the installed binary, backs up an existing
binary, replaces it, verifies the final path, and commits by deleting the
backup. Unless -NoModifyPath is set, the install directory is added once to
the current user's unexpanded PATH.

The parameters below marked as internal test seams must not be used by
production callers. They exist so the test harness can run isolated scenarios
against local fixtures and in-memory PATH state.
#>
[CmdletBinding()]
param(
    [string]$Version = 'latest',
    [switch]$NoModifyPath,

    [Parameter(DontShow)]
    [string]$Repository = 'amohamma8029/devlog',
    [Parameter(DontShow)]
    [string]$InstallDir = '',
    [Parameter(DontShow)]
    [string]$Architecture = '',
    [Parameter(DontShow)]
    [string]$ReleaseBaseUrl = '',
    [Parameter(DontShow)]
    [string]$LatestMetadataUrl = '',
    [Parameter(DontShow)]
    [string]$PathStore = '',
    [Parameter(DontShow)]
    [string[]]$FailureHooks = @()
)

$ErrorActionPreference = 'Stop'

if ($FailureHooks) {
    $FailureHooks = @($FailureHooks | ForEach-Object { $_.Split(',') } | Where-Object { $_.Trim() })
}

if ($PSVersionTable.PSVersion.Major -lt 5) {
    [Console]::Error.WriteLine('devlog install failed: PowerShell 5.1 or newer is required')
    exit 1
}

function Assert-ValidVersion {
    param([string]$Candidate)
    if ($Candidate -match '^v?([0-9]+\.[0-9]+\.[0-9]+)$') {
        return $Matches[1]
    }
    throw "invalid version '$Candidate': expected 'latest', '1.2.3', or 'v1.2.3'"
}

function Get-OSArch {
    try {
        $osArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
        switch ($osArch.ToString()) {
            'X64' { return 'amd64' }
            'Arm64' { return 'arm64' }
            default { throw "unsupported OS architecture: $osArch" }
        }
    }
    catch {
        $envArch = $env:PROCESSOR_ARCHITEW6432
        if (-not $envArch) { $envArch = $env:PROCESSOR_ARCHITECTURE }
        if ($envArch -eq 'AMD64') { return 'amd64' }
        if ($envArch -eq 'ARM64') { return 'arm64' }
        throw "unsupported OS architecture: $envArch"
    }
}

function Get-MatchingChecksum {
    param([string]$ChecksumFile, [string]$ZipName)
    $found = @()
    foreach ($line in Get-Content -LiteralPath $ChecksumFile) {
        if ($line -match '^([0-9a-fA-F]{64})(?:  | \*)(.+)$') {
            $fileName = $Matches[2].Trim()
            if ($fileName -eq $ZipName) {
                $found += $Matches[1].ToLowerInvariant()
            }
        }
    }
    if ($found.Count -ne 1) {
        throw "expected exactly one SHA-256 entry for $ZipName, found $($found.Count)"
    }
    return $found[0]
}

function Test-VersionOutput {
    param([string]$ExePath, [string]$Expected)
    $output = & $ExePath --version 2>&1
    if ($LASTEXITCODE -ne 0) { return $false }
    $text = [regex]::Replace(($output -join "`n"), '\x1b\[[0-9;]*m', '')
    $pattern = '(?m)^\s*version:\s*' + [regex]::Escape($Expected) + '\s*$'
    return [bool]($text -match $pattern)
}

function Get-StagedExe {
    param([string]$Staging)
    $candidates = @(Get-ChildItem -LiteralPath $Staging -Recurse -Filter 'devlog.exe' -File)
    $rootMatches = @($candidates | Where-Object {
        $relative = $_.FullName.Substring($Staging.Length).TrimStart('\', '/')
        $relative -eq 'devlog.exe'
    })
    if ($rootMatches.Count -ne 1) {
        throw "expected exactly one devlog.exe at the archive root, found $($rootMatches.Count)"
    }
    return $rootMatches[0].FullName
}

function Move-OwnedFile {
    param([string]$Source, [string]$Destination)
    try {
        Move-Item -LiteralPath $Source -Destination $Destination -Force
    }
    catch {
        Copy-Item -LiteralPath $Source -Destination $Destination -Force
        Remove-Item -LiteralPath $Source -Force
    }
}

function Read-UserPath {
    if ($PathStore) {
        if (Test-Path -LiteralPath $PathStore) {
            $data = Get-Content -LiteralPath $PathStore -Raw | ConvertFrom-Json
            return [ordered]@{ Exists = [bool]$data.Exists; Value = [string]$data.Value; Kind = [string]$data.Kind }
        }
        return [ordered]@{ Exists = $false; Value = ''; Kind = '' }
    }
    $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $false)
    try {
        if ($key.GetValueNames() -contains 'Path') {
            $value = [string]$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
            $kind = $key.GetValueKind('Path').ToString()
            return [ordered]@{ Exists = $true; Value = $value; Kind = $kind }
        }
        return [ordered]@{ Exists = $false; Value = ''; Kind = '' }
    }
    finally {
        $key.Dispose()
    }
}

function Write-UserPath {
    param([string]$Value, [string]$Kind)
    if ($PathStore) {
        [ordered]@{ Exists = $true; Value = $Value; Kind = $Kind } |
            ConvertTo-Json |
            Set-Content -LiteralPath $PathStore -Encoding UTF8
    }
    else {
        $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
        try {
            $key.SetValue('Path', $Value, ([Microsoft.Win32.RegistryValueKind]::$Kind))
        }
        finally {
            $key.Dispose()
        }
    }
}

function Remove-UserPathValue {
    if ($PathStore) {
        Remove-Item -LiteralPath $PathStore -Force
    }
    else {
        $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
        try {
            $key.DeleteValue('Path', $false)
        }
        finally {
            $key.Dispose()
        }
    }
}

function Normalize-PathEntry {
    param([string]$Part)
    $p = $Part.Trim().Trim('"')
    $p = $p.Replace('/', '\')
    $p = $p.TrimEnd('\')
    if ($p.Length -ge 2 -and $p[1] -eq ':') { return $p.ToLowerInvariant() }
    return $p.ToLowerInvariant()
}

function Add-PathEntry {
    param($Current, [string]$Entry)
    if (-not $Current.Exists) { return $Entry }
    $existing = $Current.Value
    $normalizedEntry = Normalize-PathEntry $Entry
    foreach ($part in $existing.Split(';')) {
        if ((Normalize-PathEntry $part) -eq $normalizedEntry) {
            return $existing
        }
    }
    if ($existing.EndsWith(';')) {
        return $existing + $Entry
    }
    return $existing + ';' + $Entry
}

if (-not $InstallDir) {
    $InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\devlog\bin'
}
$finalPath = Join-Path $InstallDir 'devlog.exe'

if (-not $Architecture) {
    $Architecture = Get-OSArch
}
if ($Architecture -ne 'amd64' -and $Architecture -ne 'arm64') {
    throw "unsupported architecture '$Architecture': only amd64 and arm64 are supported"
}

$errors = [System.Collections.Generic.List[string]]::new()

$pathState = Read-UserPath
$finalExisted = Test-Path -LiteralPath $finalPath -PathType Leaf

$tempRoot = New-Item -ItemType Directory -Path (Join-Path ([System.IO.Path]::GetTempPath()) ('devlog-install-' + [guid]::NewGuid().ToString('N')))
$tempRoot = $tempRoot.FullName

$replacementStarted = $false
$backupPath = $null
$pathChanged = $false

try {
    $normalized = ''
    if ($Version -eq 'latest') {
        $metaUrl = $LatestMetadataUrl
        if (-not $metaUrl) { $metaUrl = "https://api.github.com/repos/$Repository/releases/latest" }
        $meta = Invoke-WebRequest -Uri $metaUrl -UseBasicParsing -TimeoutSec 60 -ErrorAction Stop
        $tag = ($meta.Content | ConvertFrom-Json).tag_name
        if (-not $tag) { throw 'latest release metadata returned no tag_name' }
        $normalized = Assert-ValidVersion -Candidate ([string]$tag)
    }
    else {
        $normalized = Assert-ValidVersion -Candidate $Version
    }

    $zipName = "devlog_${normalized}_windows_${Architecture}.zip"
    $checksumName = "devlog_${normalized}_checksums.txt"
    $downloadBase = $ReleaseBaseUrl
    if (-not $downloadBase) { $downloadBase = "https://github.com/$Repository/releases/download" }
    $zipUrl = "$downloadBase/v$normalized/$zipName"
    $checksumUrl = "$downloadBase/v$normalized/$checksumName"

    $zipPath = Join-Path $tempRoot $zipName
    $checksumPath = Join-Path $tempRoot $checksumName

    Invoke-WebRequest -Uri $zipUrl -UseBasicParsing -TimeoutSec 600 -OutFile $zipPath -ErrorAction Stop
    Invoke-WebRequest -Uri $checksumUrl -UseBasicParsing -TimeoutSec 60 -OutFile $checksumPath -ErrorAction Stop

    $expectedHash = Get-MatchingChecksum -ChecksumFile $checksumPath -ZipName $zipName
    $actualHash = (Get-FileHash -LiteralPath $zipPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualHash -ne $expectedHash) {
        throw "checksum mismatch for ${zipName}: expected $expectedHash, got $actualHash"
    }

    $staging = Join-Path $tempRoot 'staging'
    New-Item -ItemType Directory -Path $staging | Out-Null
    Expand-Archive -LiteralPath $zipPath -DestinationPath $staging
    $stagedExe = Get-StagedExe -Staging $staging
    if (-not (Test-VersionOutput -ExePath $stagedExe -Expected $normalized)) {
        throw "staged devlog.exe did not report version $normalized"
    }

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

    if (Test-Path -LiteralPath $finalPath -PathType Leaf) {
        $backupPath = Join-Path $InstallDir ('devlog.exe.backup-' + [guid]::NewGuid().ToString('N'))
        Move-Item -LiteralPath $finalPath -Destination $backupPath -Force
    }
    Move-OwnedFile -Source $stagedExe -Destination $finalPath
    $replacementStarted = $true

    if (-not (Test-VersionOutput -ExePath $finalPath -Expected $normalized)) {
        throw "installed devlog.exe did not report version $normalized"
    }
    if ($FailureHooks -contains 'final-verify') { throw 'forced failure: final verification' }

    if (-not $NoModifyPath) {
        $newPathValue = Add-PathEntry -Current $pathState -Entry $InstallDir
        if ($pathState.Exists) {
            $pathKind = $pathState.Kind
        }
        else {
            $pathKind = 'ExpandString'
        }
        Write-UserPath -Value $newPathValue -Kind $pathKind
        $pathChanged = $true
        if ($FailureHooks -contains 'path-write') { throw 'forced failure: PATH write after mutation' }
        $env:Path = $InstallDir + ';' + $env:Path
    }

    if ($backupPath -and (Test-Path -LiteralPath $backupPath)) {
        Remove-Item -LiteralPath $backupPath -Force
    }
}
catch {
    $errors.Add("primary: " + $_.Exception.Message)
    if ($replacementStarted) {
        try {
            if ($FailureHooks -contains 'remove-final') { throw 'forced failure: removing new final executable' }
            if (Test-Path -LiteralPath $finalPath -PathType Leaf) {
                Remove-Item -LiteralPath $finalPath -Force
            }
        }
        catch {
            $errors.Add("rollback: removing new final executable: " + $_.Exception.Message)
        }
    }
    if ($backupPath -and (Test-Path -LiteralPath $backupPath)) {
        try {
            if ($FailureHooks -contains 'restore-backup') { throw 'forced failure: restoring backup' }
            Move-OwnedFile -Source $backupPath -Destination $finalPath
        }
        catch {
            $errors.Add("rollback: restoring backup: " + $_.Exception.Message)
        }
    }
    if ($pathChanged) {
        try {
            if ($FailureHooks -contains 'path-restore') { throw 'forced failure: restoring PATH' }
            if ($pathState.Exists) {
                Write-UserPath -Value $pathState.Value -Kind $pathState.Kind
            }
            else {
                Remove-UserPathValue
            }
        }
        catch {
            $errors.Add("rollback: restoring PATH: " + $_.Exception.Message)
        }
    }
}
finally {
    try {
        if ($FailureHooks -contains 'cleanup') { throw 'forced failure: temporary cleanup' }
        if (Test-Path -LiteralPath $tempRoot) {
            Remove-Item -LiteralPath $tempRoot -Recurse -Force
        }
    }
    catch {
        $errors.Add("cleanup: removing temporary directory: " + $_.Exception.Message)
    }
}

if ($errors.Count -gt 0) {
    [Console]::Error.WriteLine('devlog install failed: ' + ($errors -join ' | '))
    exit 1
}

[Console]::Out.WriteLine("devlog $normalized installed to $InstallDir")
if ($pathChanged) {
    [Console]::Out.WriteLine('A new shell session may be required for PATH changes to take effect.')
}
