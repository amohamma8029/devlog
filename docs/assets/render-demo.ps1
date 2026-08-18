param(
    [switch]$PreflightOnly,
    [switch]$ProbeVhsTimeout,
    [switch]$ProbeFailurePaths,
    [switch]$ProbeDrainQuarantineChild
)

$ErrorActionPreference = "Stop"
$PSDefaultParameterValues['*:Encoding'] = 'utf8'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$AssetsDir = $ScriptDir
$RepoRoot = Split-Path -Parent (Split-Path -Parent $ScriptDir)

$script:CleanupErrors = [System.Collections.Generic.List[string]]::new()
$script:Created = [System.Collections.Generic.List[string]]::new()

function Write-Step([string]$message) {
    Write-Output ("[devlog-demo] " + $message)
}

function Resolve-ToolPath {
    param([string]$Name, [string]$WingetPattern, [string]$WingetFile)
    $cmd = Get-Command $Name -ErrorAction SilentlyContinue
    if ($cmd) {
        return $cmd.Source
    }
    $candidates = Get-ChildItem (Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Packages") -Recurse -Filter $WingetFile -ErrorAction SilentlyContinue |
        Where-Object { $_.FullName -match $WingetPattern } | Sort-Object FullName -Descending
    if ($candidates) {
        return $candidates[0].FullName
    }
    return $null
}

$pwshPath = (Get-Command pwsh -ErrorAction Stop).Source
$gitPath = (Get-Command git -ErrorAction Stop).Source
$goPath = (Get-Command go -ErrorAction Stop).Source
$vhsPath = Resolve-ToolPath "vhs" "charmbracelet" "vhs.exe"
$ffmpegPath = Resolve-ToolPath "ffmpeg" "Gyan" "ffmpeg.exe"
$ffprobePath = Resolve-ToolPath "ffprobe" "Gyan" "ffprobe.exe"
if (-not $vhsPath) { throw "vhs not found" }
if (-not $ffmpegPath) { throw "ffmpeg not found" }
if (-not $ffprobePath) { throw "ffprobe not found" }

$osBuild = [System.Environment]::OSVersion.Version.Build
$ttydDir = Join-Path $env:TEMP "devlog-ttyd-msvc"
if ($osBuild -ge 26200) {
    if (-not (Test-Path -LiteralPath (Join-Path $ttydDir "ttyd.exe"))) {
        throw "MSVC ttyd required on build 26200+: $ttydDir\ttyd.exe"
    }
}

$ReferenceNames = @(
    ".demo-shell.png",
    ".demo-first-tui.png",
    ".demo-palette-before.png",
    ".demo-palette-after.png",
    ".demo-history-before.png",
    ".demo-history-after.png",
    ".demo-blocker-before.png",
    ".demo-blocker-after.png",
    ".demo-handoff-after.png"
)
$FlatRel = "demo-flat.mp4"
$OwnedPaths = ($ReferenceNames | ForEach-Object { Join-Path $AssetsDir $_ }) + (Join-Path $AssetsDir $FlatRel)

function Assert-NoOwnedArtifacts {
    foreach ($p in $OwnedPaths) {
        if (Test-Path -LiteralPath $p) {
            throw "owned path already exists: $p"
        }
    }
}

function Remove-Recorded {
    param([switch]$KeepFlat)
    $deliverables = @((Join-Path $AssetsDir "demo.gif"), (Join-Path $AssetsDir "tui-screenshot.png"))
    foreach ($p in $script:Created) {
        if (-not (Test-Path -LiteralPath $p)) { continue }
        if ($KeepFlat -and $p -eq (Join-Path $AssetsDir $FlatRel)) { continue }
        if ($deliverables -contains $p) { continue }
        try {
            Remove-Item -LiteralPath $p -Recurse -Force -ErrorAction Stop
        }
        catch {
            $script:CleanupErrors.Add("cleanup failed for $p : $($_.Exception.Message)")
        }
    }
    $script:Created.Clear()
}

function Register-Path([string]$path) {
    if (-not $script:Created.Contains($path)) {
        $script:Created.Add($path)
    }
}

function Remove-AbandonedProbe {
    $probe = Join-Path $AssetsDir "render-demo-probe.ps1"
    if (Test-Path -LiteralPath $probe) {
        try {
            Remove-Item -LiteralPath $probe -Force -ErrorAction Stop
            Write-Step "removed abandoned render-demo-probe.ps1"
        }
        catch {
            throw "failed to remove abandoned probe: $($_.Exception.Message)"
        }
    }
}

$RunId = [guid]::NewGuid().ToString("N")
$RunRoot = Join-Path $env:TEMP "devlog-demo-$RunId"
if (Test-Path -LiteralPath $RunRoot) {
    throw "run root collision: $RunRoot"
}
New-Item -ItemType Directory -Path $RunRoot | Out-Null
Register-Path $RunRoot

$DemoRoot = Join-Path $RunRoot "repo"
$FakeHome = Join-Path $RunRoot "home"
$ProbeDir = Join-Path $RunRoot "probe"
New-Item -ItemType Directory -Path $ProbeDir | Out-Null

$ForbiddenTermPattern = 'goreleaser|scoop|winget|chocolatey|handoff preview|planner|blueprint|roadmap'

function Assert-ForbiddenTerms([string]$text, [string]$where) {
    if ($text -match "(?i)$ForbiddenTermPattern") {
        throw "forbidden term matched in ${where}: $($Matches[0])"
    }
}

function Write-File {
    param([string]$Path, [string]$Content)
    $dir = Split-Path -Parent $Path
    if (-not (Test-Path -LiteralPath $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
    Set-Content -LiteralPath $Path -Value $Content -NoNewline -Encoding utf8
}

function Format-EventTime([DateTime]$utc) {
    return $utc.ToString("yyyy-MM-dd HH:mm")
}

function Format-FrontTime([DateTime]$utc) {
    return $utc.ToString("yyyy-MM-ddTHH:mm:ssZ")
}

function Write-Session {
    param(
        [string]$Id,
        [string]$Branch,
        [DateTime]$Started,
        [string]$Status,
        [string]$StartMessage,
        [object[]]$Events
    )
    $dir = Join-Path $DemoRoot ".devlog\sessions"
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
    $sb = [System.Text.StringBuilder]::new()
    [void]$sb.AppendLine("---")
    [void]$sb.AppendLine("id: $Id")
    [void]$sb.AppendLine("author: Demo Developer")
    [void]$sb.AppendLine("email: demo@example.com")
    [void]$sb.AppendLine("started: $(Format-FrontTime $Started)")
    [void]$sb.AppendLine("branch: $Branch")
    [void]$sb.AppendLine("status: $Status")
    [void]$sb.AppendLine("---")
    [void]$sb.AppendLine()
    [void]$sb.AppendLine("## Start")
    [void]$sb.AppendLine()
    [void]$sb.AppendLine($StartMessage.TrimEnd())
    $at = $Started
    foreach ($e in $Events) {
        $at = $at.AddMinutes([int]$e.MinutesAfter)
        [void]$sb.AppendLine()
        [void]$sb.AppendLine("## $($e.Type) - $(Format-EventTime $at) UTC")
        [void]$sb.AppendLine($e.Text.TrimEnd())
    }
    $file = Join-Path $dir "$Id.md"
    $content = $sb.ToString().Replace("`r`n", "`n")
    Set-Content -LiteralPath $file -Value $content -NoNewline -Encoding utf8
    Assert-ForbiddenTerms $content "session $Id"
}

function Seed-Repository {
    New-Item -ItemType Directory -Path (Join-Path $DemoRoot "cmd\app") | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $DemoRoot "internal\cache") | Out-Null

    Write-File (Join-Path $DemoRoot "go.mod") @"
module demo-app

go 1.22
"@

    Write-File (Join-Path $DemoRoot "cmd\app\main.go") @"
package main

import (
	"log"
	"net/http"

	"demo-app/internal/cache"
)

func main() {
	c := cache.New()
	http.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if v, ok := c.Get(key); ok {
			w.Write(v)
			return
		}
		log.Printf("fetching key: %s", key)
		http.Error(w, "not found", http.StatusNotFound)
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
"@

    Write-File (Join-Path $DemoRoot "internal\cache\cache.go") @"
package cache

import "sync"

// Cache stores responses keyed by string. Entries are kept for the
// lifetime of the process, which is fine for a bounded key space.
type Cache struct {
	mu    sync.Mutex
	store map[string][]byte
}

func New() *Cache {
	return &Cache{store: make(map[string][]byte)}
}

// Get returns the cached bytes for key, loading fn on a miss.
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.store[key]
	return v, ok
}

// Put stores a value under key.
func (c *Cache) Put(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = value
}
"@

    Push-Location $DemoRoot
    try {
        & $gitPath init -q
        & $gitPath config user.name "Demo Developer"
        & $gitPath config user.email "demo@example.com"
    }
    finally {
        Pop-Location
    }

    $day = (Get-Date).ToUniversalTime().Date
    $commits = @(
        @{ Msg = "Add request handler and cache package"; File = "internal/cache/cache.go"; Content = (Get-Content -Raw (Join-Path $DemoRoot "internal\cache\cache.go")) },
        @{ Msg = "Wire cache into the data endpoint"; File = "cmd/app/main.go"; Content = (Get-Content -Raw (Join-Path $DemoRoot "cmd\app\main.go")) },
        @{ Msg = "Add request logging"; File = "cmd/app/main.go"; Content = (Get-Content -Raw (Join-Path $DemoRoot "cmd\app\main.go")) },
        @{ Msg = "Return cached bytes on hit"; File = "cmd/app/main.go"; Content = (Get-Content -Raw (Join-Path $DemoRoot "cmd\app\main.go")) },
        @{ Msg = "Handle empty key gracefully"; File = "cmd/app/main.go"; Content = (Get-Content -Raw (Join-Path $DemoRoot "cmd\app\main.go")) },
        @{ Msg = "Add basic cache stats"; File = "cmd/app/main.go"; Content = (Get-Content -Raw (Join-Path $DemoRoot "cmd\app\main.go")) },
        @{ Msg = "Document the cache in the README"; File = "cmd/app/main.go"; Content = (Get-Content -Raw (Join-Path $DemoRoot "cmd\app\main.go")) },
        @{ Msg = "Investigate rising memory in production"; File = "cmd/app/main.go"; Content = (Get-Content -Raw (Join-Path $DemoRoot "cmd\app\main.go")) }
    )
    $commitDates = @(
        $day.AddDays(-42).AddHours(10),
        $day.AddDays(-38).AddHours(11),
        $day.AddDays(-33).AddHours(14),
        $day.AddDays(-27).AddHours(9),
        $day.AddDays(-21).AddHours(15),
        $day.AddDays(-15).AddHours(10),
        $day.AddDays(-10).AddHours(16),
        $day.AddDays(-1).AddHours(12)
    )
    $i = 0
    foreach ($c in $commits) {
        Assert-ForbiddenTerms $c.Msg "commit message"
        Push-Location $DemoRoot
        try {
            & $gitPath add -A
            $env:GIT_AUTHOR_DATE = $commitDates[$i].ToString("yyyy-MM-ddTHH:mm:ssZ")
            $env:GIT_COMMITTER_DATE = $env:GIT_AUTHOR_DATE
            & $gitPath commit --allow-empty -q -m $c.Msg
            if ($LASTEXITCODE -ne 0) { throw "git commit failed: $($c.Msg)" }
        }
        finally {
            Remove-Item Env:GIT_AUTHOR_DATE -ErrorAction SilentlyContinue
            Remove-Item Env:GIT_COMMITTER_DATE -ErrorAction SilentlyContinue
            Pop-Location
        }
        $i++
    }
}

function Seed-Sessions {
    $day = (Get-Date).ToUniversalTime().Date

    function Note([string]$text, [int]$min) { @{ Type = "Note"; Text = $text; MinutesAfter = $min } }
    function Blocker([string]$text, [int]$min) { @{ Type = "Blocker"; Text = $text; MinutesAfter = $min } }
    function Stop([string]$text, [int]$min) { @{ Type = "Stop"; Text = $text; MinutesAfter = $min } }

    Write-Session -Id "handler-baseline" -Branch "feat/handler" -Started $day.AddDays(-42).AddHours(9) -Status "stopped" -StartMessage "Stand up the data endpoint" -Events @(
        (Note "Wired /data to read the key query param." 5),
        (Note "Logged every fetch so I could see traffic in dev." 25),
        (Blocker "Empty key panicked the handler." 45),
        (Note "Returned 400 for empty keys instead of crashing." 70),
        (Note "Confirmed the handler responds on localhost." 90),
        (Note "Noted the cache is unbounded for now." 115),
        (Note "Wrote a small curl script to hit the endpoint." 140),
        (Note "Handler tests pass." 165),
        (Note "Decided to keep the handler thin and push logic into the cache." 175),
        (Stop "Endpoint is working, ready for the cache." 185)
    )

    Write-Session -Id "cache-package" -Branch "feat/cache" -Started $day.AddDays(-36).AddHours(13) -Status "stopped" -StartMessage "Build the cache package" -Events @(
        (Note "Started internal/cache with a map[string][]byte." 10),
        (Note "Added a mutex around Get and Put." 30),
        (Note "Get returns the bytes and a bool for misses." 55),
        (Blocker "Concurrent Puts raced and lost entries." 80),
        (Note "Held the lock for the whole write." 100),
        (Note "Kept the API tiny on purpose." 125),
        (Note "Decided not to add TTL yet." 150),
        (Note "Cache unit tests pass." 175),
        (Note "Kept the package small, no exports beyond Get and Put." 185),
        (Stop "Cache package is in and wired up." 195)
    )

    Write-Session -Id "logging-and-stats" -Branch "feat/logging" -Started $day.AddDays(-31).AddHours(10) -Status "stopped" -StartMessage "Add logging and cache stats" -Events @(
        (Note "Logged cache hits and misses separately." 15),
        (Note "Counted entries with a Len() helper." 35),
        (Note "Exposed stats on /debug/cache." 60),
        (Blocker "Stats endpoint had no auth." 85),
        (Note "Tied it to a localhost-only check for now." 105),
        (Note "Watched the entry count climb during a soak test." 130),
        (Note "Figured that was just traffic, not a real issue." 155),
        (Note "Stats and logging tests pass." 180),
        (Note "Entry count is exposed next to hit and miss counts." 190),
        (Stop "Observability is good enough to ship." 200)
    )

    Write-Session -Id "edge-cases" -Branch "fix/edge-cases" -Started $day.AddDays(-25).AddHours(9) -Status "stopped" -StartMessage "Handle edge cases in the cache" -Events @(
        (Note "Nil values now get rejected in Put." 15),
        (Note "Empty keys return early with no write." 35),
        (Blocker "Overwriting a key leaked the old slice." 60),
        (Note "Copied the slice before storing it." 80),
        (Note "Added a test for key overwrite." 105),
        (Note "Checked that large values don't truncate." 130),
        (Note "Verified the mutex isn't held during logging." 155),
        (Note "Edge-case tests pass." 180),
        (Note "Ran a quick race test, no panics." 190),
        (Stop "Cache handles the weird inputs cleanly." 200)
    )

    Write-Session -Id "docs-and-readme" -Branch "docs/cache" -Started $day.AddDays(-19).AddHours(14) -Status "stopped" -StartMessage "Document the cache and endpoint" -Events @(
        (Note "Wrote a short README section on the cache." 10),
        (Note "Noted it's in-memory and per-process." 30),
        (Blocker "Mentioned TTL which doesn't exist yet." 55),
        (Note "Removed the TTL line, clarified it's unbounded." 75),
        (Note "Added a curl example for the endpoint." 100),
        (Note "Documented the debug stats route." 125),
        (Note "Fixed a typo in the handler description." 150),
        (Note "Docs review with the team." 175),
        (Note "Team signed off, no follow-ups." 185),
        (Stop "Docs match what the code actually does." 195)
    )

    Write-Session -Id "load-test" -Branch "chore/load-test" -Started $day.AddDays(-13).AddHours(11) -Status "stopped" -StartMessage "Run a load test before release" -Events @(
        (Note "Wrote a simple load test with varying keys." 15),
        (Note "Ramped to 500 requests per second." 35),
        (Blocker "Memory kept climbing and never came back down." 60),
        (Note "Restarted the process and memory reset." 80),
        (Note "Saved the heap profile." 105),
        (Note "Noticed the entry count matched the key space." 130),
        (Note "Figured the cache was just warming up." 155),
        (Note "Load test passed on throughput." 180),
        (Note "Saved the profile to look at later." 190),
        (Stop "Throughput is fine, but the memory thing is odd." 200)
    )

    Write-Session -Id "metrics-review" -Branch "chore/metrics" -Started $day.AddDays(-8).AddHours(15) -Status "stopped" -StartMessage "Review metrics after a week in prod" -Events @(
        (Note "Pulled the memory graph for the last seven days." 10),
        (Note "Saw a slow steady climb with no plateau." 30),
        (Blocker "Ops paged me, the pod hit its memory limit." 55),
        (Note "Restarted the pod and it recovered." 75),
        (Note "Cross-checked against the entry count metric." 100),
        (Note "Entries grow with unique keys, never decrease." 125),
        (Note "Realized the cache never evicts anything." 150),
        (Note "Filed a ticket to add eviction." 170),
        (Note "Tagged it P2 since restarts paper over it for now." 180),
        (Stop "Confirmed it's a leak, not just warmup." 190)
    )

    Write-Session -Id "leak-repro" -Branch "fix/cache-leak" -Started $day.AddDays(-2).AddHours(9) -Status "stopped" -StartMessage "Reproduce the cache leak locally" -Events @(
        (Note "Wrote a test that inserts 10k unique keys." 15),
        (Note "Memory grew to 40MB and stayed there." 35),
        (Blocker "No eviction means the map only grows." 60),
        (Note "Confirmed Get doesn't touch recency." 80),
        (Note "Tried a manual clear and memory dropped." 105),
        (Note "Narrowed it down to Put never removing old keys." 130),
        (Note "Decided on a simple max-size with oldest-first eviction." 155),
        (Note "Saved the failing repro as a test." 180),
        (Note "Confirmed a manual clear drops memory right away." 190),
        (Stop "Repro is stable, the leak is in Put." 200)
    )

    $activeEvents = @(
        (Note "Picked a max size of 1000 entries." 5),
        (Note "Track insertion order with a slice of keys." 25),
        (Note "On Put, evict the oldest key when over the limit." 45),
        (Blocker "Eviction under concurrent writers corrupted the slice." 70),
        (Note "Hold the lock across evict and insert." 90),
        (Note "Added a Len check after every Put in the test." 110),
        (Note "Ran the 10k-insert repro, memory stayed flat." 130),
        (Note "Evicted keys are gone from the map." 155),
        (Note "Hits on evicted keys miss, which is expected." 175),
        (Note "Wrote a regression test for the size cap." 195),
        (Note "Test inserts 10k keys, asserts Len never exceeds 1000." 220),
        (Note "Test passes with the new eviction in place." 245),
        (Blocker "Need to check hit rate under realistic key spread." 270),
        (Note "Modeled 80/20 key distribution, hit rate holds at 94%." 295),
        (Note "Bumped max size to 2000 to keep hit rate above 95%." 320),
        (Note "Next step is landing the change and watching prod." 345)
    )
    Write-Session -Id "fix-cache-leak" -Branch "fix/cache-leak" -Started $day.AddDays(-1).AddHours(14) -Status "active" -StartMessage "Fix the cache memory leak" -Events $activeEvents
}

function Seed-Todos {
    $day = (Get-Date).ToUniversalTime().Date
    function TodoItem([string]$id, [string]$text, [string]$status, [DateTime]$created, [DateTime]$updated, [Nullable[DateTime]]$completed, [string]$session, [string]$branch) {
        $sb = [System.Text.StringBuilder]::new()
        [void]$sb.AppendLine("- id: $id")
        [void]$sb.AppendLine("  text: $text")
        [void]$sb.AppendLine("  status: $status")
        [void]$sb.AppendLine("  created_at: $(Format-FrontTime $created)")
        [void]$sb.AppendLine("  updated_at: $(Format-FrontTime $updated)")
        if ($completed) {
            [void]$sb.AppendLine("  completed_at: $(Format-FrontTime $completed)")
        }
        [void]$sb.AppendLine("  session_id: $session")
        [void]$sb.AppendLine("  branch: $branch")
        return $sb.ToString()
    }
    $items = @(
        (TodoItem "todo-1" "Land the eviction change and watch prod memory" "open" $day.AddDays(-1).AddHours(15) $day.AddDays(-1).AddHours(15) $null "fix-cache-leak" "fix/cache-leak"),
        (TodoItem "todo-2" "Add a hit-rate dashboard panel" "open" $day.AddDays(-1).AddHours(16) $day.AddDays(-1).AddHours(16) $null "fix-cache-leak" "fix/cache-leak"),
        (TodoItem "todo-3" "Re-test with a realistic key distribution" "open" $day.AddDays(-1).AddHours(17) $day.AddDays(-1).AddHours(17) $null "fix-cache-leak" "fix/cache-leak"),
        (TodoItem "todo-4" "Write the 10k-insert leak repro" "done" $day.AddDays(-2).AddHours(9) $day.AddDays(-2).AddHours(12) $day.AddDays(-2).AddHours(12) "leak-repro" "fix/cache-leak"),
        (TodoItem "todo-5" "Confirm the leak is in Put" "done" $day.AddDays(-2).AddHours(10) $day.AddDays(-2).AddHours(13) $day.AddDays(-2).AddHours(13) "leak-repro" "fix/cache-leak"),
        (TodoItem "todo-6" "Add cache stats endpoint" "done" $day.AddDays(-31).AddHours(10) $day.AddDays(-31).AddHours(13) $day.AddDays(-31).AddHours(13) "logging-and-stats" "feat/logging")
    )
    $dir = Join-Path $DemoRoot ".devlog"
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
    $content = ($items -join "").Replace("`r`n", "`n")
    Set-Content -LiteralPath (Join-Path $dir "todos.md") -Value $content -NoNewline -Encoding utf8
    Assert-ForbiddenTerms $content "todos"
}

function Write-WorkingChanges {
    Write-File (Join-Path $DemoRoot "internal\cache\cache.go") @"
package cache

import "sync"

const maxEntries = 2000

// Cache stores responses keyed by string. When the store grows past
// maxEntries the oldest keys are evicted so memory stays bounded.
type Cache struct {
	mu    sync.Mutex
	store map[string][]byte
	order []string
}

func New() *Cache {
	return &Cache{store: make(map[string][]byte)}
}

// Get returns the cached bytes for key.
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.store[key]
	return v, ok
}

// Put stores a value under key, evicting the oldest entry when full.
func (c *Cache) Put(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.store[key]; !exists {
		c.order = append(c.order, key)
		if len(c.order) > maxEntries {
			drop := c.order[0]
			c.order = c.order[1:]
			delete(c.store, drop)
		}
	}
	c.store[key] = value
}

// Len returns the number of cached entries.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.store)
}
"@

    Write-File (Join-Path $DemoRoot "internal\cache\cache_test.go") @"
package cache

import "testing"

func TestPutEvictsOldestEntries(t *testing.T) {
	c := New()
	for i := 0; i < 10000; i++ {
		c.Put(string(rune(i)), []byte{byte(i)})
	}
	if got := c.Len(); got > maxEntries {
		t.Fatalf("cache grew unbounded: Len=%d, want <= %d", got, maxEntries)
	}
}
"@
}

function Build-Devlog {
    Push-Location $RepoRoot
    try {
        & $goPath build -ldflags "-X main.version=v1.0.0" -o (Join-Path $DemoRoot "devlog.exe") ./cmd/devlog
        if ($LASTEXITCODE -ne 0) { throw "devlog build failed" }
    }
    finally {
        Pop-Location
    }
}

function Count-SessionEvents {
    param([string]$file)
    $content = Get-Content -Raw -LiteralPath $file
    $notes = ([regex]::Matches($content, "(?m)^## Note - ")).Count
    $blockers = ([regex]::Matches($content, "(?m)^## Blocker - ")).Count
    $stops = ([regex]::Matches($content, "(?m)^## Stop - ")).Count
    $starts = ([regex]::Matches($content, "(?m)^## Start$")).Count
    return @{ Notes = $notes; Blockers = $blockers; Stops = $stops; Starts = $starts }
}

function Run-Preflights {
    $sessionDir = Join-Path $DemoRoot ".devlog\sessions"
    $files = Get-ChildItem -LiteralPath $sessionDir -Filter "*.md" | Sort-Object Name
    if ($files.Count -ne 9) {
        throw "expected 9 session files, got $($files.Count)"
    }
    $stopped = 0
    $activeCount = 0
    foreach ($f in $files) {
        $c = Count-SessionEvents $f.FullName
        if ($c.Starts -ne 1) { throw "session $($f.BaseName) has $($c.Starts) start events" }
        if ($c.Stops -eq 0) {
            $activeCount++
            $visible = $c.Notes + $c.Blockers
            if ($visible -ne 16) { throw "active session has $visible pre-recorded events, want 16" }
            if ($f.BaseName -ne "fix-cache-leak") { throw "unexpected active session $($f.BaseName)" }
        }
        else {
            $stopped++
            $visible = $c.Notes + $c.Blockers + $c.Stops
            if ($visible -ne 10) { throw "stopped session $($f.BaseName) has $visible events, want 10" }
        }
    }
    if ($stopped -ne 8) { throw "expected 8 stopped sessions, got $stopped" }
    if ($activeCount -ne 1) { throw "expected 1 active session, got $activeCount" }

    $todoFile = Join-Path $DemoRoot ".devlog\todos.md"
    $todoContent = Get-Content -Raw -LiteralPath $todoFile
    $open = ([regex]::Matches($todoContent, "(?m)^  status: open$")).Count
    $done = ([regex]::Matches($todoContent, "(?m)^  status: done$")).Count
    if ($open -ne 3 -or $done -ne 3) { throw "expected 3 open and 3 done todos, got $open/$done" }

    $envBlock = @{
        PATH        = $DemoRoot + ";" + $env:PATH
        HOME        = $FakeHome
        USERPROFILE = $FakeHome
    }
    $preflightCmd = @"
`$env:PATH = '$($envBlock.PATH -replace "'", "''")'
`$env:HOME = '$FakeHome'
`$env:USERPROFILE = '$FakeHome'
& '$(Join-Path $DemoRoot "devlog.exe")' list
if (`$LASTEXITCODE -ne 0) { exit `$LASTEXITCODE }
& '$(Join-Path $DemoRoot "devlog.exe")' todo list
exit `$LASTEXITCODE
"@
    $out = & $pwshPath -NoProfile -Command $preflightCmd 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "devlog preflight failed: $out"
    }
    Write-Step "preflight: 9 sessions, 1 active, 8 stopped with 10 events, 6 todos (3 open/3 done)"
}

function Invoke-Seed {
    Seed-Repository
    Seed-Sessions
    Seed-Todos
    Write-WorkingChanges
    Build-Devlog
    Run-Preflights
}

$ToolPathEntries = @()
if (Test-Path -LiteralPath (Join-Path $ttydDir "ttyd.exe")) {
    $ToolPathEntries += $ttydDir
}
$ToolPathEntries += (Split-Path -Parent $vhsPath)
$ToolPathEntries += (Split-Path -Parent $ffmpegPath)
$ToolPathEntries += $DemoRoot

function New-EnvOverrides {
    $overrides = [System.Collections.Generic.Dictionary[string, string]]::new([System.StringComparer]::OrdinalIgnoreCase)
    $overrides["PATH"] = (($ToolPathEntries -join ";") + ";" + $env:PATH)
    $overrides["HOME"] = $FakeHome
    $overrides["USERPROFILE"] = $FakeHome
    $overrides["DEVLOG_DEMO_ROOT"] = $DemoRoot
    $overrides["DEVLOG_FAKE_HOME"] = $FakeHome
    return $overrides
}

$ProcessTreeCSharp = @'
using System;
using System.Collections.Generic;
using System.IO;
using Microsoft.Win32.SafeHandles;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;
using System.Threading.Tasks;

namespace DemoRender
{
    public enum ProbeOptions { None, AssignmentFailure, DrainCancellation, DrainQuarantine }

    public sealed class ProcessTreeResult
    {
        public string RootOutcome = "LaunchFailed";
        public int? ExitCode;
        public int RootPid;
        public int FinalActiveProcesses;
        public bool Quarantined;
        public bool DrainsTerminal;
        public string Stdout = "";
        public string Stderr = "";
        public string StdoutDrainState = "not-started";
        public string StderrDrainState = "not-started";
        public List<string> Errors = new List<string>();
    }

    public static class ProcessTreeRunner
    {
        private const uint GENERIC_READ = 0x80000000;
        private const uint GENERIC_WRITE = 0x40000000;
        private const uint FILE_SHARE_READ = 0x1;
        private const uint FILE_SHARE_WRITE = 0x2;
        private const uint OPEN_EXISTING = 3;
        private const uint FILE_FLAG_OVERLAPPED = 0x40000000;
        private const uint PIPE_ACCESS_INBOUND = 0x1;
        private const uint PIPE_TYPE_BYTE = 0x0;
        private const uint PIPE_READMODE_BYTE = 0x0;
        private const uint PIPE_WAIT = 0x0;
        private const uint ERROR_IO_PENDING = 997;
        private const uint WAIT_OBJECT_0 = 0;
        private const uint WAIT_TIMEOUT = 0x102;
        private const uint WAIT_FAILED = 0xFFFFFFFF;
        private const uint CREATE_SUSPENDED = 0x4;
        private const uint CREATE_UNICODE_ENVIRONMENT = 0x400;
        private const uint EXTENDED_STARTUPINFO_PRESENT = 0x80000;
        private const int STARTF_USESTDHANDLES = 0x100;
        private const int ERROR_INSUFFICIENT_BUFFER = 122;
        private const uint PROC_THREAD_ATTRIBUTE_HANDLE_LIST = 0x20002;
        private const int STILL_ACTIVE = 259;
        private const int JobObjectExtendedLimitInformation = 9;
        private const uint JobObjectBasicAccountingInformation = 1;
        private const uint JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE = 0x2000;
        private const uint HANDLE_FLAG_INHERIT = 0x1;
        private const uint DUPLICATE_SAME_ACCESS = 0x2;

        [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        private static extern IntPtr CreateNamedPipeW(string name, uint openMode, uint pipeMode, uint maxInstances, uint outBuffer, uint inBuffer, uint defaultTimeout, IntPtr security);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool ConnectNamedPipe(IntPtr pipe, ref OVERLAPPED overlapped);
        [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        private static extern IntPtr CreateFileW(string name, uint access, uint share, IntPtr security, uint creation, uint flags, IntPtr template);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool GetOverlappedResult(IntPtr handle, ref OVERLAPPED overlapped, out uint bytes, bool wait);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool CancelIoEx(IntPtr handle, IntPtr overlapped);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern IntPtr CreateEventW(IntPtr security, bool manualReset, bool initialState, string name);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern uint WaitForSingleObject(IntPtr handle, uint milliseconds);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool CloseHandle(IntPtr handle);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool SetHandleInformation(IntPtr handle, uint mask, uint flags);
        [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        private static extern bool CreateProcessW(string appName, StringBuilder cmdLine, IntPtr procAttr, IntPtr threadAttr, bool inherit, uint flags, IntPtr environment, string workDir, ref STARTUPINFOEX si, out PROCESS_INFORMATION pi);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool InitializeProcThreadAttributeList(IntPtr list, int count, int flags, out IntPtr size);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool UpdateProcThreadAttribute(IntPtr list, uint flags, uint attr, IntPtr value, IntPtr size, IntPtr prev, IntPtr reserved);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool DeleteProcThreadAttributeList(IntPtr list);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern IntPtr CreateJobObjectW(IntPtr security, string name);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool SetInformationJobObject(IntPtr job, int cls, IntPtr info, uint size);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool AssignProcessToJobObject(IntPtr job, IntPtr process);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool TerminateJobObject(IntPtr job, uint code);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool TerminateProcess(IntPtr process, uint code);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern uint ResumeThread(IntPtr thread);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool GetExitCodeProcess(IntPtr process, out uint code);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool QueryInformationJobObject(IntPtr job, uint cls, IntPtr info, uint size, out uint returned);
        [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        private static extern IntPtr GetEnvironmentStringsW();
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool FreeEnvironmentStringsW(IntPtr block);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool DuplicateHandle(IntPtr sourceProcess, IntPtr sourceHandle, IntPtr targetProcess, out IntPtr targetHandle, uint access, bool inherit, uint options);
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern IntPtr GetCurrentProcess();
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern IntPtr LocalFree(IntPtr block);

        [StructLayout(LayoutKind.Sequential)]
        private struct OVERLAPPED { public UIntPtr Internal; public UIntPtr InternalHigh; public uint Offset; public uint OffsetHigh; public IntPtr Event; }

        [StructLayout(LayoutKind.Sequential)]
        private struct SECURITY_ATTRIBUTES { public int nLength; public IntPtr lpSecurityDescriptor; public bool bInheritHandle; }

        [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
        private struct STARTUPINFO
        {
            public int cb; public string lpReserved; public string lpDesktop; public string lpTitle;
            public int dwX; public int dwY; public int dwXSize; public int dwYSize;
            public int dwXCountChars; public int dwYCountChars; public int dwFillAttribute; public int dwFlags;
            public short wShowWindow; public short cbReserved2; public IntPtr lpReserved2;
            public IntPtr hStdInput; public IntPtr hStdOutput; public IntPtr hStdError;
        }

        [StructLayout(LayoutKind.Sequential)]
        private struct STARTUPINFOEX { public STARTUPINFO StartupInfo; public IntPtr lpAttributeList; }

        [StructLayout(LayoutKind.Sequential)]
        private struct PROCESS_INFORMATION { public IntPtr hProcess; public IntPtr hThread; public uint dwProcessId; public uint dwThreadId; }

        [StructLayout(LayoutKind.Sequential)]
        private struct JOBOBJECT_BASIC_ACCOUNTING_INFORMATION
        {
            public long TotalUserTime; public long TotalKernelTime;
            public long ThisPeriodTotalUserTime; public long ThisPeriodTotalKernelTime;
            public uint TotalPageFaultCount; public uint TotalProcesses;
            public uint ActiveProcesses; public uint TotalTerminatedProcesses;
        }

        [StructLayout(LayoutKind.Sequential)]
        private struct JOBOBJECT_BASIC_LIMIT_INFORMATION
        {
            public long PerProcessUserTimeLimit; public long PerJobUserTimeLimit; public uint LimitFlags;
            public UIntPtr MinimumWorkingSetSize; public UIntPtr MaximumWorkingSetSize;
            public uint ActiveProcessLimit; public UIntPtr Affinity;
            public uint PriorityClass; public uint SchedulingClass;
        }

        [StructLayout(LayoutKind.Sequential)]
        private struct IO_COUNTERS
        {
            public ulong ReadOperationCount; public ulong WriteOperationCount; public ulong OtherOperationCount;
            public ulong ReadTransferCount; public ulong WriteTransferCount; public ulong OtherTransferCount;
        }

        [StructLayout(LayoutKind.Sequential)]
        private struct JOBOBJECT_EXTENDED_LIMIT_INFORMATION
        {
            public JOBOBJECT_BASIC_LIMIT_INFORMATION BasicLimitInformation;
            public IO_COUNTERS IoInfo;
            public UIntPtr ProcessMemoryLimit; public UIntPtr JobMemoryLimit;
            public UIntPtr PeakProcessMemoryUsed; public UIntPtr PeakJobMemoryUsed;
        }

        private sealed class PipePair
        {
            public IntPtr Server;
            public IntPtr Event;
            public OVERLAPPED Connect;
            public IntPtr ChildWrite;
            public IntPtr Duplicate;
        }

        private sealed class QuarantinedRootEntry
        {
            public IntPtr hProcess;
            public IntPtr hThread;
        }

        private static readonly Dictionary<int, QuarantinedRootEntry> QuarantinedRoots = new Dictionary<int, QuarantinedRootEntry>();
        private static readonly List<KeyValuePair<Task, CancellationTokenSource>> QuarantinedDrains = new List<KeyValuePair<Task, CancellationTokenSource>>();

        public static ProcessTreeResult Run(string executable, IReadOnlyList<string> arguments, string workingDirectory, IReadOnlyDictionary<string, string> overrides, int normalTimeoutMs)
        {
            return Run(executable, arguments, workingDirectory, overrides, normalTimeoutMs, ProbeOptions.None);
        }

        public static ProcessTreeResult Run(string executable, IReadOnlyList<string> arguments, string workingDirectory, IReadOnlyDictionary<string, string> overrides, int normalTimeoutMs, ProbeOptions options)
        {
            var result = new ProcessTreeResult();
            bool probe = options != ProbeOptions.None;
            int rootWaitMs = probe ? 100 : normalTimeoutMs;
            int pollDeadlineMs = probe ? 100 : 10000;
            int drainWaitMs = probe ? 100 : 10000;
            int teardownMs = 10000;
            var errors = result.Errors;

            IntPtr hJob = IntPtr.Zero;
            IntPtr hStdin = IntPtr.Zero;
            IntPtr attrList = IntPtr.Zero;
            IntPtr handlesPtr = IntPtr.Zero;
            IntPtr envBlock = IntPtr.Zero;
            PipePair pipeOut = null;
            PipePair pipeErr = null;
            PROCESS_INFORMATION pi = default(PROCESS_INFORMATION);
            FileStream fsOut = null;
            FileStream fsErr = null;
            StreamReader srOut = null;
            StreamReader srErr = null;
            Task<string> tOut = null;
            Task<string> tErr = null;
            CancellationTokenSource cts = null;
            bool assigned = false;
            bool dupKept = false;

            Action<string> addError = (msg) => { errors.Add(msg); };

            try
            {
                hJob = CreateJobObjectW(IntPtr.Zero, null);
                if (hJob == IntPtr.Zero) { addError("(1) CreateJobObjectW: " + ErrText()); return result; }
                var ext = new JOBOBJECT_EXTENDED_LIMIT_INFORMATION();
                ext.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
                IntPtr extPtr = Marshal.AllocHGlobal(Marshal.SizeOf(typeof(JOBOBJECT_EXTENDED_LIMIT_INFORMATION)));
                Marshal.StructureToPtr(ext, extPtr, false);
                bool jobOk = SetInformationJobObject(hJob, JobObjectExtendedLimitInformation, extPtr, (uint)Marshal.SizeOf(typeof(JOBOBJECT_EXTENDED_LIMIT_INFORMATION)));
                Marshal.FreeHGlobal(extPtr);
                if (!jobOk) { addError("(1) SetInformationJobObject kill-on-close: " + ErrText()); return result; }

                var saInherit = new SECURITY_ATTRIBUTES();
                saInherit.nLength = Marshal.SizeOf(typeof(SECURITY_ATTRIBUTES));
                saInherit.bInheritHandle = true;
                IntPtr pSaInherit = Marshal.AllocHGlobal(Marshal.SizeOf(typeof(SECURITY_ATTRIBUTES)));
                Marshal.StructureToPtr(saInherit, pSaInherit, false);

                int pipeSeq = Environment.TickCount & 0x7fffffff;
                string pipeBase = @"\\.\pipe\devlog-demo-" + pipeSeq + "-" + pidOfSelf();
                try
                {
                    pipeOut = CreatePipePair(pipeBase + "-out", pSaInherit, ref errors);
                    pipeErr = CreatePipePair(pipeBase + "-err", pSaInherit, ref errors);
                    if (errors.Count > 0) return result;

                    hStdin = CreateFileW("NUL", GENERIC_READ, FILE_SHARE_READ | FILE_SHARE_WRITE, pSaInherit, OPEN_EXISTING, 0, IntPtr.Zero);
                    if (hStdin == IntPtr.Zero || hStdin == new IntPtr(-1)) { addError("(1) open NUL stdin: " + ErrText()); return result; }

                    envBlock = BuildEnvironmentBlock(overrides, ref errors);
                    if (errors.Count > 0) return result;

                    int attrCount = 1;
                    IntPtr attrSize;
                    if (!InitializeProcThreadAttributeList(IntPtr.Zero, attrCount, 0, out attrSize))
                    {
                        int sizeErr = Marshal.GetLastWin32Error();
                        if (sizeErr != ERROR_INSUFFICIENT_BUFFER) { addError("(1) InitializeProcThreadAttributeList size: " + sizeErr); return result; }
                    }
                    attrList = Marshal.AllocHGlobal(attrSize);
                    if (!InitializeProcThreadAttributeList(attrList, attrCount, 0, out attrSize))
                    { addError("(1) InitializeProcThreadAttributeList: " + ErrText()); return result; }
                    IntPtr[] handles = new IntPtr[] { hStdin, pipeOut.ChildWrite, pipeErr.ChildWrite };
                    handlesPtr = Marshal.AllocHGlobal(IntPtr.Size * handles.Length);
                    Marshal.Copy(handles, 0, handlesPtr, handles.Length);
                    if (!UpdateProcThreadAttribute(attrList, 0, PROC_THREAD_ATTRIBUTE_HANDLE_LIST, handlesPtr, (IntPtr)(IntPtr.Size * handles.Length), IntPtr.Zero, IntPtr.Zero))
                    { addError("(1) UpdateProcThreadAttribute: " + ErrText()); return result; }

                    var siEx = new STARTUPINFOEX();
                    siEx.StartupInfo.cb = Marshal.SizeOf(typeof(STARTUPINFOEX));
                    siEx.StartupInfo.dwFlags = STARTF_USESTDHANDLES;
                    siEx.StartupInfo.hStdInput = hStdin;
                    siEx.StartupInfo.hStdOutput = pipeOut.ChildWrite;
                    siEx.StartupInfo.hStdError = pipeErr.ChildWrite;
                    siEx.lpAttributeList = attrList;

                    StringBuilder cmdLine = BuildCommandLine(executable, arguments);
                    if (!CreateProcessW(executable, cmdLine, IntPtr.Zero, IntPtr.Zero, true,
                        CREATE_SUSPENDED | CREATE_UNICODE_ENVIRONMENT | EXTENDED_STARTUPINFO_PRESENT,
                        envBlock, workingDirectory, ref siEx, out pi))
                    { addError("(1) CreateProcessW: " + ErrText()); return result; }
                    result.RootPid = (int)pi.dwProcessId;

                    if (options == ProbeOptions.DrainCancellation || options == ProbeOptions.DrainQuarantine)
                    {
                        IntPtr dup;
                        if (!DuplicateHandle(GetCurrentProcess(), pipeOut.ChildWrite, GetCurrentProcess(), out dup, 0, false, DUPLICATE_SAME_ACCESS))
                        { addError("(1) DuplicateHandle drain probe: " + ErrText()); return result; }
                        pipeOut.Duplicate = dup;
                        dupKept = true;
                    }

                    CloseHandle(pipeOut.ChildWrite);
                    pipeOut.ChildWrite = IntPtr.Zero;
                    CloseHandle(pipeErr.ChildWrite);
                    pipeErr.ChildWrite = IntPtr.Zero;
                    CloseHandle(hStdin);
                    hStdin = IntPtr.Zero;

                    fsOut = new FileStream(new SafeFileHandle(pipeOut.Server, true), FileAccess.Read, 4096, true);
                    fsErr = new FileStream(new SafeFileHandle(pipeErr.Server, true), FileAccess.Read, 4096, true);
                    pipeOut.Server = IntPtr.Zero;
                    pipeErr.Server = IntPtr.Zero;
                    srOut = new StreamReader(fsOut, Encoding.UTF8, false, 1024, false);
                    srErr = new StreamReader(fsErr, Encoding.UTF8, false, 1024, false);
                    cts = new CancellationTokenSource();
                    tOut = ReadToEndAsync(srOut, cts.Token);
                    tErr = ReadToEndAsync(srErr, cts.Token);
                    result.StdoutDrainState = "started";
                    result.StderrDrainState = "started";

                    if (options == ProbeOptions.AssignmentFailure)
                    {
                        addError("(1) synthetic assignment failure");
                    }
                    else
                    {
                        assigned = AssignProcessToJobObject(hJob, pi.hProcess);
                        if (!assigned)
                        {
                            addError("(1) AssignProcessToJobObject: " + ErrText());
                        }
                    }

                    if (!assigned)
                    {
                        string termError = null;
                        if (!TerminateProcess(pi.hProcess, 1))
                        {
                            termError = "TerminateProcess failed: " + ErrText();
                        }
                        else
                        {
                            bool exited = WaitForProcessExit(pi.hProcess, teardownMs, out uint code);
                            if (!exited)
                            {
                                termError = "suspended root did not exit after termination within " + teardownMs + "ms";
                            }
                        }
                        if (termError != null)
                        {
                            QuarantinedRoots[result.RootPid] = new QuarantinedRootEntry { hProcess = pi.hProcess, hThread = pi.hThread };
                            pi.hProcess = IntPtr.Zero;
                            pi.hThread = IntPtr.Zero;
                            result.Quarantined = true;
                            result.RootOutcome = "Quarantined";
                            addError("(10) quarantined root pid " + result.RootPid + " retained: " + termError);
                            return result;
                        }
                        result.RootOutcome = "AssignmentFailed";
                        CloseHandle(pi.hThread);
                        pi.hThread = IntPtr.Zero;
                        CloseHandle(pi.hProcess);
                        pi.hProcess = IntPtr.Zero;
                        return result;
                    }

                    if (ResumeThread(pi.hThread) == 0xFFFFFFFF)
                    {
                        addError("(2) ResumeThread failed: " + ErrText());
                    }
                    CloseHandle(pi.hThread);
                    pi.hThread = IntPtr.Zero;

                    uint waitCode = WaitForSingleObject(pi.hProcess, (uint)rootWaitMs);
                    if (waitCode == WAIT_OBJECT_0)
                    {
                        result.RootOutcome = "Exited";
                        uint exitCode;
                        if (!GetExitCodeProcess(pi.hProcess, out exitCode))
                        {
                            addError("(3) GetExitCodeProcess: " + ErrText());
                        }
                        else
                        {
                            result.ExitCode = (int)exitCode;
                            if (exitCode != 0)
                            {
                                addError("(3) unexpected non-zero exit code " + exitCode);
                            }
                        }
                    }
                    else if (waitCode == WAIT_TIMEOUT)
                    {
                        result.RootOutcome = "TimedOut";
                        addError("(2) root wait timed out after " + rootWaitMs + "ms");
                    }
                    else
                    {
                        result.RootOutcome = "WaitFailed";
                        addError("(2) root wait failed: " + ErrText());
                    }

                    int active = QueryActiveProcesses(hJob);
                    if (result.RootOutcome == "Exited" && result.ExitCode == 0)
                    {
                        if (active != 0)
                        {
                            addError("(4) job still has " + active + " active processes after root exit");
                            TerminateJobObject(hJob, 1);
                            addError("(5) termination request issued");
                            if (!TerminateAndProve(hJob, pi.hProcess, pollDeadlineMs, teardownMs, result, addError)) return result;
                        }
                        else
                        {
                            result.FinalActiveProcesses = 0;
                        }
                    }
                    else
                    {
                        if (active != 0)
                        {
                            addError("(4) job still has " + active + " active processes after root outcome");
                        }
                        TerminateJobObject(hJob, 1);
                        addError("(5) termination request issued");
                        if (!TerminateAndProve(hJob, pi.hProcess, pollDeadlineMs, teardownMs, result, addError)) return result;
                    }

                    bool drainsOk = AwaitDrains(tOut, tErr, srOut, srErr, fsOut, fsErr, pipeOut, pipeErr, cts, drainWaitMs, teardownMs, result, options, addError);
                    if (result.Quarantined) return result;
                    if (!drainsOk)
                    {
                        return result;
                    }

                    try
                    {
                        if (dupKept && pipeOut.Duplicate != IntPtr.Zero)
                        {
                            CloseHandle(pipeOut.Duplicate);
                            pipeOut.Duplicate = IntPtr.Zero;
                        }
                        srOut.Dispose(); srErr.Dispose();
                        fsOut.Dispose(); fsErr.Dispose();
                        srOut = null; srErr = null; fsOut = null; fsErr = null;
                        CloseHandle(pipeOut.Event); pipeOut.Event = IntPtr.Zero;
                        CloseHandle(pipeErr.Event); pipeErr.Event = IntPtr.Zero;
                        if (pi.hProcess != IntPtr.Zero) { CloseHandle(pi.hProcess); pi.hProcess = IntPtr.Zero; }
                    }
                    catch (Exception ex)
                    {
                        addError("(10) disposal failed: " + ex.Message);
                    }
                    return result;
                }
                finally
                {
                    if (attrList != IntPtr.Zero)
                    {
                        DeleteProcThreadAttributeList(attrList);
                        Marshal.FreeHGlobal(attrList);
                        attrList = IntPtr.Zero;
                    }
                    if (handlesPtr != IntPtr.Zero) { Marshal.FreeHGlobal(handlesPtr); handlesPtr = IntPtr.Zero; }
                    if (envBlock != IntPtr.Zero) { Marshal.FreeHGlobal(envBlock); envBlock = IntPtr.Zero; }
                    Marshal.FreeHGlobal(pSaInherit);
                    if (pipeOut != null)
                    {
                        if (pipeOut.Server != IntPtr.Zero) CloseHandle(pipeOut.Server);
                        if (pipeOut.ChildWrite != IntPtr.Zero) CloseHandle(pipeOut.ChildWrite);
                        if (pipeOut.Event != IntPtr.Zero) CloseHandle(pipeOut.Event);
                        if (pipeOut.Duplicate != IntPtr.Zero && dupKept) CloseHandle(pipeOut.Duplicate);
                    }
                    if (pipeErr != null)
                    {
                        if (pipeErr.Server != IntPtr.Zero) CloseHandle(pipeErr.Server);
                        if (pipeErr.ChildWrite != IntPtr.Zero) CloseHandle(pipeErr.ChildWrite);
                        if (pipeErr.Event != IntPtr.Zero) CloseHandle(pipeErr.Event);
                    }
                    if (hStdin != IntPtr.Zero) CloseHandle(hStdin);
                    if (pi.hThread != IntPtr.Zero) CloseHandle(pi.hThread);
                    if (pi.hProcess != IntPtr.Zero) CloseHandle(pi.hProcess);
                    if (hJob != IntPtr.Zero) CloseHandle(hJob);
                    if (cts != null && !result.Quarantined) cts.Dispose();
                }
            }
            catch (Exception ex)
            {
                addError("(10) wrapper fault: " + ex.Message);
                return result;
            }
        }

        private static int pidOfSelf()
        {
            try { return System.Diagnostics.Process.GetCurrentProcess().Id; }
            catch { return 0; }
        }

        private static string ErrText()
        {
            int code = Marshal.GetLastWin32Error();
            return "win32 error " + code;
        }

        private static PipePair CreatePipePair(string name, IntPtr saInherit, ref List<string> errors)
        {
            var pair = new PipePair();
            pair.Server = CreateNamedPipeW(name, PIPE_ACCESS_INBOUND | FILE_FLAG_OVERLAPPED,
                PIPE_TYPE_BYTE | PIPE_READMODE_BYTE | PIPE_WAIT, 1, 4096, 4096, 0, IntPtr.Zero);
            if (pair.Server == IntPtr.Zero || pair.Server == new IntPtr(-1))
            {
                errors.Add("(1) CreateNamedPipeW " + name + ": " + ErrText());
                return pair;
            }
            SetHandleInformation(pair.Server, HANDLE_FLAG_INHERIT, 0);
            pair.Event = CreateEventW(IntPtr.Zero, true, false, null);
            if (pair.Event == IntPtr.Zero)
            {
                errors.Add("(1) CreateEventW: " + ErrText());
                return pair;
            }
            pair.Connect = new OVERLAPPED();
            pair.Connect.Event = pair.Event;
            if (!ConnectNamedPipe(pair.Server, ref pair.Connect))
            {
                if (Marshal.GetLastWin32Error() != ERROR_IO_PENDING)
                {
                    errors.Add("(1) ConnectNamedPipe: " + ErrText());
                    return pair;
                }
            }
            pair.ChildWrite = CreateFileW(name, GENERIC_WRITE, FILE_SHARE_READ | FILE_SHARE_WRITE, saInherit, OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL_VALUE(), IntPtr.Zero);
            if (pair.ChildWrite == IntPtr.Zero || pair.ChildWrite == new IntPtr(-1))
            {
                errors.Add("(1) CreateFileW pipe client " + name + ": " + ErrText());
                return pair;
            }
            uint wait = WaitForSingleObject(pair.Event, 10000);
            if (wait != WAIT_OBJECT_0)
            {
                errors.Add("(1) pipe connection wait: " + ErrText());
                return pair;
            }
            uint transferred;
            if (!GetOverlappedResult(pair.Server, ref pair.Connect, out transferred, true))
            {
                errors.Add("(1) GetOverlappedResult pipe " + name + ": " + ErrText());
                return pair;
            }
            return pair;
        }

        private static uint FILE_ATTRIBUTE_NORMAL_VALUE()
        {
            return 0x80;
        }

        private static StringBuilder BuildCommandLine(string executable, IReadOnlyList<string> arguments)
        {
            var sb = new StringBuilder();
            sb.Append(QuoteArg(executable));
            foreach (var a in arguments)
            {
                sb.Append(' ');
                sb.Append(QuoteArg(a));
            }
            return sb;
        }

        private static string QuoteArg(string arg)
        {
            if (arg.Length == 0) return "\"\"";
            bool needsQuotes = false;
            for (int i = 0; i < arg.Length; i++)
            {
                char c = arg[i];
                if (c == ' ' || c == '\t' || c == '"') { needsQuotes = true; break; }
            }
            if (!needsQuotes) return arg;
            var sb = new StringBuilder();
            sb.Append('"');
            int backslashes = 0;
            for (int i = 0; i < arg.Length; i++)
            {
                char c = arg[i];
                if (c == '\\')
                {
                    backslashes++;
                    continue;
                }
                if (c == '"')
                {
                    sb.Append('\\', backslashes * 2 + 1);
                    sb.Append('"');
                    backslashes = 0;
                    continue;
                }
                sb.Append('\\', backslashes);
                backslashes = 0;
                sb.Append(c);
            }
            sb.Append('\\', backslashes * 2);
            sb.Append('"');
            return sb.ToString();
        }

        private static IntPtr BuildEnvironmentBlock(IReadOnlyDictionary<string, string> overrides, ref List<string> errors)
        {
            IntPtr raw = GetEnvironmentStringsW();
            if (raw == IntPtr.Zero)
            {
                errors.Add("(1) GetEnvironmentStringsW: " + ErrText());
                return IntPtr.Zero;
            }
            var entries = new List<KeyValuePair<string, string>>();
            try
            {
                IntPtr p = raw;
                while (true)
                {
                    int len = 0;
                    while (Marshal.ReadInt16(p, len * 2) != 0) len++;
                    if (len == 0) break;
                    string entry = Marshal.PtrToStringUni(p, len);
                    int eq = entry.IndexOf('=');
                    if (eq < 0) { p = IntPtr.Add(p, (len + 1) * 2); continue; }
                    string key = entry.Substring(0, eq);
                    string value = entry.Substring(eq + 1);
                    entries.Add(new KeyValuePair<string, string>(key, value));
                    p = IntPtr.Add(p, (len + 1) * 2);
                }
            }
            finally
            {
                FreeEnvironmentStringsW(raw);
            }
            var map = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase);
            foreach (var kv in entries)
            {
                map[kv.Key] = kv.Value;
            }
            foreach (var ov in overrides)
            {
                string existingKey = null;
                foreach (var key in map.Keys)
                {
                    if (string.Equals(key, ov.Key, StringComparison.OrdinalIgnoreCase) && !key.StartsWith("="))
                    {
                        existingKey = key;
                        break;
                    }
                }
                if (existingKey != null) map.Remove(existingKey);
                map[ov.Key] = ov.Value;
            }
            var sorted = new List<KeyValuePair<string, string>>(map);
            sorted.Sort((a, b) => string.Compare(a.Key, b.Key, StringComparison.OrdinalIgnoreCase));
            int total = 0;
            foreach (var kv in sorted) total += kv.Key.Length + kv.Value.Length + 2;
            total += 2;
            IntPtr block = Marshal.AllocHGlobal(total * 2);
            IntPtr p2 = block;
            foreach (var kv in sorted)
            {
                string line = kv.Key + "=" + kv.Value;
                byte[] bytes = Encoding.Unicode.GetBytes(line);
                Marshal.Copy(bytes, 0, p2, bytes.Length);
                p2 = IntPtr.Add(p2, bytes.Length);
                Marshal.WriteInt16(p2, 0);
                p2 = IntPtr.Add(p2, 2);
            }
            Marshal.WriteInt16(p2, 0);
            return block;
        }

        private static bool WaitForProcessExit(IntPtr hProcess, int timeoutMs, out uint exitCode)
        {
            exitCode = 0;
            uint wait = WaitForSingleObject(hProcess, (uint)timeoutMs);
            if (wait != WAIT_OBJECT_0) return false;
            if (!GetExitCodeProcess(hProcess, out exitCode)) return false;
            return exitCode != STILL_ACTIVE;
        }

        private static int QueryActiveProcesses(IntPtr hJob)
        {
            IntPtr info = Marshal.AllocHGlobal(Marshal.SizeOf(typeof(JOBOBJECT_BASIC_ACCOUNTING_INFORMATION)));
            try
            {
                Marshal.StructureToPtr(new JOBOBJECT_BASIC_ACCOUNTING_INFORMATION(), info, false);
                uint returned;
                if (!QueryInformationJobObject(hJob, JobObjectBasicAccountingInformation, info, (uint)Marshal.SizeOf(typeof(JOBOBJECT_BASIC_ACCOUNTING_INFORMATION)), out returned))
                {
                    return -1;
                }
                var ba = (JOBOBJECT_BASIC_ACCOUNTING_INFORMATION)Marshal.PtrToStructure(info, typeof(JOBOBJECT_BASIC_ACCOUNTING_INFORMATION));
                return (int)ba.ActiveProcesses;
            }
            finally
            {
                Marshal.FreeHGlobal(info);
            }
        }

        private static bool TerminateAndProve(IntPtr hJob, IntPtr hProcess, int pollDeadlineMs, int teardownMs, ProcessTreeResult result, Action<string> addError)
        {
            bool rootExited = WaitForProcessExit(hProcess, teardownMs, out uint _);
            if (!rootExited)
            {
                addError("(6) root did not exit within " + teardownMs + "ms after Job termination");
                return false;
            }
            if (result.ExitCode == null && result.RootOutcome != "Exited")
            {
                uint code;
                if (GetExitCodeProcess(hProcess, out code)) result.ExitCode = (int)code;
            }
            long deadline = Environment.TickCount64 + pollDeadlineMs;
            int active = -1;
            while (true)
            {
                active = QueryActiveProcesses(hJob);
                if (active == 0) break;
                if (Environment.TickCount64 >= deadline) break;
                Thread.Sleep(25);
            }
            result.FinalActiveProcesses = active == -1 ? -1 : active;
            if (active != 0)
            {
                addError("(7) job active process count did not reach zero after termination");
                return false;
            }
            return true;
        }

        private static bool AwaitDrains(
            Task<string> tOut, Task<string> tErr,
            StreamReader srOut, StreamReader srErr,
            FileStream fsOut, FileStream fsErr,
            PipePair pipeOut, PipePair pipeErr,
            CancellationTokenSource cts,
            int drainWaitMs, int teardownMs,
            ProcessTreeResult result, ProbeOptions options, Action<string> addError)
        {
            bool bothCompleted = Task.WaitAll(new Task[] { tOut, tErr }, drainWaitMs);
            if (bothCompleted)
            {
                result.DrainsTerminal = true;
                result.Stdout = ReadDrainValue(tOut, result, true);
                result.Stderr = ReadDrainValue(tErr, result, false);
                return true;
            }

            addError("(8) stdout drain did not complete within " + drainWaitMs + "ms");
            addError("(9) stderr drain did not complete within " + drainWaitMs + "ms");
            cts.Cancel();
            CancelIoEx(fsOut.SafeFileHandle.DangerousGetHandle(), IntPtr.Zero);
            CancelIoEx(fsErr.SafeFileHandle.DangerousGetHandle(), IntPtr.Zero);
            srOut.Dispose();
            srErr.Dispose();
            fsOut.Dispose();
            fsErr.Dispose();

            if (options == ProbeOptions.DrainQuarantine)
            {
                var synthetic = new TaskCompletionSource<string>();
                result.Quarantined = true;
                result.RootOutcome = "Quarantined";
                result.DrainsTerminal = false;
                addError("(10) quarantined drain retained");
                QuarantinedDrains.Add(new KeyValuePair<Task, CancellationTokenSource>(synthetic.Task, cts));
                return false;
            }

            bool secondOk = Task.WaitAll(new Task[] { tOut, tErr }, teardownMs);
            result.DrainsTerminal = secondOk;
            result.Stdout = ReadDrainValue(tOut, result, true);
            result.Stderr = ReadDrainValue(tErr, result, false);
            if (!secondOk)
            {
                addError("(10) drain quarantine: drain task still incomplete after cancellation");
                result.Quarantined = true;
                result.RootOutcome = "Quarantined";
                QuarantinedDrains.Add(new KeyValuePair<Task, CancellationTokenSource>(tOut, cts));
                return false;
            }
            return true;
        }

        private static string ReadDrainValue(Task<string> task, ProcessTreeResult result, bool isStdout)
        {
            if (task.IsCompletedSuccessfully)
            {
                if (isStdout) result.StdoutDrainState = "completed"; else result.StderrDrainState = "completed";
                return task.Result ?? "";
            }
            if (task.IsCanceled)
            {
                if (isStdout) result.StdoutDrainState = "canceled"; else result.StderrDrainState = "canceled";
                return "";
            }
            if (task.IsFaulted)
            {
                var ex = task.Exception;
                string text = ex != null && ex.InnerException != null ? ex.InnerException.Message : "drain fault";
                if (isStdout) result.StdoutDrainState = "faulted: " + text; else result.StderrDrainState = "faulted: " + text;
                return "";
            }
            if (isStdout) result.StdoutDrainState = "incomplete"; else result.StderrDrainState = "incomplete";
            return "";
        }

        private static async Task<string> ReadToEndAsync(StreamReader reader, CancellationToken token)
        {
            try
            {
                return await reader.ReadToEndAsync().ConfigureAwait(false);
            }
            catch (OperationCanceledException)
            {
                return null;
            }
            catch (IOException)
            {
                return null;
            }
            catch (ObjectDisposedException)
            {
                return null;
            }
        }
    }
}
'@

function Invoke-Runner {
    param(
        [string]$Executable,
        [string[]]$Arguments,
        [string]$WorkingDirectory,
        [System.Collections.Generic.Dictionary[string, string]]$EnvOverrides,
        [int]$TimeoutMs,
        [DemoRender.ProbeOptions]$Options = [DemoRender.ProbeOptions]::None
    )
    return [DemoRender.ProcessTreeRunner]::Run($Executable, $Arguments, $WorkingDirectory, $EnvOverrides, $TimeoutMs, $Options)
}

$ImageMathCSharp = @'
using System;
using System.Collections.Generic;

namespace DemoRender
{
    public sealed class MaskResult
    {
        public int RetainedPixels;
        public int BboxX0; public int BboxY0; public int BboxW; public int BboxH;
        public double CentroidX; public double CentroidY;
        public double TotalEnergy;
        public byte[] Mask;
        public float[] Energy;
        public int Width; public int Height;
    }

    public sealed class CropVerify
    {
        public double VisibleEnergyFraction;
        public double RegionEnergyFraction;
        public double CentroidX; public double CentroidY;
        public bool CentroidInsideRegion;
        public double VisibleEnergy;
        public double RegionEnergy;
    }

    public static class ImageMath
    {
        public static double MeanAbsError(byte[] a, byte[] b)
        {
            if (a.Length != b.Length) return double.MaxValue;
            long sum = 0;
            for (int i = 0; i < a.Length; i++)
            {
                int d = a[i] - b[i];
                if (d < 0) d = -d;
                sum += d;
            }
            return (double)sum / a.Length;
        }

        public static MaskResult ComputeMask(byte[] before, byte[] after, int width, int height,
            int bandX0, int bandX1, int bandY0, int bandY1, int minComponentSize, double threshold, int dilateH, int dilateV)
        {
            var result = new MaskResult { Width = width, Height = height };
            var diff = new float[width * height];
            var mask = new byte[width * height];
            for (int y = 0; y < height; y++)
            {
                for (int x = 0; x < width; x++)
                {
                    int idx = y * width + x;
                    int b = idx * 3;
                    int dr = before[b] - after[b]; if (dr < 0) dr = -dr;
                    int dg = before[b + 1] - after[b + 1]; if (dg < 0) dg = -dg;
                    int db2 = before[b + 2] - after[b + 2]; if (db2 < 0) db2 = -db2;
                    int m = dr > dg ? dr : dg;
                    if (db2 > m) m = db2;
                    diff[idx] = m;
                    if (m >= threshold && x >= bandX0 && x <= bandX1 && y >= bandY0 && y <= bandY1)
                    {
                        mask[idx] = 1;
                    }
                }
            }
            var comp = new int[width * height];
            var sizes = new List<int>();
            int compCount = 0;
            var stack = new Stack<int>();
            for (int y = 0; y < height; y++)
            {
                for (int x = 0; x < width; x++)
                {
                    int idx = y * width + x;
                    if (mask[idx] == 0 || comp[idx] != 0) continue;
                    compCount++;
                    int size = 0;
                    stack.Push(idx);
                    mask[idx] = 0;
                    while (stack.Count > 0)
                    {
                        int cur = stack.Pop();
                        int cx = cur % width;
                        int cy = cur / width;
                        comp[cur] = compCount;
                        size++;
                        for (int dy = -1; dy <= 1; dy++)
                        {
                            int ny = cy + dy;
                            if (ny < 0 || ny >= height) continue;
                            for (int dx = -1; dx <= 1; dx++)
                            {
                                int nx = cx + dx;
                                if (nx < 0 || nx >= width) continue;
                                int nidx = ny * width + nx;
                                if (mask[nidx] == 0 || comp[nidx] != 0) continue;
                                mask[nidx] = 0;
                                stack.Push(nidx);
                            }
                        }
                    }
                    sizes.Add(size);
                }
            }
            for (int idx = 0; idx < width * height; idx++)
            {
                if (comp[idx] != 0 && sizes[comp[idx] - 1] < minComponentSize)
                {
                    comp[idx] = 0;
                }
            }
            var dilated = new byte[width * height];
            for (int y = 0; y < height; y++)
            {
                for (int x = 0; x < width; x++)
                {
                    int idx = y * width + x;
                    if (comp[idx] == 0) continue;
                    int x0 = x - dilateH; if (x0 < 0) x0 = 0;
                    int x1 = x + dilateH; if (x1 >= width) x1 = width - 1;
                    int y0 = y - dilateV; if (y0 < 0) y0 = 0;
                    int y1 = y + dilateV; if (y1 >= height) y1 = height - 1;
                    for (int yy = y0; yy <= y1; yy++)
                    {
                        for (int xx = x0; xx <= x1; xx++)
                        {
                            dilated[yy * width + xx] = 1;
                        }
                    }
                }
            }
            double ex = 0, ey = 0, energy = 0;
            int minX = width, minY = height, maxX = -1, maxY = -1;
            int retained = 0;
            for (int y = 0; y < height; y++)
            {
                for (int x = 0; x < width; x++)
                {
                    int idx = y * width + x;
                    if (comp[idx] == 0) continue;
                    double w = diff[idx];
                    ex += w * x;
                    ey += w * y;
                    energy += w;
                }
                for (int x = 0; x < width; x++)
                {
                    int idx = y * width + x;
                    if (dilated[idx] == 0) continue;
                    retained++;
                    if (x < minX) minX = x;
                    if (x > maxX) maxX = x;
                    if (y < minY) minY = y;
                    if (y > maxY) maxY = y;
                }
            }
            result.RetainedPixels = retained;
            result.BboxX0 = minX; result.BboxY0 = minY;
            result.BboxW = maxX - minX + 1; result.BboxH = maxY - minY + 1;
            result.CentroidX = energy > 0 ? ex / energy : 0;
            result.CentroidY = energy > 0 ? ey / energy : 0;
            result.TotalEnergy = energy;
            result.Mask = dilated;
            result.Energy = diff;
            return result;
        }

        public static CropVerify VerifyThroughCrop(MaskResult mask, int cropX, int cropY, double zEffW, double zEffH,
            int regionX0, int regionX1, int regionY0, int regionY1)
        {
            var v = new CropVerify();
            double visible = 0, region = 0, ex = 0, ey = 0;
            int width = mask.Width, height = mask.Height;
            for (int y = 0; y < height; y++)
            {
                for (int x = 0; x < width; x++)
                {
                    int idx = y * width + x;
                    if (mask.Mask[idx] == 0) continue;
                    double w = mask.Energy[idx];
                    if (w <= 0) continue;
                    double outX = (x - cropX) * zEffW;
                    double outY = (y - cropY) * zEffH;
                    if (outX < 0 || outX >= 960 || outY < 0 || outY >= 900) continue;
                    visible += w;
                    ex += w * outX;
                    ey += w * outY;
                    if (outX >= regionX0 && outX <= regionX1 && outY >= regionY0 && outY <= regionY1)
                    {
                        region += w;
                    }
                }
            }
            v.VisibleEnergy = visible;
            v.RegionEnergy = region;
            v.VisibleEnergyFraction = mask.TotalEnergy > 0 ? visible / mask.TotalEnergy : 0;
            v.RegionEnergyFraction = visible > 0 ? region / visible : 0;
            v.CentroidX = visible > 0 ? ex / visible : 0;
            v.CentroidY = visible > 0 ? ey / visible : 0;
            v.CentroidInsideRegion = v.CentroidX >= regionX0 && v.CentroidX <= regionX1 && v.CentroidY >= regionY0 && v.CentroidY <= regionY1;
            return v;
        }
    }
}
'@

function Add-ImageMath {
    try {
        Add-Type -TypeDefinition $ImageMathCSharp -Language CSharp
    }
    catch {
        throw "ImageMath compile failed: $($_.Exception.Message)"
    }
}

function Read-RawRgb {
    param([string]$Path, [int]$Width, [int]$Height)
    $bytes = [System.IO.File]::ReadAllBytes($Path)
    if ($bytes.Length -ne ($Width * $Height * 3)) {
        throw "raw rgb size mismatch for $Path : $($bytes.Length) vs $($Width * $Height * 3)"
    }
    return $bytes
}

function To-TrimmedString {
    param([string]$Text)
    return $Text.Trim()
}

function Start-SeedForMode {
    if (-not $ProbeVhsTimeout -and -not $ProbeFailurePaths -and -not $ProbeDrainQuarantineChild -and -not $PreflightOnly) {
        Invoke-Seed
    }
}

function Write-ProbeTape {
    param([string]$Path, [string]$Body)
    Set-Content -LiteralPath $Path -Value $Body -NoNewline -Encoding utf8
    Register-Path $Path
}

function New-ProbeShellReferenceTape([string]$tapePath, [string]$outPath) {
    $outLeaf = Split-Path -Leaf $outPath
    Write-ProbeTape $tapePath @"
Output probe/$outLeaf
Set Shell pwsh
Set Width 960
Set Height 900
Set FontSize 24
Set Padding 0
Set WindowBar Rings
Set Margin 20
Set MarginFill "#1e1e2e"
Set BorderRadius 8
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms
Set CursorBlink false
Sleep 1s
Screenshot probe/probe-shell.png
Type `cd "`$env:DEVLOG_DEMO_ROOT"`
Enter
Type "devlog"
Enter
Wait+Screen /Fix the cache memory leak/
Sleep 500ms
Screenshot probe/probe-tui.png
Sleep 500ms
"@
}

function New-TimeoutTape([string]$tapePath, [string]$outPath) {
    $outLeaf = Split-Path -Leaf $outPath
    Write-ProbeTape $tapePath @"
Output probe/$outLeaf
Set Shell pwsh
Set Width 960
Set Height 900
Hide
Type "Start-Sleep -Seconds 300"
Enter
Sleep 1s
"@
}

function New-FlatProbeTape([string]$tapePath, [string]$outPath) {
    $outLeaf = Split-Path -Leaf $outPath
    Write-ProbeTape $tapePath @"
Output docs/assets/$outLeaf
Set Shell pwsh
Set Width 960
Set Height 900
Type "Write-Output 'flat-probe'"
Enter
Sleep 1s
Screenshot docs/assets/.demo-palette-before.png
"@
}

function New-ProbeShellOnlyTape([string]$tapePath, [string]$outPath) {
    $outLeaf = Split-Path -Leaf $outPath
    Write-ProbeTape $tapePath @"
Output probe/$outLeaf
Set Shell pwsh
Set Width 960
Set Height 900
Set FontSize 24
Set Padding 0
Set WindowBar Rings
Set Margin 20
Set MarginFill "#1e1e2e"
Set BorderRadius 8
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms
Set CursorBlink false
Sleep 1s
Screenshot probe/probe-shell.png
Sleep 500ms
"@
}

function Invoke-PreflightProbe {
    $overrides = New-EnvOverrides

    $shellTape = Join-Path $ProbeDir "shell-ref.tape"
    $shellFlat = Join-Path $ProbeDir "shell-ref-flat.mp4"
    New-ProbeShellOnlyTape $shellTape $shellFlat
    $shellResult = Invoke-Runner -Executable $vhsPath -Arguments @($shellTape) -WorkingDirectory $RunRoot -EnvOverrides $overrides -TimeoutMs 60000
    if ($shellResult.RootOutcome -ne "Exited" -or $shellResult.ExitCode -ne 0) {
        throw "shell reference probe failed: $($shellResult.RootOutcome) exit=$($shellResult.ExitCode) errors=$($shellResult.Errors -join '; ') stdout=$($shellResult.Stdout) stderr=$($shellResult.Stderr)"
    }

    $tape = Join-Path $ProbeDir "preflight.tape"
    $flat = Join-Path $ProbeDir "preflight-flat.mp4"
    New-ProbeShellReferenceTape $tape $flat
    $result = Invoke-Runner -Executable $vhsPath -Arguments @($tape) -WorkingDirectory $RunRoot -EnvOverrides $overrides -TimeoutMs 180000
    $overrides = $null
    if ($result.Quarantined) {
        throw "preflight probe quarantined root pid $($result.RootPid): $($result.Errors -join '; ')"
    }
    if ($result.RootOutcome -ne "Exited" -or $result.ExitCode -ne 0) {
        throw "preflight probe run failed: $($result.RootOutcome) exit=$($result.ExitCode) errors=$($result.Errors -join '; ') stdout=$($result.Stdout) stderr=$($result.Stderr)"
    }
    if (-not $result.DrainsTerminal) {
        throw "preflight probe drains not terminal"
    }
    if (-not (Test-Path -LiteralPath $flat)) {
        throw "preflight probe produced no flat"
    }
    Add-ImageMath
    $lastRaw = Join-Path $ProbeDir "preflight-last.raw"
    $shellRaw = Join-Path $ProbeDir "probe-shell.raw"
    $tuiRaw = Join-Path $ProbeDir "probe-tui.raw"
    & $ffmpegPath -y -v error -i $flat -vf "reverse" -frames:v 1 -f rawvideo -pix_fmt rgb24 $lastRaw
    if ($LASTEXITCODE -ne 0 -or (Get-Item -LiteralPath $lastRaw).Length -eq 0) { throw "last frame extraction failed" }
    foreach ($png in @(@{ Png = (Join-Path $ProbeDir "probe-shell.png"); Raw = $shellRaw }, @{ Png = (Join-Path $ProbeDir "probe-tui.png"); Raw = $tuiRaw })) {
        if (-not (Test-Path -LiteralPath $png.Png)) { throw "probe reference missing: $($png.Png)" }
        & $ffmpegPath -y -v error -i $png.Png -vf "scale=960:900" -frames:v 1 -f rawvideo -pix_fmt rgb24 $png.Raw
        if ($LASTEXITCODE -ne 0) { throw "probe reference decode failed: $($png.Png)" }
    }
    $aLast = Read-RawRgb $lastRaw 960 900
    $b = Read-RawRgb $tuiRaw 960 900
    $c = Read-RawRgb $shellRaw 960 900
    $settledErr = [DemoRender.ImageMath]::MeanAbsError($aLast, $b)
    $shellErr = [DemoRender.ImageMath]::MeanAbsError($aLast, $c)
    if ($settledErr -gt 3.0) {
        throw "settled frame does not match TUI reference (error $settledErr > 3.0)"
    }
    if ($shellErr -lt 5.0) {
        throw "settled frame is not distinct from the hidden shell (error $shellErr < 5.0)"
    }
    Write-Step "first-frame probe: pass (settled tui error $([math]::Round($settledErr,2)), settled shell error $([math]::Round($shellErr,2)))"
}

function Invoke-TimeoutProbe {
    $tape = Join-Path $ProbeDir "timeout.tape"
    $flat = Join-Path $ProbeDir "timeout-flat.mp4"
    New-TimeoutTape $tape $flat
    $overrides = New-EnvOverrides
    $result = Invoke-Runner -Executable $vhsPath -Arguments @($tape) -WorkingDirectory $RunRoot -EnvOverrides $overrides -TimeoutMs 1000
    $overrides = $null
    $match = $result.RootOutcome -eq "TimedOut" -and
        -not $result.Quarantined -and
        $result.FinalActiveProcesses -eq 0 -and
        $result.DrainsTerminal -and
        $result.Errors.Count -eq 3 -and
        $result.Errors[0] -eq "(2) root wait timed out after 1000ms" -and
        $result.Errors[1] -like "(4) job still has * active processes after root outcome" -and
        $result.Errors[2] -eq "(5) termination request issued"
    if (-not $match) {
        throw "timeout probe unexpected result: outcome=$($result.RootOutcome) active=$($result.FinalActiveProcesses) quarantine=$($result.Quarantined) drains=$($result.DrainsTerminal) errors=$($result.Errors -join '; ')"
    }
    if (Test-Path -LiteralPath $flat) {
        Register-Path $flat
        Remove-Recorded
    }
    Write-Step "timeout probe: pass"
}

function Invoke-ProbeVhsTimeout {
    Invoke-TimeoutProbe
}

function Invoke-AssignmentProbe {
    $overrides = New-EnvOverrides
    $child = "$pwshPath -NoProfile -Command Write-Output assignment-probe"
    $result = Invoke-Runner -Executable $pwshPath -Arguments @("-NoProfile", "-Command", "Write-Output 'assignment-probe'") -WorkingDirectory $RepoRoot -EnvOverrides $overrides -TimeoutMs 30000 -Options ([DemoRender.ProbeOptions]::AssignmentFailure)
    $overrides = $null
    $expected = @(
        "(1) synthetic assignment failure"
    )
    $match = $result.RootOutcome -eq "AssignmentFailed" -and
        -not $result.Quarantined -and
        $result.FinalActiveProcesses -eq 0 -and
        $result.Errors.Count -eq $expected.Count
    if ($match) {
        for ($i = 0; $i -lt $expected.Count; $i++) {
            if ($result.Errors[$i] -ne $expected[$i]) { $match = $false; break }
        }
    }
    if (-not $match) {
        throw "assignment probe unexpected: outcome=$($result.RootOutcome) quarantine=$($result.Quarantined) active=$($result.FinalActiveProcesses) errors=$($result.Errors -join '; ')"
    }
    Write-Step "assignment termination: pass"
}

function Invoke-DrainCancellationProbe {
    $overrides = New-EnvOverrides
    $result = Invoke-Runner -Executable $pwshPath -Arguments @("-NoProfile", "-Command", "Write-Output 'probe-output'; Start-Sleep -Seconds 2") -WorkingDirectory $RepoRoot -EnvOverrides $overrides -TimeoutMs 30000 -Options ([DemoRender.ProbeOptions]::DrainCancellation)
    $overrides = $null
    $disposalError = $false
    foreach ($e in $result.Errors) {
        if ($e -match "^\(10\)") { $disposalError = $true }
    }
    $match = -not $result.Quarantined -and
        $result.DrainsTerminal -and
        $result.FinalActiveProcesses -eq 0 -and
        -not $disposalError
    if (-not $match) {
        throw "drain cancellation unexpected: quarantine=$($result.Quarantined) drains=$($result.DrainsTerminal) active=$($result.FinalActiveProcesses) stdout='$($result.Stdout)' errors=$($result.Errors -join '; ')"
    }
    Write-Step "drain cancellation: pass"
}

function Invoke-CompetingErrorProbe {
    $overrides = New-EnvOverrides
    $result = Invoke-Runner -Executable $pwshPath -Arguments @("-NoProfile", "-Command", "exit 3") -WorkingDirectory $RepoRoot -EnvOverrides $overrides -TimeoutMs 30000
    $overrides = $null
    if ($result.RootOutcome -ne "Exited" -or $result.ExitCode -ne 3 -or $result.Quarantined) {
        throw "competing probe lifecycle result unexpected: outcome=$($result.RootOutcome) exit=$($result.ExitCode)"
    }
    $primary = $null
    foreach ($e in $result.Errors) {
        if ($e -match "^\(3\) unexpected non-zero exit code 3$") { $primary = $e }
    }
    if (-not $primary) {
        throw "competing probe missing primary lifecycle error: $($result.Errors -join '; ')"
    }
    $lockPath = Join-Path $RunRoot "competing-lock.txt"
    Set-Content -LiteralPath $lockPath -Value "lock" -NoNewline -Encoding utf8
    $stream = [System.IO.File]::Open($lockPath, [System.IO.FileMode]::Open, [System.IO.FileAccess]::ReadWrite, [System.IO.FileShare]::None)
    try {
        try {
            Remove-Item -LiteralPath $lockPath -Force -ErrorAction Stop
        }
        catch {
            $script:CleanupErrors.Add("cleanup failed for $lockPath : $($_.Exception.Message)")
        }
    }
    finally {
        $stream.Dispose()
    }
    if (-not (Test-Path -LiteralPath $lockPath)) {
        throw "competing probe lock removal unexpectedly succeeded while locked"
    }
    $secondary = $script:CleanupErrors[-1]
    if ($secondary -notmatch "^cleanup failed for .*competing-lock") {
        throw "competing probe missing secondary cleanup error"
    }
    Remove-Item -LiteralPath $lockPath -Force -ErrorAction Stop
    $script:CleanupErrors.Clear()
    Write-Step "competing errors: pass (primary `"$primary`" then `"$secondary`")"
}

function Invoke-FlatRetentionProbe {
    $tape = Join-Path $ProbeDir "flat-probe.tape"
    $flat = Join-Path $AssetsDir $FlatRel
    $ref = Join-Path $AssetsDir ".demo-palette-before.png"
    New-FlatProbeTape $tape $flat
    $overrides = New-EnvOverrides
    $result = Invoke-Runner -Executable $vhsPath -Arguments @($tape) -WorkingDirectory $RepoRoot -EnvOverrides $overrides -TimeoutMs 60000
    $overrides = $null
    if ($result.RootOutcome -ne "Exited" -or $result.ExitCode -ne 0) {
        throw "flat probe run failed: $($result.RootOutcome) exit=$($result.ExitCode) errors=$($result.Errors -join '; ')"
    }
    if (-not (Test-Path -LiteralPath $flat)) {
        throw "flat probe produced no flat"
    }
    if (-not (Test-Path -LiteralPath $ref)) {
        throw "flat probe produced no reference"
    }
    Register-Path $flat
    Register-Path $ref
    try {
        throw "synthetic post-flat failure"
    }
    catch {
        $script:CleanupErrors.Add("synthetic post-flat failure: $($_.Exception.Message)")
    }
    Remove-Recorded -KeepFlat
    if (-not (Test-Path -LiteralPath $flat)) {
        throw "flat was not retained on failure"
    }
    if (Test-Path -LiteralPath $ref) {
        throw "reference was not removed on failure"
    }
    if (Test-Path -LiteralPath $RunRoot) {
        throw "run root was not removed on failure"
    }
    $script:CleanupErrors.Clear()
    Remove-Item -LiteralPath $flat -Force -ErrorAction Stop
    Write-Step "flat retention: pass"
}

function Invoke-DrainQuarantineChild {
    $overrides = New-EnvOverrides
    $result = Invoke-Runner -Executable $pwshPath -Arguments @("-NoProfile", "-Command", "Write-Output 'quarantine-child'; Start-Sleep -Seconds 2") -WorkingDirectory $RepoRoot -EnvOverrides $overrides -TimeoutMs 30000 -Options ([DemoRender.ProbeOptions]::DrainQuarantine)
    $overrides = $null
    if (-not $result.Quarantined -or -not ($result.Errors -join '; ') -match "quarantined drain retained") {
        throw "drain quarantine child unexpected: quarantine=$($result.Quarantined) errors=$($result.Errors -join '; ')"
    }
    Write-Step "drain quarantine: pass"
    Remove-Recorded
    exit 0
}

function Invoke-ProbeFailurePaths {
    Add-ImageMath
    Invoke-AssignmentProbe
    Invoke-DrainCancellationProbe
    Invoke-CompetingErrorProbe
    Invoke-FlatRetentionProbe
    $childScript = Join-Path $AssetsDir "render-demo.ps1"
    $overrides = New-EnvOverrides
    $result = Invoke-Runner -Executable $pwshPath -Arguments @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $childScript, "-ProbeDrainQuarantineChild") -WorkingDirectory $RepoRoot -EnvOverrides $overrides -TimeoutMs 30000
    $overrides = $null
    $match = $result.RootOutcome -eq "Exited" -and
        $result.ExitCode -eq 0 -and
        $result.FinalActiveProcesses -eq 0 -and
        $result.DrainsTerminal -and
        -not $result.Quarantined -and
        $result.Stdout -match "drain quarantine: pass"
    if (-not $match) {
        throw "drain quarantine parent unexpected: outcome=$($result.RootOutcome) exit=$($result.ExitCode) active=$($result.FinalActiveProcesses) drains=$($result.DrainsTerminal) stdout='$($result.Stdout)' stderr='$($result.Stderr)' errors=$($result.Errors -join '; ')"
    }
    Write-Step "drain quarantine: pass"
}

function Invoke-Render {
    param([string]$Flat)
    $gif = Join-Path $AssetsDir "demo.gif"
    $filter = "[0:v]fps=50,scale=960:900,split[z1][z2];[z1]palettegen=stats_mode=diff[p];[z2][p]paletteuse=dither=bayer:bayer_scale=5"
    $renderErr = & $ffmpegPath -y -v error -i $Flat -filter_complex $filter -loop 0 $gif 2>&1
    if ($LASTEXITCODE -ne 0) { throw "gif render failed: $renderErr" }
    Register-Path $gif
}

function Get-Signature {
    param([string]$Path, [int]$Count)
    $bytes = [System.IO.File]::ReadAllBytes($Path)
    $sig = ""
    for ($i = 0; $i -lt $Count; $i++) {
        $sig += $bytes[$i].ToString("X2")
    }
    return $sig
}

function Invoke-MediaChecks {
    param([string]$Gif, [string]$Png)
    $gifSig = Get-Signature $Gif 6
    if ($gifSig -ne "474946383761" -and $gifSig -ne "474946383961") {
        throw "demo.gif has invalid signature $gifSig"
    }
    $pngSig = Get-Signature $Png 8
    if ($pngSig -ne "89504E470D0A1A0A") {
        throw "tui-screenshot.png has invalid signature $pngSig"
    }
    $gifLen = (Get-Item -LiteralPath $Gif).Length
    $pngLen = (Get-Item -LiteralPath $Png).Length
    if ($gifLen -lt 10000 -or $gifLen -gt 50000000) { throw "demo.gif size out of bounds: $gifLen" }
    if ($pngLen -lt 10000 -or $pngLen -gt 2000000) { throw "tui-screenshot.png size out of bounds: $pngLen" }
    $stream = & $ffprobePath -v error -select_streams v:0 -show_entries stream=r_frame_rate,width,height -show_entries format=duration -of default=nw=1 $Gif
    $rate = $null; $width = $null; $height = $null; $duration = $null
    foreach ($line in $stream) {
        if ($line -match "^r_frame_rate=(.+)$") { $rate = $Matches[1] }
        elseif ($line -match "^width=(\d+)$") { $width = [int]$Matches[1] }
        elseif ($line -match "^height=(\d+)$") { $height = [int]$Matches[1] }
        elseif ($line -match "^duration=([\d\.]+)$") { $duration = [double]$Matches[1] }
    }
    $frameProbe = & $ffprobePath -v error -select_streams v:0 -count_frames -show_entries stream=nb_read_frames -of csv=p=0 $Gif
    $frameCount = [int]($frameProbe.Trim())
    $expectedFrames = [int][math]::Round($duration * 50)
    if ([math]::Abs($frameCount - $expectedFrames) -gt [math]::Max(2, [int]($expectedFrames * 0.01))) {
        throw "demo.gif has $frameCount frames, want ~$expectedFrames for 50fps over $([math]::Round($duration,2))s"
    }
    if ($width -ne 960 -or $height -ne 900) { throw "demo.gif is ${width}x${height}, want 960x900" }
    if ($duration -lt 25 -or $duration -gt 90) { throw "demo.gif duration $duration not in 25-90s" }
    $tuiItem = Get-Item -LiteralPath $Png
    if (-not $script:RunStarted -or $tuiItem.LastWriteTime -lt $script:RunStarted) {
        throw "tui-screenshot.png is stale: written $($tuiItem.LastWriteTime), run started $($script:RunStarted)"
    }
    Write-Step "media checks: pass (${width}x${height}, 50 fps, $([math]::Round($duration,1))s, gif $gifLen bytes, png $pngLen bytes)"
}

function Get-PostStateChecks {
    $sessionFile = Join-Path $DemoRoot ".devlog\sessions\fix-cache-leak.md"
    if (-not (Test-Path -LiteralPath $sessionFile)) {
        throw "active session file missing after render"
    }
    $content = Get-Content -Raw -LiteralPath $sessionFile
    $c = Count-SessionEvents $sessionFile
    $visible = $c.Notes + $c.Blockers
    if ($visible -ne 18) {
        throw "post-render active events = $visible, want 18"
    }
    if ($content -notmatch "Eviction drops the oldest key when the cache is full\.") {
        throw "live note missing after render"
    }
    if ($content -notmatch "Decide whether to backfill eviction on existing instances or just restart") {
        throw "CLI blocker missing after render"
    }
    $todoContent = Get-Content -Raw -LiteralPath (Join-Path $DemoRoot ".devlog\todos.md")
    $open = ([regex]::Matches($todoContent, "(?m)^  status: open$")).Count
    $done = ([regex]::Matches($todoContent, "(?m)^  status: done$")).Count
    if ($open -ne 3 -or $done -ne 4) {
        throw "post-render todos = $open open / $done done, want 3/4"
    }
    if ($todoContent -notmatch "Add cache TTL metrics") {
        throw "CLI-added todo missing after render"
    }
    Write-Step "post-state: pass (18 active events, one live note, one CLI blocker, 7 todos 3/4)"
}

function Invoke-LinkCheck {
    $errors = @()
    $docs = @(Join-Path $RepoRoot "README.md") + (Get-ChildItem -LiteralPath (Join-Path $RepoRoot "docs") -Filter "*.md" | ForEach-Object { $_.FullName })
    foreach ($doc in $docs) {
        $content = Get-Content -Raw -LiteralPath $doc
        $matches = [regex]::Matches($content, '(?m)\[([^\]]+)\]\(([^\)]+)\)')
        foreach ($m in $matches) {
            $target = $m.Groups[2].Value
            if ($target -match '^(https?://|mailto:|#|devlog://)') { continue }
            if ($target -match '\s') { continue }
            $clean = $target.Split('#')[0]
            if ($clean -eq "") { continue }
            $path = Join-Path (Split-Path -Parent $doc) ($clean -replace '/', '\')
            if ($clean -match 'banner\.png$') {
                continue
            }
            if (-not (Test-Path -LiteralPath $path)) {
                $errors += "missing link $target in $doc"
            }
        }
    }
    if ($errors.Count -gt 0) {
        throw "link check failed: $($errors -join '; ')"
    }
    Write-Step "link check: pass"
}

function Invoke-Production {
    $overrides = New-EnvOverrides
    $tapePath = Join-Path $RepoRoot "docs\assets\demo.tape"
    $result = $null
    for ($attempt = 1; $attempt -le 3; $attempt++) {
        $result = Invoke-Runner -Executable $vhsPath -Arguments @($tapePath) -WorkingDirectory $DemoRoot -EnvOverrides $overrides -TimeoutMs 1200000
        if ($result.RootOutcome -eq "Exited" -and $result.ExitCode -eq 0) { break }
        $browserFail = ($result.Stderr -match "could not launch browser" -or $result.Stdout -match "could not launch browser")
        if (-not $browserFail -or $attempt -eq 3) {
            $overrides = $null
            break
        }
        Write-Step "vhs browser launch failed (attempt $attempt), retrying"
        Start-Sleep -Seconds 2
    }
    $overrides = $null
    # VHS ran with CWD = $DemoRoot, so relative Output/Screenshot paths land there.
    # Copy every produced artifact into $AssetsDir so derivation and checks read them there.
    $flat = Join-Path $AssetsDir $FlatRel
    $srcFlat = Join-Path $DemoRoot $FlatRel
    if (Test-Path -LiteralPath $srcFlat) {
        Copy-Item -LiteralPath $srcFlat -Destination $flat -Force
        Register-Path $flat
    }
    foreach ($ref in $ReferenceNames) {
        $src = Join-Path $DemoRoot $ref
        $dst = Join-Path $AssetsDir $ref
        if (Test-Path -LiteralPath $src) {
            Copy-Item -LiteralPath $src -Destination $dst -Force
            Register-Path $dst
        }
    }
    $srcTui = Join-Path $DemoRoot "tui-screenshot.png"
    if (Test-Path -LiteralPath $srcTui) {
        Copy-Item -LiteralPath $srcTui -Destination (Join-Path $AssetsDir "tui-screenshot.png") -Force
    }
    if ($result.Quarantined) {
        Remove-Recorded -KeepFlat
        throw "production run quarantined root pid $($result.RootPid): $($result.Errors -join '; ')"
    }
    if ($result.RootOutcome -ne "Exited" -or $result.ExitCode -ne 0) {
        Remove-Recorded -KeepFlat
        throw "production run failed: outcome=$($result.RootOutcome) exit=$($result.ExitCode) errors=$($result.Errors -join '; ') stdout=$($result.Stdout) stderr=$($result.Stderr)"
    }
    if ($result.FinalActiveProcesses -ne 0 -or -not $result.DrainsTerminal) {
        Remove-Recorded -KeepFlat
        throw "production run left work behind: active=$($result.FinalActiveProcesses) drains=$($result.DrainsTerminal)"
    }
    if ($result.Errors.Count -gt 0) {
        Remove-Recorded -KeepFlat
        throw "production run lifecycle errors: $($result.Errors -join '; ')"
    }
    if (-not (Test-Path -LiteralPath $flat)) {
        Remove-Recorded -KeepFlat
        throw "production run produced no flat"
    }
    Write-Step "vhs run: pass (flat $flat)"
}

function Invoke-Main {
    if ($ProbeDrainQuarantineChild) {
        try {
            $script:Created.Clear()
            Add-Type -TypeDefinition $ProcessTreeCSharp -Language CSharp
            Invoke-DrainQuarantineChild
        }
        catch {
            Write-Error $_
            exit 1
        }
    }
    try {
        $script:RunStarted = Get-Date
        Remove-AbandonedProbe
        & $vhsPath validate (Join-Path $AssetsDir "demo.tape") | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "vhs validate failed" }
        Add-Type -TypeDefinition $ProcessTreeCSharp -Language CSharp
        Assert-NoOwnedArtifacts
        Start-SeedForMode
        if ($PreflightOnly) {
            Invoke-Seed
            Invoke-PreflightProbe
        }
        elseif ($ProbeVhsTimeout) {
            Invoke-TimeoutProbe
        }
        elseif ($ProbeFailurePaths) {
            Invoke-ProbeFailurePaths
        }
        else {
            Invoke-Production
            Invoke-Render (Join-Path $AssetsDir $FlatRel)
            $gif = Join-Path $AssetsDir "demo.gif"
            $png = Join-Path $AssetsDir "tui-screenshot.png"
            if (-not (Test-Path -LiteralPath $png)) {
                throw "production run produced no screenshot"
            }
            Register-Path $png
            Invoke-MediaChecks $gif $png
            Invoke-LinkCheck
            Get-PostStateChecks
            Remove-Recorded
            Write-Step "render complete"
        }
    }
    catch {
        $primary = $_.Exception.Message
        Remove-Recorded -KeepFlat
        $flatPath = Join-Path $AssetsDir $FlatRel
        if (Test-Path -LiteralPath $flatPath) {
            Write-Output "retained flat diagnostic: $flatPath"
        }
        if ($script:CleanupErrors.Count -gt 0) {
            Write-Error "primary failure: $primary"
            Write-Error "cleanup failure: $($script:CleanupErrors -join '; ')"
        }
        else {
            Write-Error $_
        }
        exit 1
    }
}

Invoke-Main
