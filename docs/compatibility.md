# Compatibility

## Supported platforms

| Platform | Architectures | Install routes |
| --- | --- | --- |
| macOS | `amd64`, `arm64` | Homebrew, direct archive, `go install` |
| Linux | `amd64`, `arm64` | Homebrew, `.deb`/`.rpm`/`.apk` packages, direct archive, `go install` |
| Windows | `amd64`, `arm64` | PowerShell installer, Scoop, direct archive, `go install`; WinGet after the first public release |

## Requirements

- git is required; devlog records session context inside a git repository and reads author identity from git config.
- Windows installation via the PowerShell installer requires PowerShell 5.1 or newer.
- No account or external service is required to use devlog.

## Configuration

devlog reads an optional global config file at `~/.config/devlog/config.yml` on all platforms. See [configuration.md](./configuration.md) for the full schema.

## Existing data

Sessions stored under `.devlog/sessions/` are plain Markdown files with YAML front matter. Existing `.devlog` data needs no migration and keeps working across upgrades.

## Version output

`devlog --version` prints the release version without a leading `v` (for example `1.0.0`). Development builds print `dev`.

## Not supported

- npm/pnpm, MSI, MSIX, Snap, Flatpak, hosted APT/RPM repositories, AUR, and Nixpkgs are not provided.
- Chocolatey is not yet published; package source is prepared for a future moderated submission.
- WinGet is available after the first public release.
