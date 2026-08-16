<#
.SYNOPSIS
Isolated evidence harness for scripts/install.ps1.

.DESCRIPTION
Runs every installer scenario in a fresh PowerShell process whose lifetime is
owned by a kill-on-close Windows Job Object. Fixtures (ZIPs, checksums,
version-reporting devlog.exe clones, PATH store files, and an in-process HTTP
listener) live under a test-owned temporary root. The real registry, profile,
PATH, install directory, and GitHub are never touched.
#>
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

$script:RepoRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$script:InstallScript = Join-Path $script:RepoRoot 'scripts\install.ps1'
$script:TestRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('devlog-installer-tests-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $script:TestRoot | Out-Null
$script:PwshPath = (Get-Process -Id $PID).Path
$script:tempBefore = @(Get-ChildItem -LiteralPath $env:TEMP -Directory -Filter 'devlog-install-*' -ErrorAction SilentlyContinue | ForEach-Object FullName)
$script:servers = [System.Collections.Generic.List[object]]::new()
$script:failures = [System.Collections.Generic.List[string]]::new()

Add-Type -AssemblyName System.IO.Compression.FileSystem

$script:nativeCs = @'
using System;
using System.Runtime.InteropServices;

public static class NativeProc
{
    const uint CREATE_SUSPENDED = 0x4;
    const int STARTF_USESTDHANDLES = 0x100;
    const int JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE = 0x2000;
    const int JobObjectExtendedLimitInformation = 9;
    const int JobObjectBasicAccountingInformation = 1;
    const uint GENERIC_READ = 0x80000000;
    const uint GENERIC_WRITE = 0x40000000;
    const uint FILE_SHARE_READ = 1;
    const uint FILE_SHARE_WRITE = 2;
    const uint OPEN_EXISTING = 3;
    const uint OPEN_ALWAYS = 4;
    const uint FILE_ATTRIBUTE_NORMAL = 0x80;
    const uint HANDLE_FLAG_INHERIT = 0x1;
    const uint STILL_ACTIVE = 259;
    const uint WAIT_TIMEOUT = 0x102;
    const uint WAIT_FAILED = 0xFFFFFFFF;

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    public struct STARTUPINFO
    {
        public int cb;
        public string lpReserved;
        public string lpDesktop;
        public string lpTitle;
        public int dwX, dwY, dwXSize, dwYSize;
        public int dwXCountChars, dwYCountChars, dwFillAttribute, dwFlags;
        public short wShowWindow, cbReserved2;
        public IntPtr lpReserved2;
        public IntPtr hStdInput, hStdOutput, hStdError;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct PROCESS_INFORMATION
    {
        public IntPtr hProcess;
        public IntPtr hThread;
        public int dwProcessId;
        public int dwThreadId;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct JOBOBJECT_BASIC_LIMIT_INFORMATION
    {
        public long PerProcessUserTimeLimit, PerJobUserTimeLimit;
        public int LimitFlags;
        public UIntPtr MinimumWorkingSetSize, MaximumWorkingSetSize;
        public int ActiveProcessLimit;
        public UIntPtr Affinity;
        public int PriorityClass, SchedulingClass;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct IO_COUNTERS
    {
        public ulong ReadOperationCount, WriteOperationCount, OtherOperationCount;
        public ulong ReadTransferCount, WriteTransferCount, OtherTransferCount;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct JOBOBJECT_EXTENDED_LIMIT_INFORMATION
    {
        public JOBOBJECT_BASIC_LIMIT_INFORMATION Basic;
        public IO_COUNTERS Io;
        public UIntPtr ProcessMemoryLimit, JobMemoryLimit;
        public UIntPtr PeakProcessMemoryUsed, PeakJobMemoryUsed;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct JOBOBJECT_BASIC_ACCOUNTING_INFORMATION
    {
        public long TotalUserTime, TotalKernelTime;
        public long ThisPeriodTotalUserTime, ThisPeriodTotalKernelTime;
        public int TotalPageFaultCount, TotalProcesses, ActiveProcesses, TotalTerminatedProcesses;
    }

    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern bool CreateProcess(string lpApplicationName, string lpCommandLine, IntPtr lpProcessAttributes, IntPtr lpThreadAttributes, bool bInheritHandles, uint dwCreationFlags, IntPtr lpEnvironment, string lpCurrentDirectory, ref STARTUPINFO lpStartupInfo, out PROCESS_INFORMATION lpProcessInformation);

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern IntPtr CreateJobObject(IntPtr lpJobAttributes, string lpName);

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool SetInformationJobObject(IntPtr hJob, int JobObjectInfoClass, IntPtr lpJobObjectInfo, uint cbJobObjectInfoLength);

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool AssignProcessToJobObject(IntPtr hJob, IntPtr hProcess);

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern uint ResumeThread(IntPtr hThread);

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool QueryInformationJobObject(IntPtr hJob, int JobObjectInfoClass, IntPtr lpJobObjectInfo, uint cbJobObjectInfoLength, out uint lpReturnLength);

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool TerminateJobObject(IntPtr hJob, uint uExitCode);

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool CloseHandle(IntPtr hObject);

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern uint WaitForSingleObject(IntPtr hHandle, uint dwMilliseconds);

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool GetExitCodeProcess(IntPtr hProcess, out uint lpExitCode);

    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern IntPtr CreateFile(string lpFileName, uint dwDesiredAccess, uint dwShareMode, IntPtr lpSecurityAttributes, uint dwCreationDisposition, uint dwFlagsAndAttributes, IntPtr hTemplateFile);

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool SetHandleInformation(IntPtr hObject, uint dwMask, uint dwFlags);

    public static IntPtr OpenLogFile(string path)
    {
        IntPtr h = CreateFile(path, GENERIC_WRITE, FILE_SHARE_READ | FILE_SHARE_WRITE, IntPtr.Zero, OPEN_ALWAYS, FILE_ATTRIBUTE_NORMAL, IntPtr.Zero);
        if (h == IntPtr.Zero || h == (IntPtr)(-1))
            throw new Exception("CreateFile failed: " + Marshal.GetLastWin32Error());
        SetHandleInformation(h, HANDLE_FLAG_INHERIT, HANDLE_FLAG_INHERIT);
        return h;
    }

    public sealed class JobProcess
    {
        public IntPtr Job;
        public IntPtr Process;
        public IntPtr StdOut;
        public IntPtr StdErr;
        public uint Pid;
    }

    public static JobProcess StartSuspended(string exePath, string commandLine, string workingDirectory, IntPtr stdoutHandle, IntPtr stderrHandle)
    {
        IntPtr job = CreateJobObject(IntPtr.Zero, null);
        if (job == IntPtr.Zero)
            throw new Exception("CreateJobObject failed: " + Marshal.GetLastWin32Error());

        var ext = new JOBOBJECT_EXTENDED_LIMIT_INFORMATION();
        ext.Basic.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
        IntPtr mem = Marshal.AllocHGlobal(Marshal.SizeOf(ext));
        try
        {
            Marshal.StructureToPtr(ext, mem, false);
            if (!SetInformationJobObject(job, JobObjectExtendedLimitInformation, mem, (uint)Marshal.SizeOf(ext)))
            {
                int err = Marshal.GetLastWin32Error();
                CloseHandle(job);
                throw new Exception("SetInformationJobObject failed: " + err);
            }
        }
        finally { Marshal.FreeHGlobal(mem); }

        IntPtr stdinHandle = CreateFile("NUL", GENERIC_READ, FILE_SHARE_READ | FILE_SHARE_WRITE, IntPtr.Zero, OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, IntPtr.Zero);
        if (stdinHandle == IntPtr.Zero || stdinHandle == (IntPtr)(-1))
        {
            CloseHandle(job);
            throw new Exception("CreateFile NUL failed: " + Marshal.GetLastWin32Error());
        }
        SetHandleInformation(stdinHandle, HANDLE_FLAG_INHERIT, HANDLE_FLAG_INHERIT);

        var si = new STARTUPINFO();
        si.cb = Marshal.SizeOf(si);
        si.dwFlags = STARTF_USESTDHANDLES;
        si.hStdInput = stdinHandle;
        si.hStdOutput = stdoutHandle;
        si.hStdError = stderrHandle;

        PROCESS_INFORMATION pi;
        bool created = CreateProcess(exePath, commandLine, IntPtr.Zero, IntPtr.Zero, true, CREATE_SUSPENDED, IntPtr.Zero, workingDirectory, ref si, out pi);
        CloseHandle(stdinHandle);
        if (!created)
        {
            int err = Marshal.GetLastWin32Error();
            TerminateJobObject(job, 1);
            CloseHandle(job);
            throw new Exception("CreateProcess failed: " + err);
        }

        if (!AssignProcessToJobObject(job, pi.hProcess))
        {
            int err = Marshal.GetLastWin32Error();
            TerminateJobObject(job, 1);
            WaitForSingleObject(pi.hProcess, 10000);
            CloseHandle(pi.hThread);
            CloseHandle(pi.hProcess);
            CloseHandle(job);
            throw new Exception("AssignProcessToJobObject failed: " + err);
        }

        if (ResumeThread(pi.hThread) == 0xFFFFFFFF)
        {
            int err = Marshal.GetLastWin32Error();
            TerminateJobObject(job, 1);
            WaitForSingleObject(pi.hProcess, 10000);
            CloseHandle(pi.hThread);
            CloseHandle(pi.hProcess);
            CloseHandle(job);
            throw new Exception("ResumeThread failed: " + err);
        }
        CloseHandle(pi.hThread);

        var jp = new JobProcess();
        jp.Job = job;
        jp.Process = pi.hProcess;
        jp.Pid = (uint)pi.dwProcessId;
        jp.StdOut = stdoutHandle;
        jp.StdErr = stderrHandle;
        return jp;
    }

    public static uint WaitWithTimeout(IntPtr hProcess, uint timeoutMs)
    {
        uint r = WaitForSingleObject(hProcess, timeoutMs);
        if (r == WAIT_FAILED)
            throw new Exception("WaitForSingleObject failed: " + Marshal.GetLastWin32Error());
        return r;
    }

    public static uint ExitCode(IntPtr hProcess)
    {
        uint code;
        if (!GetExitCodeProcess(hProcess, out code))
            return STILL_ACTIVE;
        return code;
    }

    public static int ActiveProcesses(IntPtr job)
    {
        var acc = new JOBOBJECT_BASIC_ACCOUNTING_INFORMATION();
        IntPtr mem = Marshal.AllocHGlobal(Marshal.SizeOf(acc));
        try
        {
            uint ret;
            if (!QueryInformationJobObject(job, JobObjectBasicAccountingInformation, mem, (uint)Marshal.SizeOf(acc), out ret))
                throw new Exception("QueryInformationJobObject failed: " + Marshal.GetLastWin32Error());
            acc = (JOBOBJECT_BASIC_ACCOUNTING_INFORMATION)Marshal.PtrToStructure(mem, typeof(JOBOBJECT_BASIC_ACCOUNTING_INFORMATION));
            return acc.ActiveProcesses;
        }
        finally { Marshal.FreeHGlobal(mem); }
    }

    public static void KillJob(IntPtr job) { TerminateJobObject(job, 1); }
    public static void CloseJob(IntPtr job) { CloseHandle(job); }
    public static void CloseHandleSafe(IntPtr h) { if (h != IntPtr.Zero) CloseHandle(h); }
}
'@
Add-Type -TypeDefinition $script:nativeCs

function Assert-True {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw "assertion failed: $Message" }
}

function Assert-Equal {
    param($Actual, $Expected, [string]$Message)
    if ($Actual -ne $Expected) { throw "assertion failed: $Message (expected '$Expected', got '$Actual')" }
}

function Assert-Contains {
    param([string]$Text, [string]$Needle, [string]$Message)
    if (-not $Text.Contains($Needle)) { throw "assertion failed: $Message (missing '$Needle')" }
}

function Assert-NotContains {
    param([string]$Text, [string]$Needle, [string]$Message)
    if ($Text.Contains($Needle)) { throw "assertion failed: $Message (unexpected '$Needle')" }
}

function Assert-ErrorOrder {
    param([string]$Stderr, [string[]]$Categories)
    $last = -1
    foreach ($c in $Categories) {
        $idx = $Stderr.IndexOf($c)
        Assert-True ($idx -gt $last) "error categories out of order: '$($Categories -join ' < ')' in: $Stderr"
        $last = $idx
    }
}

function Test-BytesEqual {
    param([byte[]]$A, [byte[]]$B)
    if ($A.Length -ne $B.Length) { return $false }
    for ($i = 0; $i -lt $A.Length; $i++) {
        if ($A[$i] -ne $B[$i]) { return $false }
    }
    return $true
}

function Get-FreePort {
    $tcp = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
    $tcp.Start()
    try { return ([System.Net.IPEndPoint]$tcp.LocalEndpoint).Port }
    finally { $tcp.Stop() }
}

function Quote-CL {
    param([string]$Value)
    return '"' + ($Value -replace '"', '""') + '"'
}

function New-FixtureExe {
    param([string]$Version, [string]$OutPath)
    $dir = Split-Path -Parent $OutPath
    $src = @"
package main

import "fmt"

func main() {
    fmt.Println("devlog")
    fmt.Println("  version: $Version")
}
"@
    $goFile = Join-Path $dir 'fixture_main.go'
    Set-Content -LiteralPath $goFile -Value $src -Encoding UTF8
    go build -o $OutPath $goFile
    if ($LASTEXITCODE -ne 0) { throw "go build of fixture $Version failed" }
    Remove-Item -LiteralPath $goFile -Force
}

function New-ReleaseZip {
    param([string]$ZipPath, [string]$ExeSource, [switch]$MissingExe)
    $zip = [System.IO.Compression.ZipFile]::Open($ZipPath, [System.IO.Compression.ZipArchiveMode]::Create)
    try {
        if (-not $MissingExe) {
            [System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, $ExeSource, 'devlog.exe') | Out-Null
        }
    }
    finally {
        $zip.Dispose()
    }
}

function Start-FixtureServer {
    param([string]$Root, [string]$MetaMode)
    $port = Get-FreePort
    $listener = [System.Net.HttpListener]::new()
    $listener.Prefixes.Add("http://127.0.0.1:$port/")
    $listener.Start()
    $log = [System.Collections.Concurrent.ConcurrentQueue[string]]::new()
    $handler = {
        param($Listener, $Log, $Root, $MetaMode)
        while ($Listener.IsListening) {
            try { $ctx = $Listener.GetContext() } catch { break }
            $path = $ctx.Request.Url.AbsolutePath
            $Log.Enqueue($path)
            try {
                if ($path -eq '/releases/latest') {
                    switch ($MetaMode) {
                        'bad-tag' { $body = '{"tag_name":"not-a-version"}' }
                        'no-tag' { $body = '{}' }
                        'prerelease' { $body = '{"tag_name":"v1.2.3-beta.1"}' }
                        'notfound' { $body = '' }
                        default { $body = '{"tag_name":"v1.2.3"}' }
                    }
                    $status = if ($MetaMode -eq 'notfound') { 404 } else { 200 }
                    $bytes = [Text.Encoding]::UTF8.GetBytes($body)
                    $contentType = 'application/json; charset=utf-8'
                }
                else {
                    $rel = $path.TrimStart('/').Replace('/', [IO.Path]::DirectorySeparatorChar)
                    $file = Join-Path $Root $rel
                    if (Test-Path -LiteralPath $file -PathType Leaf) {
                        $bytes = [IO.File]::ReadAllBytes($file)
                        $status = 200
                        $contentType = if ($file -like '*.txt') { 'text/plain; charset=utf-8' } else { 'application/octet-stream' }
                    }
                    else {
                        $bytes = [byte[]]@()
                        $status = 404
                        $contentType = 'text/plain; charset=utf-8'
                    }
                }
                $ctx.Response.StatusCode = $status
                $ctx.Response.ContentType = $contentType
                $ctx.Response.ContentLength64 = $bytes.Length
                if ($bytes.Length -gt 0) {
                    $ctx.Response.OutputStream.Write($bytes, 0, $bytes.Length)
                }
            }
            catch { }
            finally { try { $ctx.Response.Close() } catch { } }
        }
    }
    $ps = [System.Management.Automation.PowerShell]::Create()
    $null = $ps.AddScript($handler.ToString())
    $null = $ps.AddParameter('Listener', $listener)
    $null = $ps.AddParameter('Log', $log)
    $null = $ps.AddParameter('Root', $Root)
    $null = $ps.AddParameter('MetaMode', $MetaMode)
    $handle = $ps.BeginInvoke()
    $server = [pscustomobject]@{
        BaseUrl = "http://127.0.0.1:$port"
        Listener = $listener
        Log = $log
        MetaMode = $MetaMode
        Ps = $ps
        Handle = $handle
        Stopped = $false
    }
    $script:servers.Add($server)
    return $server
}

function Stop-Server {
    param($Server)
    if ($Server.Stopped) { return }
    try { $Server.Listener.Stop() } catch { }
    try {
        if (-not $Server.Handle.IsCompleted) { $Server.Ps.Stop() }
    }
    catch { }
    try { $Server.Ps.EndInvoke($Server.Handle) } catch { }
    try { $Server.Ps.Dispose() } catch { }
    try { $Server.Listener.Close() } catch { }
    $Server.Stopped = $true
}

function New-Fixtures {
    param(
        [string]$RequestedVersion = '1.2.3',
        [string]$NewExeVersion = '1.2.3',
        [switch]$WithOld,
        [string]$OldExeVersion = '1.0.0',
        [string]$ChecksumMode = 'ok',
        [switch]$ZipMissingExe,
        [string]$MetaMode = '',
        [string]$StoreValue = '',
        [string]$RootOverride = $null,
        [string]$StoreKind = 'ExpandString',
        [switch]$StoreReadOnly
    )
    $root = if ($RootOverride) { $RootOverride } else { Join-Path $script:TestRoot ("fx-" + [guid]::NewGuid().ToString('N')) }
    New-Item -ItemType Directory -Force -Path $root | Out-Null
    $installDir = Join-Path $root 'install'

    if ($WithOld) {
        $oldExe = Join-Path $root 'old.exe'
        New-FixtureExe -Version $OldExeVersion -OutPath $oldExe
        New-Item -ItemType Directory -Force -Path $installDir | Out-Null
        Copy-Item -LiteralPath $oldExe -Destination (Join-Path $installDir 'devlog.exe')
    }

    $newExe = Join-Path $root 'new.exe'
    New-FixtureExe -Version $NewExeVersion -OutPath $newExe

    $www = Join-Path $root 'www'
    New-Item -ItemType Directory -Force -Path $www | Out-Null
    $normalizedVersion = ($RequestedVersion -replace '^v', '')
    $versionDir = Join-Path $www ('v' + $normalizedVersion)
    New-Item -ItemType Directory -Force -Path $versionDir | Out-Null
    $zipName = "devlog_${normalizedVersion}_windows_amd64.zip"
    $zipPath = Join-Path $versionDir $zipName
    New-ReleaseZip -ZipPath $zipPath -ExeSource $newExe -MissingExe:$ZipMissingExe
    $realHash = (Get-FileHash -LiteralPath $zipPath -Algorithm SHA256).Hash.ToLowerInvariant()

    $checksumName = "devlog_${normalizedVersion}_checksums.txt"
    $checksumPath = Join-Path $versionDir $checksumName
    switch ($ChecksumMode) {
        'ok'        { $line = "$realHash  $zipName" }
        'star'      { $line = "$realHash *$zipName" }
        'wrong'     { $line = ('0' * 64) + "  $zipName" }
        'missing'   { $line = "$realHash  devlog_${normalizedVersion}_windows_arm64.zip" }
        'duplicate' { $line = "$realHash  $zipName`n$realHash  $zipName" }
        default     { throw "unknown checksum mode $ChecksumMode" }
    }
    Set-Content -LiteralPath $checksumPath -Value $line -Encoding ASCII

    $storePath = Join-Path $root 'store.json'
    if ($StoreValue) {
        [ordered]@{ Exists = $true; Value = $StoreValue; Kind = $StoreKind } |
            ConvertTo-Json |
            Set-Content -LiteralPath $storePath -Encoding UTF8
        if ($StoreReadOnly) {
            Set-ItemProperty -LiteralPath $storePath -Name IsReadOnly -Value $true
        }
    }

    $finalPath = Join-Path $installDir 'devlog.exe'
    return [pscustomobject]@{
        Root = $root
        InstallDir = $installDir
        StorePath = $storePath
        StoreBefore = if (Test-Path -LiteralPath $storePath) { Get-Content -Raw -LiteralPath $storePath } else { $null }
        Www = $www
        ZipName = $zipName
        ChecksumName = $checksumName
        NormalizedVersion = $normalizedVersion
        FinalPath = $finalPath
        OldBytes = if ($WithOld) { [IO.File]::ReadAllBytes($finalPath) } else { $null }
        NewBytes = [IO.File]::ReadAllBytes($newExe)
    }
}

function Read-Store {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) { return $null }
    $data = Get-Content -Raw -LiteralPath $Path | ConvertFrom-Json
    return [pscustomobject]@{ Exists = [bool]$data.Exists; Value = [string]$data.Value; Kind = [string]$data.Kind }
}

function Assert-NoTempLeftovers {
    $left = @(Get-ChildItem -LiteralPath $env:TEMP -Directory -Filter 'devlog-install-*' -ErrorAction SilentlyContinue | Where-Object { $_.FullName -notin $script:tempBefore })
    Assert-True ($left.Count -eq 0) "leftover installer temp dirs: $($left.FullName -join ', ')"
}

function Remove-NewTempLeftovers {
    $left = @(Get-ChildItem -LiteralPath $env:TEMP -Directory -Filter 'devlog-install-*' -ErrorAction SilentlyContinue | Where-Object { $_.FullName -notin $script:tempBefore })
    Assert-True ($left.Count -ge 1) 'expected a leftover installer temp dir'
    foreach ($d in $left) {
        Remove-Item -LiteralPath $d.FullName -Recurse -Force
    }
}

function Assert-NoBackups {
    param($F)
    $backups = @(Get-ChildItem -LiteralPath $F.InstallDir -Filter 'devlog.exe.backup-*' -ErrorAction SilentlyContinue)
    Assert-True ($backups.Count -eq 0) "unexpected backups: $($backups.Name -join ', ')"
}

function Assert-RequestLog {
    param($Server, [string[]]$Expected)
    $actual = @($Server.Log.ToArray())
    $ok = $actual.Count -eq $Expected.Count
    if ($ok) {
        for ($i = 0; $i -lt $Expected.Count; $i++) {
            if ($actual[$i] -ne $Expected[$i]) { $ok = $false; break }
        }
    }
    Assert-True $ok "unexpected request log: got [$($actual -join ', ')], expected [$($Expected -join ', ')]"
}

function Invoke-Installer {
    param(
        [string]$Version = '1.2.3',
        [psobject]$F,
        [psobject]$Server,
        [string[]]$ExtraArgs = @(),
        [int]$TimeoutSeconds = 120
    )
    $logDir = Join-Path $F.Root 'logs'
    New-Item -ItemType Directory -Force -Path $logDir | Out-Null
    $stdoutPath = Join-Path $logDir 'stdout.txt'
    $stderrPath = Join-Path $logDir 'stderr.txt'
    $hOut = [NativeProc]::OpenLogFile($stdoutPath)
    $hErr = [NativeProc]::OpenLogFile($stderrPath)

    $argList = @(
        '-Version', $Version,
        '-InstallDir', $F.InstallDir,
        '-ReleaseBaseUrl', $Server.BaseUrl,
        '-PathStore', $F.StorePath
    )
    if (($ExtraArgs | Where-Object { $_ -eq '-Architecture' }).Count -eq 0) {
        $argList += @('-Architecture', 'amd64')
    }
    if ($Server.MetaMode) {
        $argList += @('-LatestMetadataUrl', "$($Server.BaseUrl)/releases/latest")
    }
    $i = 0
    while ($i -lt $ExtraArgs.Count) {
        $arg = $ExtraArgs[$i]
        if ($arg -eq '-FailureHooks') {
            $values = [System.Collections.Generic.List[string]]::new()
            $i++
            while ($i -lt $ExtraArgs.Count -and -not $ExtraArgs[$i].StartsWith('-')) {
                $values.Add($ExtraArgs[$i])
                $i++
            }
            if ($values.Count -gt 0) {
                $argList += '-FailureHooks'
                $argList += ($values -join ',')
            }
        }
        else {
            $argList += $arg
            $i++
        }
    }
    $cmdLine = (Quote-CL $script:PwshPath) +
        ' -NoProfile -ExecutionPolicy Bypass -File ' + (Quote-CL $script:InstallScript) + ' ' +
        (($argList | ForEach-Object { Quote-CL $_ }) -join ' ')

    $jp = $null
    try {
        $jp = [NativeProc]::StartSuspended($script:PwshPath, $cmdLine, $script:RepoRoot, $hOut, $hErr)
    }
    catch {
        [NativeProc]::CloseHandleSafe($hOut)
        [NativeProc]::CloseHandleSafe($hErr)
        throw
    }

    $exitCode = -1
    $descendants = -1
    $timedOut = $false
    try {
        $wait = [NativeProc]::WaitWithTimeout($jp.Process, $TimeoutSeconds * 1000)
        if ($wait -eq 0x102) {
            $timedOut = $true
            [NativeProc]::KillJob($jp.Job)
            $wait2 = [NativeProc]::WaitWithTimeout($jp.Process, 15000)
            if ($wait2 -eq 0x102) {
                throw 'installer process did not exit after job termination'
            }
        }
        $exitCode = [int][NativeProc]::ExitCode($jp.Process)
        $descendants = [NativeProc]::ActiveProcesses($jp.Job)
        if ($descendants -lt 0) {
            throw 'could not query job object accounting for descendant processes'
        }
        if ($descendants -gt 0) {
            [NativeProc]::KillJob($jp.Job)
            [NativeProc]::WaitWithTimeout($jp.Process, 15000) | Out-Null
            throw "installer left $descendants descendant processes running"
        }
    }
    finally {
        [NativeProc]::CloseJob($jp.Job)
        [NativeProc]::CloseHandleSafe($jp.Process)
        [NativeProc]::CloseHandleSafe($jp.StdOut)
        [NativeProc]::CloseHandleSafe($jp.StdErr)
    }

    $stdout = if (Test-Path -LiteralPath $stdoutPath) { Get-Content -Raw -LiteralPath $stdoutPath } else { '' }
    $stderr = if (Test-Path -LiteralPath $stderrPath) { Get-Content -Raw -LiteralPath $stderrPath } else { '' }
    return [pscustomobject]@{
        ExitCode = $exitCode
        Stdout = [string]$stdout
        Stderr = [string]$stderr
        TimedOut = $timedOut
        Descendants = $descendants
    }
}

function Assert-FailedInstall {
    param($R, $F, [string[]]$ExpectedStderr, [switch]$AllowTempLeftover)
    Assert-True ($R.ExitCode -ne 0) 'installer must exit non-zero'
    Assert-NotContains $R.Stdout 'installed to' 'success text must not appear on failure'
    foreach ($needle in $ExpectedStderr) {
        Assert-Contains $R.Stderr $needle "stderr must mention '$needle'"
    }
    if (-not $AllowTempLeftover) { Assert-NoTempLeftovers }
}

function Assert-StoreUnchanged {
    param($F)
    $after = if (Test-Path -LiteralPath $F.StorePath) { Get-Content -Raw -LiteralPath $F.StorePath } else { $null }
    Assert-Equal $after $F.StoreBefore 'PATH store must be unchanged'
}

function Assert-FinalReportsVersion {
    param($F)
    $out = & $F.FinalPath --version 2>&1
    Assert-True ($LASTEXITCODE -eq 0) 'installed devlog.exe must exit 0'
    Assert-Contains (($out -join "`n")) "version: $($F.NormalizedVersion)" 'installed devlog.exe must report the requested version'
}

# --- scenarios ---

function Test-InvalidVersion {
    $f = New-Fixtures
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server -Version '1.2'
        Assert-FailedInstall $r $f @('invalid version')
        Assert-RequestLog $server @()
        Assert-True (-not (Test-Path $f.InstallDir)) 'install dir must not be created'
        Assert-StoreUnchanged $f
    }
    finally { Stop-Server $server }
}

function Test-PrereleaseVersion {
    $f = New-Fixtures
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server -Version 'v1.2.3-beta.1'
        Assert-FailedInstall $r $f @('invalid version')
        Assert-RequestLog $server @()
        Assert-StoreUnchanged $f
    }
    finally { Stop-Server $server }
}

function Test-UnsupportedArchitecture {
    $f = New-Fixtures
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server -ExtraArgs @('-Architecture', 'x86')
        Assert-FailedInstall $r $f @('unsupported architecture')
        Assert-RequestLog $server @()
        Assert-StoreUnchanged $f
    }
    finally { Stop-Server $server }
}

function Test-LatestMalformedTag {
    $f = New-Fixtures -MetaMode 'bad-tag'
    $server = Start-FixtureServer -Root $f.Www -MetaMode 'bad-tag'
    try {
        $r = Invoke-Installer -F $f -Server $server -Version latest
        Assert-FailedInstall $r $f @('invalid version')
        Assert-RequestLog $server @('/releases/latest')
        Assert-StoreUnchanged $f
    }
    finally { Stop-Server $server }
}

function Test-LatestMissingTag {
    $f = New-Fixtures -MetaMode 'no-tag'
    $server = Start-FixtureServer -Root $f.Www -MetaMode 'no-tag'
    try {
        $r = Invoke-Installer -F $f -Server $server -Version latest
        Assert-FailedInstall $r $f @('no tag_name')
        Assert-RequestLog $server @('/releases/latest')
        Assert-StoreUnchanged $f
    }
    finally { Stop-Server $server }
}

function Test-LatestPrereleaseTag {
    $f = New-Fixtures -MetaMode 'prerelease'
    $server = Start-FixtureServer -Root $f.Www -MetaMode 'prerelease'
    try {
        $r = Invoke-Installer -F $f -Server $server -Version latest
        Assert-FailedInstall $r $f @('invalid version')
        Assert-RequestLog $server @('/releases/latest')
        Assert-StoreUnchanged $f
    }
    finally { Stop-Server $server }
}

function Test-LatestHttpFailure {
    $f = New-Fixtures -MetaMode 'notfound'
    $server = Start-FixtureServer -Root $f.Www -MetaMode 'notfound'
    try {
        $r = Invoke-Installer -F $f -Server $server -Version latest
        Assert-FailedInstall $r $f @('404')
        Assert-RequestLog $server @('/releases/latest')
        Assert-StoreUnchanged $f
    }
    finally { Stop-Server $server }
}

function Test-ChecksumMismatch {
    $f = New-Fixtures -WithOld -ChecksumMode wrong
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server
        Assert-FailedInstall $r $f @('checksum mismatch')
        Assert-True ((Test-Path $f.FinalPath) -and (Test-BytesEqual ([IO.File]::ReadAllBytes($f.FinalPath)) $f.OldBytes)) 'old binary must be unchanged'
        Assert-NoBackups $f
        Assert-StoreUnchanged $f
        Assert-RequestLog $server @("/v1.2.3/$($f.ZipName)", "/v1.2.3/$($f.ChecksumName)")
    }
    finally { Stop-Server $server }
}

function Test-ChecksumMissingEntry {
    $f = New-Fixtures -WithOld -ChecksumMode missing
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server
        Assert-FailedInstall $r $f @('exactly one SHA-256 entry')
        Assert-True (Test-BytesEqual ([IO.File]::ReadAllBytes($f.FinalPath)) $f.OldBytes) 'old binary must be unchanged'
        Assert-NoBackups $f
        Assert-StoreUnchanged $f
    }
    finally { Stop-Server $server }
}

function Test-ChecksumDuplicateEntry {
    $f = New-Fixtures -WithOld -ChecksumMode duplicate
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server
        Assert-FailedInstall $r $f @('exactly one SHA-256 entry')
        Assert-True (Test-BytesEqual ([IO.File]::ReadAllBytes($f.FinalPath)) $f.OldBytes) 'old binary must be unchanged'
        Assert-NoBackups $f
        Assert-StoreUnchanged $f
    }
    finally { Stop-Server $server }
}

function Test-StarChecksumLine {
    $f = New-Fixtures -ChecksumMode star
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server
        Assert-True ($r.ExitCode -eq 0) "installer must succeed, stderr: $($r.Stderr)"
        Assert-Contains $r.Stdout 'installed to' 'success text must appear'
        Assert-FinalReportsVersion $f
        Assert-NoBackups $f
        Assert-NoTempLeftovers
    }
    finally { Stop-Server $server }
}

function Test-StagedVersionMismatch {
    $f = New-Fixtures -WithOld -NewExeVersion '9.9.9'
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server
        Assert-FailedInstall $r $f @('staged devlog.exe did not report version 1.2.3')
        Assert-True (Test-BytesEqual ([IO.File]::ReadAllBytes($f.FinalPath)) $f.OldBytes) 'old binary must be unchanged'
        Assert-NoBackups $f
        Assert-StoreUnchanged $f
    }
    finally { Stop-Server $server }
}

function Test-ZipMissingExe {
    $f = New-Fixtures -WithOld -ZipMissingExe
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server
        Assert-FailedInstall $r $f @('exactly one devlog.exe')
        Assert-True (Test-BytesEqual ([IO.File]::ReadAllBytes($f.FinalPath)) $f.OldBytes) 'old binary must be unchanged'
        Assert-NoBackups $f
        Assert-StoreUnchanged $f
    }
    finally { Stop-Server $server }
}

function Test-FreshInstall {
    $f = New-Fixtures
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server
        Assert-True ($r.ExitCode -eq 0) "installer must succeed, stderr: $($r.Stderr)"
        Assert-Contains $r.Stdout 'installed to' 'success text must appear'
        Assert-Contains $r.Stdout 'new shell' 'new-shell guidance must appear'
        Assert-FinalReportsVersion $f
        Assert-NoBackups $f
        Assert-NoTempLeftovers
        $store = Read-Store $f.StorePath
        Assert-True ($null -ne $store) 'PATH store must be created'
        Assert-Equal $store.Value $f.InstallDir 'store must contain only the install dir'
        Assert-Equal $store.Kind 'ExpandString' 'store must use ExpandString'
        Assert-RequestLog $server @("/v1.2.3/$($f.ZipName)", "/v1.2.3/$($f.ChecksumName)")
    }
    finally { Stop-Server $server }
}

function Test-FreshInstallNoModifyPath {
    $f = New-Fixtures
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server -ExtraArgs @('-NoModifyPath')
        Assert-True ($r.ExitCode -eq 0) "installer must succeed, stderr: $($r.Stderr)"
        Assert-Contains $r.Stdout 'installed to' 'success text must appear'
        Assert-NotContains $r.Stdout 'new shell' 'no new-shell guidance with -NoModifyPath'
        Assert-FinalReportsVersion $f
        Assert-NoBackups $f
        Assert-True (-not (Test-Path -LiteralPath $f.StorePath)) 'PATH store must not be written'
        Assert-NoTempLeftovers
    }
    finally { Stop-Server $server }
}

function Test-Upgrade {
    $oldValue = 'C:\old;%USERPROFILE%\bin'
    $f = New-Fixtures -WithOld -StoreValue $oldValue -StoreKind 'ExpandString'
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server
        Assert-True ($r.ExitCode -eq 0) "installer must succeed, stderr: $($r.Stderr)"
        Assert-Contains $r.Stdout 'new shell' 'new-shell guidance must appear'
        Assert-True (Test-BytesEqual ([IO.File]::ReadAllBytes($f.FinalPath)) $f.NewBytes) 'final binary must be the new one'
        Assert-NoBackups $f
        $store = Read-Store $f.StorePath
        Assert-Equal $store.Value ($oldValue + ';' + $f.InstallDir) 'install dir must be appended once, preserving existing entries'
        Assert-Equal $store.Kind 'ExpandString' 'store kind must be preserved'
        Assert-NoTempLeftovers
    }
    finally { Stop-Server $server }
}

function Test-LatestInstall {
    $f = New-Fixtures -MetaMode valid
    $server = Start-FixtureServer -Root $f.Www -MetaMode valid
    try {
        $r = Invoke-Installer -F $f -Server $server -Version latest
        Assert-True ($r.ExitCode -eq 0) "installer must succeed, stderr: $($r.Stderr)"
        Assert-FinalReportsVersion $f
        Assert-RequestLog $server @('/releases/latest', "/v1.2.3/$($f.ZipName)", "/v1.2.3/$($f.ChecksumName)")
        Assert-NoTempLeftovers
    }
    finally { Stop-Server $server }
}

function Test-PathDedupQuotedEquivalent {
    $root = Join-Path $script:TestRoot ("fx-" + [guid]::NewGuid().ToString('N'))
    $dir = Join-Path $root 'install'
    $value = '"' + $dir + '\";C:\other'
    $f = New-Fixtures -RootOverride $root -StoreValue $value
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server
        Assert-True ($r.ExitCode -eq 0) "installer must succeed, stderr: $($r.Stderr)"
        Assert-True (Test-BytesEqual ([IO.File]::ReadAllBytes($f.FinalPath)) $f.NewBytes) 'final binary must be installed'
        Assert-Equal (Read-Store $f.StorePath).Value $value 'equivalent quoted entry must not be duplicated'
        Assert-NoTempLeftovers
    }
    finally { Stop-Server $server }
}

function Test-PathTrailingSeparatorEquivalent {
    $root = Join-Path $script:TestRoot ("fx-" + [guid]::NewGuid().ToString('N'))
    $dir = Join-Path $root 'install'
    $value = $dir.ToUpperInvariant() + '\'
    $f = New-Fixtures -RootOverride $root -StoreValue $value
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server
        Assert-True ($r.ExitCode -eq 0) "installer must succeed, stderr: $($r.Stderr)"
        Assert-Equal (Read-Store $f.StorePath).Value $value 'trailing-separator equivalent must not be duplicated'
        Assert-NoTempLeftovers
    }
    finally { Stop-Server $server }
}

function Test-PathPreservesEmptyEntries {
    $value = ';;C:\foo;;'
    $f = New-Fixtures -StoreValue $value
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server
        Assert-True ($r.ExitCode -eq 0) "installer must succeed, stderr: $($r.Stderr)"
        Assert-Equal (Read-Store $f.StorePath).Value ($value + $f.InstallDir) 'empty entries and separators must be preserved byte-for-byte'
        Assert-NoTempLeftovers
    }
    finally { Stop-Server $server }
}

function Test-PathWriteFailureBeforeMutation {
    $f = New-Fixtures -WithOld -StoreValue 'C:\old' -StoreReadOnly
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server
        Assert-FailedInstall $r $f @('primary:')
        Assert-NotContains $r.Stderr 'rollback:' 'no rollback category expected'
        Assert-NotContains $r.Stderr 'cleanup:' 'no cleanup category expected'
        Assert-True (Test-BytesEqual ([IO.File]::ReadAllBytes($f.FinalPath)) $f.OldBytes) 'old binary must be restored'
        Assert-NoBackups $f
        Assert-StoreUnchanged $f
    }
    finally { Stop-Server $server }
}

function Test-PathPartialMutation {
    $f = New-Fixtures -WithOld -StoreValue 'C:\old'
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server -ExtraArgs @('-FailureHooks', 'path-write')
        Assert-FailedInstall $r $f @('primary:', 'PATH write after mutation')
        Assert-True (Test-BytesEqual ([IO.File]::ReadAllBytes($f.FinalPath)) $f.OldBytes) 'old binary must be restored'
        Assert-NoBackups $f
        Assert-Equal (Read-Store $f.StorePath).Value 'C:\old' 'PATH must be restored exactly'
        Assert-Equal (Read-Store $f.StorePath).Kind 'ExpandString' 'PATH kind must be restored exactly'
    }
    finally { Stop-Server $server }
}

function Test-PathRestoreFailure {
    $f = New-Fixtures -WithOld -StoreValue 'C:\old'
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server -ExtraArgs @('-FailureHooks', 'path-write', 'path-restore')
        Assert-FailedInstall $r $f @('primary:', 'rollback: restoring PATH')
        Assert-ErrorOrder $r.Stderr @('primary:', 'rollback: restoring PATH')
        Assert-True (Test-BytesEqual ([IO.File]::ReadAllBytes($f.FinalPath)) $f.OldBytes) 'old binary must be restored'
        Assert-Contains (Read-Store $f.StorePath).Value $f.InstallDir 'mutated PATH must remain because restore failed'
    }
    finally { Stop-Server $server }
}

function Test-RollbackUpgradeFinalVerify {
    $f = New-Fixtures -WithOld -StoreValue 'C:\old'
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server -ExtraArgs @('-FailureHooks', 'final-verify')
        Assert-FailedInstall $r $f @('primary:', 'final verification')
        Assert-True (Test-BytesEqual ([IO.File]::ReadAllBytes($f.FinalPath)) $f.OldBytes) 'old binary must be restored'
        Assert-NoBackups $f
        Assert-StoreUnchanged $f
    }
    finally { Stop-Server $server }
}

function Test-RollbackFreshFinalVerify {
    $f = New-Fixtures
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server -ExtraArgs @('-FailureHooks', 'final-verify')
        Assert-FailedInstall $r $f @('primary:', 'final verification')
        Assert-True (-not (Test-Path -LiteralPath $f.FinalPath)) 'fresh final executable must be removed'
        Assert-NoBackups $f
        Assert-StoreUnchanged $f
    }
    finally { Stop-Server $server }
}

function Test-RemoveFinalFailure {
    $f = New-Fixtures
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server -ExtraArgs @('-FailureHooks', 'final-verify', 'remove-final')
        Assert-FailedInstall $r $f @('primary:', 'rollback: removing new final executable')
        Assert-True ((Test-Path -LiteralPath $f.FinalPath) -and (Test-BytesEqual ([IO.File]::ReadAllBytes($f.FinalPath)) $f.NewBytes)) 'failed removal must leave the new final in place'
        Assert-NoBackups $f
    }
    finally { Stop-Server $server }
}

function Test-RestoreBackupFailure {
    $f = New-Fixtures -WithOld
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server -ExtraArgs @('-FailureHooks', 'final-verify', 'restore-backup')
        Assert-FailedInstall $r $f @('primary:', 'rollback: restoring backup')
        Assert-True (-not (Test-Path -LiteralPath $f.FinalPath)) 'final must be absent after removal'
        $backups = @(Get-ChildItem -LiteralPath $f.InstallDir -Filter 'devlog.exe.backup-*' -ErrorAction SilentlyContinue)
        Assert-True ($backups.Count -eq 1) 'backup must remain when restore failed'
    }
    finally { Stop-Server $server }
}

function Test-CleanupFailure {
    $f = New-Fixtures
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server -ExtraArgs @('-FailureHooks', 'cleanup')
        Assert-FailedInstall $r $f @('cleanup:') -AllowTempLeftover
        Assert-NotContains $r.Stderr 'primary:' 'no primary error expected when only cleanup fails'
        Assert-True ((Test-Path -LiteralPath $f.FinalPath) -and (Test-BytesEqual ([IO.File]::ReadAllBytes($f.FinalPath)) $f.NewBytes)) 'successful install state must remain'
        Remove-NewTempLeftovers
    }
    finally { Stop-Server $server }
}

function Test-CompetingErrors {
    $f = New-Fixtures -WithOld
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server -ExtraArgs @('-FailureHooks', 'final-verify', 'remove-final', 'restore-backup', 'cleanup')
        Assert-FailedInstall $r $f @('primary:', 'rollback: removing new final executable', 'rollback: restoring backup', 'cleanup:') -AllowTempLeftover
        Assert-ErrorOrder $r.Stderr @('primary:', 'rollback: removing new final executable', 'rollback: restoring backup', 'cleanup:')
        Assert-True (Test-BytesEqual ([IO.File]::ReadAllBytes($f.FinalPath)) $f.NewBytes) 'failed removal must leave the new final in place'
        $backups = @(Get-ChildItem -LiteralPath $f.InstallDir -Filter 'devlog.exe.backup-*' -ErrorAction SilentlyContinue)
        Assert-True ($backups.Count -eq 1) 'backup must remain when restore failed'
        Remove-NewTempLeftovers
    }
    finally { Stop-Server $server }
}

function Test-CompetingErrorsWithPath {
    $f = New-Fixtures -WithOld -StoreValue 'C:\old'
    $server = Start-FixtureServer -Root $f.Www -MetaMode ''
    try {
        $r = Invoke-Installer -F $f -Server $server -ExtraArgs @('-FailureHooks', 'path-write', 'path-restore', 'cleanup')
        Assert-FailedInstall $r $f @('primary:', 'rollback: restoring PATH', 'cleanup:') -AllowTempLeftover
        Assert-ErrorOrder $r.Stderr @('primary:', 'rollback: restoring PATH', 'cleanup:')
        Assert-True (Test-BytesEqual ([IO.File]::ReadAllBytes($f.FinalPath)) $f.OldBytes) 'old binary must be restored'
        Assert-Contains (Read-Store $f.StorePath).Value $f.InstallDir 'mutated PATH must remain because restore failed'
        Remove-NewTempLeftovers
    }
    finally { Stop-Server $server }
}

# --- dispatcher ---

$scenarioNames = @(
    'Test-InvalidVersion',
    'Test-PrereleaseVersion',
    'Test-UnsupportedArchitecture',
    'Test-LatestMalformedTag',
    'Test-LatestMissingTag',
    'Test-LatestPrereleaseTag',
    'Test-LatestHttpFailure',
    'Test-ChecksumMismatch',
    'Test-ChecksumMissingEntry',
    'Test-ChecksumDuplicateEntry',
    'Test-StarChecksumLine',
    'Test-StagedVersionMismatch',
    'Test-ZipMissingExe',
    'Test-FreshInstall',
    'Test-FreshInstallNoModifyPath',
    'Test-Upgrade',
    'Test-LatestInstall',
    'Test-PathDedupQuotedEquivalent',
    'Test-PathTrailingSeparatorEquivalent',
    'Test-PathPreservesEmptyEntries',
    'Test-PathWriteFailureBeforeMutation',
    'Test-PathPartialMutation',
    'Test-PathRestoreFailure',
    'Test-RollbackUpgradeFinalVerify',
    'Test-RollbackFreshFinalVerify',
    'Test-RemoveFinalFailure',
    'Test-RestoreBackupFailure',
    'Test-CleanupFailure',
    'Test-CompetingErrors',
    'Test-CompetingErrorsWithPath'
)

Write-Host "Running $($scenarioNames.Count) installer scenarios..."

try {
    foreach ($name in $scenarioNames) {
        try {
            & $name
            Write-Host "PASS $name"
        }
        catch {
            $script:failures.Add($name)
            Write-Host "FAIL $name :: $($_.Exception.Message)"
        }
    }
}
finally {
    foreach ($server in $script:servers) {
        try { Stop-Server $server } catch { }
    }
    Remove-Item -LiteralPath $script:TestRoot -Recurse -Force -ErrorAction SilentlyContinue
}

if ($script:failures.Count -gt 0) {
    Write-Host "installer tests: $($script:failures.Count) failed: $($script:failures -join ', ')"
    exit 1
}

Write-Host "installer tests: all $($scenarioNames.Count) scenarios passed"
exit 0
