$ErrorActionPreference = 'Stop'

$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$version = '1.0.0'

$url64 = "https://github.com/amohamma8029/devlog/releases/download/v$version/devlog_${version}_windows_amd64.zip"
$checksum64 = 'REPLACE_WITH_PUBLIC_RELEASE_SHA256_AMD64'

$url64arm64 = "https://github.com/amohamma8029/devlog/releases/download/v$version/devlog_${version}_windows_arm64.zip"
$checksum64arm64 = 'REPLACE_WITH_PUBLIC_RELEASE_SHA256_ARM64'

if ($checksum64 -like 'REPLACE_*' -or $checksum64arm64 -like 'REPLACE_*') {
    throw "devlog package: release SHA-256 placeholders must be replaced with the public v$version checksums before submission"
}

$isArm64 = $env:PROCESSOR_ARCHITECTURE -eq 'ARM64' -or $env:PROCESSOR_ARCHITEW6432 -eq 'ARM64'
if ($isArm64) {
    $url = $url64arm64
    $checksum = $checksum64arm64
}
elseif ($env:PROCESSOR_ARCHITEW6432 -eq 'AMD64' -or $env:PROCESSOR_ARCHITECTURE -eq 'AMD64') {
    $url = $url64
    $checksum = $checksum64
}
else {
    throw "devlog package: unsupported architecture; only amd64 and arm64 are supported"
}

$packageArgs = @{
    packageName  = $env:ChocolateyPackageName
    url          = $url
    checksum     = $checksum
    checksumType = 'sha256'
    fileFullPath = Join-Path $toolsDir 'devlog.zip'
}
Get-ChocolateyWebFile @packageArgs
Get-ChocolateyUnzip -FileFullPath (Join-Path $toolsDir 'devlog.zip') -Destination $toolsDir
Remove-Item -LiteralPath (Join-Path $toolsDir 'devlog.zip') -Force
