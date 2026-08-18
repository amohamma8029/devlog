Output demo-flat.mp4
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
Set TypingSpeed 100ms
Set CursorBlink false
Set Framerate 60

Sleep 1s
Screenshot .demo-shell.png
Type "devlog"
Enter
Wait+Screen /Fix the cache memory leak/
Sleep 500ms
Screenshot .demo-first-tui.png
Sleep 2s

Type "/"
Sleep 2s
Wait+Screen /\/note/
Sleep 2s
Screenshot .demo-palette-before.png
Type "note"
Sleep 2s
Wait+Screen /Add a note/
Screenshot .demo-palette-after.png
Enter
Type "Eviction drops the oldest key when the cache is full."
Alt+Enter
Sleep 800ms
Type "Will tune the cap once prod numbers land."
Enter
Sleep 1s
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Down
Down
Down
Sleep 1s
Wait+Screen /Eviction drops the oldest key/
Sleep 2s

Type "/list"
Enter
Sleep 2s
Wait+Screen /TITLE/
Screenshot .demo-history-before.png
Type "/"
Sleep 1s
Type "leak"
Sleep 1s
Wait+Screen /2 matches/
Screenshot .demo-history-after.png
Sleep 2s
Enter
Sleep 2s
Down
Sleep 1s
Enter
Sleep 1s
Sleep 2s
Screenshot .demo-repro.png
Sleep 2s

Type "/list"
Enter
Sleep 2s
Up
Sleep 1s
Enter
Wait+Screen /Fix the cache memory leak/
Sleep 2s
Tab
Sleep 1s
Tab
Sleep 800ms
Tab
Sleep 800ms
Tab
Sleep 800ms
Tab
Sleep 800ms
Tab
Sleep 800ms
Tab
Sleep 800ms
Shift+Tab
Sleep 1s
Screenshot .demo-tab-select.png
Sleep 1s

Type "/todo"
Enter
Wait+Screen /Land the eviction change/
Sleep 2s
Down
Sleep 1s
Space
Sleep 2s
Escape
Sleep 1s

Type "/exit"
Enter
Sleep 2s
Type `devlog block "Decide whether to backfill eviction on existing instances or just restart"`
Enter
Wait+Screen /Logged blocker/
Sleep 1s
Type `devlog todo add "Add cache TTL metrics"`
Enter
Wait+Screen /Added todo/
Sleep 2s

Type "devlog"
Enter
Wait+Screen /Fix the cache memory leak/
Sleep 2s
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 800ms
Down
Down
Sleep 1s
Wait+Screen /backfill eviction/
Screenshot .demo-blocker-before.png
Shift+Tab
Sleep 1s
Screenshot .demo-blocker-after.png
Sleep 4s

Type "/handoff"
Enter
Wait+Screen /Handoff Preview/
Sleep 5s
Screenshot .demo-tui-warmup-1.png
Screenshot .demo-tui-warmup-2.png
Screenshot tui-screenshot.png
Type "]"
Sleep 1s
Type "]"
Sleep 1s
Wait+Screen /internal\/cache\/cache_test.go/
Sleep 1s
Down
Down
Down
Down
Down
Sleep 1s
Down
Down
Down
Down
Down
Sleep 2s
Screenshot .demo-handoff-after.png
Type "/"
Sleep 1s
Type "eviction"
Enter
Sleep 2s
Sleep 3s
