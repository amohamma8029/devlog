$ErrorActionPreference = 'Stop'

$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Uninstall-BinFile -Name 'devlog' -Path (Join-Path $toolsDir 'devlog.exe')
