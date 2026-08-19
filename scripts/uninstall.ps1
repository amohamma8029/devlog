<#
.SYNOPSIS
Uninstalls devlog for the current user on Windows without administrator rights.

.DESCRIPTION
Reverses scripts/install.ps1: removes devlog.exe and any devlog.exe.backup-*
files from the install directory, removes the empty bin/ and Programs/devlog/
directories, and removes the install directory's entry from the current
user's PATH. Idempotent: safe to run more than once. Unless -NoModifyPath is
set, the user PATH (HKCU\Environment\Path) is edited in place.

The parameters below marked as internal test seams must not be used by
production callers. They exist so the test harness can run isolated scenarios
against local fixtures and in-memory PATH state.
#>
[CmdletBinding()]
param(
    [switch]$NoModifyPath,

    [Parameter(DontShow)]
    [string]$InstallDir = '',
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
    [Console]::Error.WriteLine('devlog uninstall failed: PowerShell 5.1 or newer is required')
    exit 1
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

function Remove-PathEntry {
    param($Current, [string]$Entry)
    if (-not $Current.Exists) { return '' }
    $normalizedEntry = Normalize-PathEntry $Entry
    $parts = $Current.Value.Split(';')
    $kept = @()
    foreach ($part in $parts) {
        if ((Normalize-PathEntry $part) -ne $normalizedEntry) {
            $kept += $part
        }
    }
    return ($kept -join ';')
}

if (-not $InstallDir) {
    $InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\devlog\bin'
}
$finalPath = Join-Path $InstallDir 'devlog.exe'

$errors = [System.Collections.Generic.List[string]]::new()

$pathState = Read-UserPath
$pathChanged = $false
$binaryRemoved = $false
$backupsRemoved = $false
$keptBinDir = $null
$keptParentDir = $null

try {
    if (Test-Path -LiteralPath $finalPath -PathType Leaf) {
        if ($FailureHooks -contains 'remove-binary') { throw 'forced failure: removing binary' }
        Remove-Item -LiteralPath $finalPath -Force
        $binaryRemoved = $true
    }

    $backups = @(Get-ChildItem -LiteralPath $InstallDir -Filter 'devlog.exe.backup-*' -File -ErrorAction SilentlyContinue)
    foreach ($backup in $backups) {
        Remove-Item -LiteralPath $backup.FullName -Force
    }
    $backupsRemoved = $true

    if (Test-Path -LiteralPath $InstallDir -PathType Container) {
        $remaining = @(Get-ChildItem -LiteralPath $InstallDir -Force -ErrorAction SilentlyContinue)
        if ($remaining.Count -eq 0) {
            if ($FailureHooks -contains 'remove-bin-dir') { throw 'forced failure: removing bin dir' }
            Remove-Item -LiteralPath $InstallDir -Force
        }
        else {
            $keptBinDir = $remaining | ForEach-Object { $_.Name }
        }
    }

    $parentDir = Split-Path -Parent $InstallDir
    if (Test-Path -LiteralPath $parentDir -PathType Container) {
        $remainingParent = @(Get-ChildItem -LiteralPath $parentDir -Force -ErrorAction SilentlyContinue)
        if ($remainingParent.Count -eq 0) {
            if ($FailureHooks -contains 'remove-parent-dir') { throw 'forced failure: removing parent dir' }
            Remove-Item -LiteralPath $parentDir -Force
        }
        else {
            $keptParentDir = $remainingParent | ForEach-Object { $_.Name }
        }
    }

    if (-not $NoModifyPath) {
        if ($pathState.Exists) {
            $newValue = Remove-PathEntry -Current $pathState -Entry $InstallDir
            if ($newValue -ne $pathState.Value) {
                $pathChanged = $true
                if ($FailureHooks -contains 'path-write') { throw 'forced failure: PATH write after mutation' }
                Write-UserPath -Value $newValue -Kind $pathState.Kind
                $env:Path = ($env:Path -split ';' | Where-Object { (Normalize-PathEntry $_) -ne (Normalize-PathEntry $InstallDir) }) -join ';'
            }
        }
    }
}
catch {
    $errors.Add("primary: " + $_.Exception.Message)
    if ($pathChanged) {
        try {
            if ($FailureHooks -contains 'path-restore') { throw 'forced failure: restoring PATH' }
            Write-UserPath -Value $pathState.Value -Kind $pathState.Kind
        }
        catch {
            $errors.Add("rollback: restoring PATH: " + $_.Exception.Message)
        }
    }
}

if ($errors.Count -gt 0) {
    [Console]::Error.WriteLine('devlog uninstall failed: ' + ($errors -join ' | '))
    exit 1
}

[Console]::Out.WriteLine("devlog uninstalled from $InstallDir")
if ($keptBinDir) {
    [Console]::Out.WriteLine("Kept non-empty directory $InstallDir with: " + ($keptBinDir -join ', '))
}
if ($keptParentDir) {
    [Console]::Out.WriteLine("Kept non-empty directory $parentDir with: " + ($keptParentDir -join ', '))
}
if ($pathChanged) {
    [Console]::Out.WriteLine('A new shell session may be required for PATH changes to take effect.')
}