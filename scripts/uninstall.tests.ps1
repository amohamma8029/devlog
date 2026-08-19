<#
.SYNOPSIS
Isolated evidence harness for scripts/uninstall.ps1.

.DESCRIPTION
Runs every uninstaller scenario in a fresh PowerShell process whose lifetime is
owned by a kill-on-close Windows Job Object. Fixtures (fake devlog.exe binaries,
devlog.exe.backup-* files, extra user files, and PATH store files) live under a
test-owned temporary root. The real registry, profile, PATH, install directory,
and GitHub are never touched. Scaffolding (NativeProc, assertion helpers, and
Invoke-Uninstaller) mirrors scripts/install.tests.ps1 so the two harnesses stay
structurally aligned.
#>
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

$script:RepoRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$script:UninstallScript = Join-Path $script:RepoRoot 'scripts\uninstall.ps1'
$script:TestRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('devlog-uninstaller-tests-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $script:TestRoot | Out-Null
$script:PwshPath = (Get-Process -Id $PID).Path
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

function Read-Store {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) { return $null }
    $data = Get-Content -Raw -LiteralPath $Path | ConvertFrom-Json
    return [pscustomobject]@{ Exists = [bool]$data.Exists; Value = [string]$data.Value; Kind = [string]$data.Kind }
}

function Normalize-PathEntry {
    param([string]$Part)
    $p = $Part.Trim().Trim('"')
    $p = $p.Replace('/', '\')
    $p = $p.TrimEnd('\')
    if ($p.Length -ge 2 -and $p[1] -eq ':') { return $p.ToLowerInvariant() }
    return $p.ToLowerInvariant()
}

function Path-ContainsEntry {
    param([string]$Value, [string]$Entry)
    $normalizedEntry = Normalize-PathEntry $Entry
    foreach ($part in $Value.Split(';')) {
        if ((Normalize-PathEntry $part) -eq $normalizedEntry) { return $true }
    }
    return $false
}

function New-UninstallFixtures {
    param(
        [switch]$WithBinary,
        [int]$BackupCount = 0,
        [switch]$ExtraFiles,
        [switch]$ExtraParentFiles,
        [string]$StoreValue = '',
        [string]$StoreKind = 'ExpandString',
        [switch]$StoreReadOnly
    )
    $root = Join-Path $script:TestRoot ("fx-" + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Force -Path $root | Out-Null
    # Mirror the real install layout: %LOCALAPPDATA%\Programs\devlog\bin\devlog.exe.
    # The parent (Programs\devlog) can become empty after removing bin/, which lets
    # the parent-removal logic be exercised. The store and logs live at <root> so
    # they never interfere with parent-removal assertions.
    $installDir = Join-Path $root 'Programs\devlog\bin'

    if ($WithBinary) {
        New-Item -ItemType Directory -Force -Path $installDir | Out-Null
        $exe = Join-Path $installDir 'devlog.exe'
        New-FixtureExe -Version '1.2.3' -OutPath $exe
    }
    elseif ($BackupCount -gt 0 -or $ExtraFiles) {
        New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    }

    if ($BackupCount -gt 0) {
        for ($i = 0; $i -lt $BackupCount; $i++) {
            $backupName = 'devlog.exe.backup-' + [guid]::NewGuid().ToString('N')
            Set-Content -LiteralPath (Join-Path $installDir $backupName) -Value 'fake backup' -Encoding UTF8
        }
    }

    if ($ExtraFiles) {
        Set-Content -LiteralPath (Join-Path $installDir 'user-notes.txt') -Value 'user content' -Encoding UTF8
    }

    $storePath = Join-Path $root 'store.json'
    if ($StoreValue -ne '' -or $StoreReadOnly) {
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
        FinalPath = $finalPath
    }
}

function Invoke-Uninstaller {
    param(
        [psobject]$F,
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
        '-InstallDir', $F.InstallDir,
        '-PathStore', $F.StorePath
    )
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
        ' -NoProfile -ExecutionPolicy Bypass -File ' + (Quote-CL $script:UninstallScript) + ' ' +
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
                throw 'uninstaller process did not exit after job termination'
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
            throw "uninstaller left $descendants descendant processes running"
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

function Assert-StoreUnchanged {
    param($F)
    $after = if (Test-Path -LiteralPath $F.StorePath) { Get-Content -Raw -LiteralPath $F.StorePath } else { $null }
    Assert-Equal $after $F.StoreBefore 'PATH store must be unchanged'
}

function Assert-NoBackups {
    param($F)
    $backups = @(Get-ChildItem -LiteralPath $F.InstallDir -Filter 'devlog.exe.backup-*' -ErrorAction SilentlyContinue)
    Assert-True ($backups.Count -eq 0) "unexpected backups: $($backups.Name -join ', ')"
}

# --- scenarios ---

function Test-FreshUninstall {
    $f = New-UninstallFixtures -WithBinary -StoreValue '<installDir>'
    # fix the store value to the actual install dir (the placeholder is replaced below)
    Set-Content -LiteralPath $f.StorePath -Value ([ordered]@{ Exists = $true; Value = $f.InstallDir; Kind = 'ExpandString' } | ConvertTo-Json) -Encoding UTF8
    $f.StoreBefore = Get-Content -Raw -LiteralPath $f.StorePath
    try {
        $r = Invoke-Uninstaller -F $f
        Assert-True ($r.ExitCode -eq 0) "uninstaller must succeed, stderr: $($r.Stderr)"
        Assert-Contains $r.Stdout 'uninstalled from' 'success text must appear'
        Assert-True (-not (Test-Path -LiteralPath $f.FinalPath)) 'devlog.exe must be gone'
        Assert-True (-not (Test-Path -LiteralPath $f.InstallDir)) 'bin dir must be gone'
        $parentDir = Split-Path -Parent $f.InstallDir
        Assert-True (-not (Test-Path -LiteralPath $parentDir)) 'parent dir must be gone'
        Assert-NoBackups $f
        $store = Read-Store $f.StorePath
        Assert-True ($null -ne $store) 'PATH store must exist'
        Assert-Equal $store.Value '' 'PATH store value must be empty after removing the only entry'
        Assert-Equal $store.Kind 'ExpandString' 'store kind must be preserved'
    }
    finally { }
}

function Test-UpgradeThenUninstall {
    $f = New-UninstallFixtures -WithBinary
    $seedValue = "C:\old;%USERPROFILE%\bin;$($f.InstallDir)"
    Set-Content -LiteralPath $f.StorePath -Value ([ordered]@{ Exists = $true; Value = $seedValue; Kind = 'ExpandString' } | ConvertTo-Json) -Encoding UTF8
    $f.StoreBefore = Get-Content -Raw -LiteralPath $f.StorePath
    try {
        $r = Invoke-Uninstaller -F $f
        Assert-True ($r.ExitCode -eq 0) "uninstaller must succeed, stderr: $($r.Stderr)"
        Assert-True (-not (Test-Path -LiteralPath $f.FinalPath)) 'devlog.exe must be gone'
        Assert-True (-not (Test-Path -LiteralPath $f.InstallDir)) 'bin dir must be gone'
        $store = Read-Store $f.StorePath
        Assert-Equal $store.Value 'C:\old;%USERPROFILE%\bin' 'PATH must have only the install dir entry removed'
        Assert-Equal $store.Kind 'ExpandString' 'PATH kind must be preserved'
    }
    finally { }
}

function Test-IdempotentNoBinary {
    $f = New-UninstallFixtures -StoreValue '<placeholder>'
    Set-Content -LiteralPath $f.StorePath -Value ([ordered]@{ Exists = $true; Value = $f.InstallDir; Kind = 'ExpandString' } | ConvertTo-Json) -Encoding UTF8
    $f.StoreBefore = Get-Content -Raw -LiteralPath $f.StorePath
    try {
        $r = Invoke-Uninstaller -F $f
        Assert-True ($r.ExitCode -eq 0) "uninstaller must succeed with no binary, stderr: $($r.Stderr)"
        Assert-True (-not (Test-Path -LiteralPath $f.FinalPath)) 'devlog.exe must be absent'
        $store = Read-Store $f.StorePath
        Assert-Equal $store.Value '' 'PATH entry must be removed'
    }
    finally { }
}

function Test-IdempotentNoPathEntry {
    $f = New-UninstallFixtures -WithBinary -StoreValue 'C:\other' -StoreKind 'ExpandString'
    try {
        $r = Invoke-Uninstaller -F $f
        Assert-True ($r.ExitCode -eq 0) "uninstaller must succeed with no PATH entry, stderr: $($r.Stderr)"
        Assert-True (-not (Test-Path -LiteralPath $f.FinalPath)) 'devlog.exe must be gone'
        Assert-True (-not (Test-Path -LiteralPath $f.InstallDir)) 'bin dir must be gone'
        Assert-StoreUnchanged $f
    }
    finally { }
}

function Test-IdempotentRerun {
    $f = New-UninstallFixtures -WithBinary
    Set-Content -LiteralPath $f.StorePath -Value ([ordered]@{ Exists = $true; Value = $f.InstallDir; Kind = 'ExpandString' } | ConvertTo-Json) -Encoding UTF8
    try {
        $r1 = Invoke-Uninstaller -F $f
        Assert-True ($r1.ExitCode -eq 0) "first uninstall must succeed, stderr: $($r1.Stderr)"
        $storeAfter1 = Read-Store $f.StorePath
        $r2 = Invoke-Uninstaller -F $f
        Assert-True ($r2.ExitCode -eq 0) "second uninstall must succeed, stderr: $($r2.Stderr)"
        Assert-Contains $r2.Stdout 'uninstalled from' 'success text must appear on idempotent rerun'
        $storeAfter2 = Read-Store $f.StorePath
        Assert-Equal $storeAfter2.Value $storeAfter1.Value 'PATH must not change on second run'
    }
    finally { }
}

function Test-PathPreservesOtherEntries {
    $f = New-UninstallFixtures -WithBinary
    $seedValue = "$($f.InstallDir);C:\other;D:\tools"
    Set-Content -LiteralPath $f.StorePath -Value ([ordered]@{ Exists = $true; Value = $seedValue; Kind = 'ExpandString' } | ConvertTo-Json) -Encoding UTF8
    try {
        $r = Invoke-Uninstaller -F $f
        Assert-True ($r.ExitCode -eq 0) "uninstaller must succeed, stderr: $($r.Stderr)"
        $store = Read-Store $f.StorePath
        Assert-Equal $store.Value 'C:\other;D:\tools' 'other entries must be preserved exactly'
    }
    finally { }
}

function Test-PathPreservesEmptyEntries {
    $f = New-UninstallFixtures -WithBinary
    $seedValue = ";;$($f.InstallDir);;"
    Set-Content -LiteralPath $f.StorePath -Value ([ordered]@{ Exists = $true; Value = $seedValue; Kind = 'ExpandString' } | ConvertTo-Json) -Encoding UTF8
    try {
        $r = Invoke-Uninstaller -F $f
        Assert-True ($r.ExitCode -eq 0) "uninstaller must succeed, stderr: $($r.Stderr)"
        $store = Read-Store $f.StorePath
        $inputParts = $seedValue.Split(';')
        $outputParts = $store.Value.Split(';')
        $inputEmptyCount = ($inputParts | Where-Object { $_ -eq '' }).Count
        $outputEmptyCount = ($outputParts | Where-Object { $_ -eq '' }).Count
        Assert-Equal $outputEmptyCount $inputEmptyCount 'empty entry count must be preserved (4 before, 4 after)'
        Assert-True (-not (Path-ContainsEntry $store.Value $f.InstallDir)) 'install dir entry must be gone'
    }
    finally { }
}

function Test-PathDedupQuotedEquivalent {
    $f = New-UninstallFixtures -WithBinary
    $quoted = '"' + $f.InstallDir + '\"'
    $seedValue = "$quoted;C:\other"
    Set-Content -LiteralPath $f.StorePath -Value ([ordered]@{ Exists = $true; Value = $seedValue; Kind = 'ExpandString' } | ConvertTo-Json) -Encoding UTF8
    try {
        $r = Invoke-Uninstaller -F $f
        Assert-True ($r.ExitCode -eq 0) "uninstaller must succeed, stderr: $($r.Stderr)"
        $store = Read-Store $f.StorePath
        Assert-Equal $store.Value 'C:\other' 'quoted install dir entry must be removed, leaving C:\other'
    }
    finally { }
}

function Test-PathTrailingSeparatorEquivalent {
    $f = New-UninstallFixtures -WithBinary
    $withTrailing = $f.InstallDir + '\'
    Set-Content -LiteralPath $f.StorePath -Value ([ordered]@{ Exists = $true; Value = $withTrailing; Kind = 'ExpandString' } | ConvertTo-Json) -Encoding UTF8
    try {
        $r = Invoke-Uninstaller -F $f
        Assert-True ($r.ExitCode -eq 0) "uninstaller must succeed, stderr: $($r.Stderr)"
        $store = Read-Store $f.StorePath
        Assert-Equal $store.Value '' 'trailing-separator entry must be removed'
    }
    finally { }
}

function Test-PathUppercaseEquivalent {
    $f = New-UninstallFixtures -WithBinary
    $upper = $f.InstallDir.ToUpperInvariant()
    $seedValue = "$upper;C:\other"
    Set-Content -LiteralPath $f.StorePath -Value ([ordered]@{ Exists = $true; Value = $seedValue; Kind = 'ExpandString' } | ConvertTo-Json) -Encoding UTF8
    try {
        $r = Invoke-Uninstaller -F $f
        Assert-True ($r.ExitCode -eq 0) "uninstaller must succeed, stderr: $($r.Stderr)"
        $store = Read-Store $f.StorePath
        Assert-Equal $store.Value 'C:\other' 'uppercase install dir entry must be removed'
    }
    finally { }
}

function Test-PathRemovesDuplicates {
    $f = New-UninstallFixtures -WithBinary
    $seedValue = "$($f.InstallDir);C:\other;$($f.InstallDir)"
    Set-Content -LiteralPath $f.StorePath -Value ([ordered]@{ Exists = $true; Value = $seedValue; Kind = 'ExpandString' } | ConvertTo-Json) -Encoding UTF8
    try {
        $r = Invoke-Uninstaller -F $f
        Assert-True ($r.ExitCode -eq 0) "uninstaller must succeed, stderr: $($r.Stderr)"
        $store = Read-Store $f.StorePath
        Assert-Equal $store.Value 'C:\other' 'both duplicate install dir entries must be removed'
    }
    finally { }
}

function Test-NoModifyPathSkipsPath {
    $f = New-UninstallFixtures -WithBinary
    Set-Content -LiteralPath $f.StorePath -Value ([ordered]@{ Exists = $true; Value = $f.InstallDir; Kind = 'ExpandString' } | ConvertTo-Json) -Encoding UTF8
    $f.StoreBefore = Get-Content -Raw -LiteralPath $f.StorePath
    try {
        $r = Invoke-Uninstaller -F $f -ExtraArgs @('-NoModifyPath')
        Assert-True ($r.ExitCode -eq 0) "uninstaller must succeed, stderr: $($r.Stderr)"
        Assert-True (-not (Test-Path -LiteralPath $f.FinalPath)) 'devlog.exe must be gone'
        Assert-True (-not (Test-Path -LiteralPath $f.InstallDir)) 'bin dir must be gone'
        Assert-StoreUnchanged $f
    }
    finally { }
}

function Test-RemovesBackupFiles {
    $f = New-UninstallFixtures -WithBinary -BackupCount 2
    try {
        $r = Invoke-Uninstaller -F $f
        Assert-True ($r.ExitCode -eq 0) "uninstaller must succeed, stderr: $($r.Stderr)"
        Assert-True (-not (Test-Path -LiteralPath $f.FinalPath)) 'devlog.exe must be gone'
        Assert-NoBackups $f
        Assert-True (-not (Test-Path -LiteralPath $f.InstallDir)) 'bin dir must be gone after removing all files'
    }
    finally { }
}

function Test-KeepsNonEmptyDir {
    $f = New-UninstallFixtures -WithBinary -ExtraFiles
    try {
        $r = Invoke-Uninstaller -F $f
        Assert-True ($r.ExitCode -eq 0) "uninstaller must succeed, stderr: $($r.Stderr)"
        Assert-True (-not (Test-Path -LiteralPath $f.FinalPath)) 'devlog.exe must be gone'
        Assert-True (Test-Path -LiteralPath $f.InstallDir) 'bin dir must be kept (non-empty)'
        Assert-True (Test-Path -LiteralPath (Join-Path $f.InstallDir 'user-notes.txt')) 'user file must remain'
        Assert-Contains $r.Stdout $f.InstallDir 'stdout must mention the kept directory'
        Assert-Contains $r.Stdout 'user-notes.txt' 'stdout must mention the remaining file'
    }
    finally { }
}

function Test-KeepsNonEmptyParent {
    $f = New-UninstallFixtures -WithBinary
    $parentDir = Split-Path -Parent $f.InstallDir
    Set-Content -LiteralPath (Join-Path $parentDir 'README-user.txt') -Value 'user parent content' -Encoding UTF8
    try {
        $r = Invoke-Uninstaller -F $f
        Assert-True ($r.ExitCode -eq 0) "uninstaller must succeed, stderr: $($r.Stderr)"
        Assert-True (-not (Test-Path -LiteralPath $f.InstallDir)) 'bin dir must be gone'
        Assert-True (Test-Path -LiteralPath $parentDir) 'parent dir must be kept (non-empty)'
        Assert-True (Test-Path -LiteralPath (Join-Path $parentDir 'README-user.txt')) 'parent user file must remain'
        Assert-Contains $r.Stdout $parentDir 'stdout must mention the kept parent directory'
    }
    finally { }
}

function Test-PathWriteFailureAfterBinaryRemoved {
    $f = New-UninstallFixtures -WithBinary
    Set-Content -LiteralPath $f.StorePath -Value ([ordered]@{ Exists = $true; Value = $f.InstallDir; Kind = 'ExpandString' } | ConvertTo-Json) -Encoding UTF8
    $f.StoreBefore = Get-Content -Raw -LiteralPath $f.StorePath
    Set-ItemProperty -LiteralPath $f.StorePath -Name IsReadOnly -Value $true
    try {
        $r = Invoke-Uninstaller -F $f
        Assert-True ($r.ExitCode -ne 0) 'uninstaller must exit non-zero on PATH write failure'
        Assert-Contains $r.Stderr 'primary:' 'stderr must mention primary error'
        Assert-True (-not (Test-Path -LiteralPath $f.FinalPath)) 'devlog.exe must be gone (filesystem not rolled back)'
        Assert-StoreUnchanged $f
    }
    finally {
        Set-ItemProperty -LiteralPath $f.StorePath -Name IsReadOnly -Value $false -ErrorAction SilentlyContinue
    }
}

function Test-PathRestoreFailure {
    $f = New-UninstallFixtures -WithBinary
    Set-Content -LiteralPath $f.StorePath -Value ([ordered]@{ Exists = $true; Value = $f.InstallDir; Kind = 'ExpandString' } | ConvertTo-Json) -Encoding UTF8
    $f.StoreBefore = Get-Content -Raw -LiteralPath $f.StorePath
    Set-ItemProperty -LiteralPath $f.StorePath -Name IsReadOnly -Value $true
    try {
        $r = Invoke-Uninstaller -F $f -ExtraArgs @('-FailureHooks', 'path-write', 'path-restore')
        Assert-True ($r.ExitCode -ne 0) 'uninstaller must exit non-zero'
        Assert-Contains $r.Stderr 'primary:' 'stderr must mention primary error'
        Assert-Contains $r.Stderr 'rollback: restoring PATH:' 'stderr must mention PATH restore failure'
        Assert-ErrorOrder $r.Stderr @('primary:', 'rollback: restoring PATH:')
        Assert-True (-not (Test-Path -LiteralPath $f.FinalPath)) 'devlog.exe must be gone'
    }
    finally {
        Set-ItemProperty -LiteralPath $f.StorePath -Name IsReadOnly -Value $false -ErrorAction SilentlyContinue
    }
}

function Test-CompetingErrorsBinaryAndPath {
    $f = New-UninstallFixtures -WithBinary
    Set-Content -LiteralPath $f.StorePath -Value ([ordered]@{ Exists = $true; Value = $f.InstallDir; Kind = 'ExpandString' } | ConvertTo-Json) -Encoding UTF8
    $f.StoreBefore = Get-Content -Raw -LiteralPath $f.StorePath
    Set-ItemProperty -LiteralPath $f.StorePath -Name IsReadOnly -Value $true
    try {
        $r = Invoke-Uninstaller -F $f -ExtraArgs @('-FailureHooks', 'remove-binary', 'path-write', 'path-restore')
        Assert-True ($r.ExitCode -ne 0) 'uninstaller must exit non-zero'
        Assert-Contains $r.Stderr 'primary:' 'stderr must mention primary error'
        Assert-Contains $r.Stderr 'forced failure: removing binary' 'primary must be the remove-binary hook'
        Assert-True (Test-Path -LiteralPath $f.FinalPath) 'binary must remain (remove-binary threw before removal)'
        Assert-StoreUnchanged $f
    }
    finally {
        Set-ItemProperty -LiteralPath $f.StorePath -Name IsReadOnly -Value $false -ErrorAction SilentlyContinue
    }
}

function Test-FailureHookRemoveBinDir {
    $f = New-UninstallFixtures
    New-Item -ItemType Directory -Force -Path $f.InstallDir | Out-Null
    try {
        $r = Invoke-Uninstaller -F $f -ExtraArgs @('-FailureHooks', 'remove-bin-dir')
        Assert-True ($r.ExitCode -ne 0) 'uninstaller must exit non-zero'
        Assert-Contains $r.Stderr 'primary:' 'stderr must mention primary error'
        Assert-Contains $r.Stderr 'forced failure: removing bin dir' 'must mention the bin-dir hook'
        Assert-True (Test-Path -LiteralPath $f.InstallDir) 'bin dir must still exist (throw was before Remove-Item)'
    }
    finally { }
}

function Test-FailureHookRemoveParentDir {
    $f = New-UninstallFixtures
    New-Item -ItemType Directory -Force -Path $f.InstallDir | Out-Null
    $parentDir = Split-Path -Parent $f.InstallDir
    try {
        $r = Invoke-Uninstaller -F $f -ExtraArgs @('-FailureHooks', 'remove-parent-dir')
        Assert-True ($r.ExitCode -ne 0) 'uninstaller must exit non-zero'
        Assert-Contains $r.Stderr 'primary:' 'stderr must mention primary error'
        Assert-Contains $r.Stderr 'forced failure: removing parent dir' 'must mention the parent-dir hook'
        Assert-True (Test-Path -LiteralPath $parentDir) 'parent dir must still exist (throw was before Remove-Item)'
    }
    finally { }
}

# --- dispatcher ---

$scenarioNames = @(
    'Test-FreshUninstall',
    'Test-UpgradeThenUninstall',
    'Test-IdempotentNoBinary',
    'Test-IdempotentNoPathEntry',
    'Test-IdempotentRerun',
    'Test-PathPreservesOtherEntries',
    'Test-PathPreservesEmptyEntries',
    'Test-PathDedupQuotedEquivalent',
    'Test-PathTrailingSeparatorEquivalent',
    'Test-PathUppercaseEquivalent',
    'Test-PathRemovesDuplicates',
    'Test-NoModifyPathSkipsPath',
    'Test-RemovesBackupFiles',
    'Test-KeepsNonEmptyDir',
    'Test-KeepsNonEmptyParent',
    'Test-PathWriteFailureAfterBinaryRemoved',
    'Test-PathRestoreFailure',
    'Test-CompetingErrorsBinaryAndPath',
    'Test-FailureHookRemoveBinDir',
    'Test-FailureHookRemoveParentDir'
)

Write-Host "Running $($scenarioNames.Count) uninstaller scenarios..."

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
    Remove-Item -LiteralPath $script:TestRoot -Recurse -Force -ErrorAction SilentlyContinue
}

if ($script:failures.Count -gt 0) {
    Write-Host "uninstaller tests: $($script:failures.Count) failed: $($script:failures -join ', ')"
    exit 1
}

Write-Host "uninstaller tests: all $($scenarioNames.Count) scenarios passed"