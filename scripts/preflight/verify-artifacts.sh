#!/usr/bin/env bash
# Test-only fixture: asserts the release artifact inventory — 6 OS/arch
# archives, 6 Linux native packages, and the checksums file — all carrying a
# single consistent version, and cross-verifies each artifact SHA-256 against
# the checksums file. Used by .github/workflows/preflight.yml
# (artifact-inventory job). Not shipped; not part of the devlog binary.
set -euo pipefail

dist="${1:?usage: verify-artifacts.sh <dist-dir> [expected-version]}"
expected_version="${2:-}"

# Canonicalize to a POSIX absolute path so sha256sum and globs see clean
# paths even when invoked from MSYS/git-bash with a Windows-style path.
dist="$(cd "$dist" && pwd)"

shopt -s nullglob

suffixes=(
    windows_amd64.zip
    windows_arm64.zip
    darwin_amd64.tar.gz
    darwin_arm64.tar.gz
    linux_amd64.tar.gz
    linux_arm64.tar.gz
    linux_amd64.deb
    linux_arm64.deb
    linux_amd64.rpm
    linux_arm64.rpm
    linux_amd64.apk
    linux_arm64.apk
)

declare -a found_files=()
declare -a versions=()
fail=0

for suffix in "${suffixes[@]}"; do
    matches=("$dist"/devlog_*_"$suffix")
    if [[ ${#matches[@]} -eq 0 ]]; then
        echo "FAIL: missing artifact devlog_<version>_$suffix"
        fail=1
        continue
    fi
    if [[ ${#matches[@]} -gt 1 ]]; then
        echo "FAIL: multiple artifacts match devlog_*_$suffix:"
        for m in "${matches[@]}"; do echo "  $(basename "$m")"; done
        fail=1
        continue
    fi
    name=$(basename "${matches[0]}")
    if [[ ! -f "$dist/$name" ]]; then
        echo "FAIL: artifact is not a regular file: $name"
        fail=1
        continue
    fi
    version=${name#devlog_}
    version=${version%_$suffix}
    versions+=("$version")
    found_files+=("$name")
done

checksum_matches=("$dist"/devlog_*_checksums.txt)
if [[ ${#checksum_matches[@]} -eq 0 ]]; then
    echo "FAIL: missing artifact devlog_<version>_checksums.txt"
    fail=1
elif [[ ${#checksum_matches[@]} -gt 1 ]]; then
    echo "FAIL: multiple checksum files:"
    for m in "${checksum_matches[@]}"; do echo "  $(basename "$m")"; done
    fail=1
else
    name=$(basename "${checksum_matches[0]}")
    version=${name#devlog_}
    version=${version%_checksums.txt}
    versions+=("$version")
    found_files+=("$name")
fi

if [[ $fail -eq 0 ]]; then
    first=${versions[0]}
    if [[ ! "$first" =~ ^[0-9A-Za-z][0-9A-Za-z.+-]*$ ]]; then
        echo "FAIL: suspicious version string '$first'"
        fail=1
    fi
    for v in "${versions[@]}"; do
        if [[ "$v" != "$first" ]]; then
            echo "FAIL: inconsistent versions across artifacts: $first vs $v"
            fail=1
            break
        fi
    done
    if [[ -n "$expected_version" && "$first" != "$expected_version" ]]; then
        echo "FAIL: expected version $expected_version, got $first"
        fail=1
    fi
fi

for f in "$dist"/devlog_*; do
    [[ -f "$f" ]] || continue
    name=$(basename "$f")
    seen=0
    for g in "${found_files[@]}"; do [[ "$g" == "$name" ]] && seen=1; done
    if [[ $seen -eq 0 ]]; then
        echo "FAIL: unexpected artifact $name"
        fail=1
    fi
done

if [[ $fail -eq 0 ]]; then
    checksums=$(basename "${checksum_matches[0]}")
    missing_hash=0
    for f in "${found_files[@]}"; do
        if [[ "$f" == "$checksums" ]]; then continue; fi
        hash=$(sha256sum "$dist/$f" | awk '{print $1}')
        if ! grep -qE "^${hash}(  | \*)${f}\r?$" "$dist/$checksums"; then
            echo "FAIL: checksums file has no entry for $f"
            missing_hash=1
        fi
    done
    if [[ $missing_hash -eq 1 ]]; then fail=1; fi
fi

if [[ $fail -eq 0 ]]; then
    stale_entry=0
    while IFS= read -r line; do
        [[ -z "$line" ]] && continue
        name=$(awk '{print $2}' <<<"$line")
        name=${name#\*}
        name=${name%$'\r'}
        seen=0
        for g in "${found_files[@]}"; do [[ "$g" == "$name" ]] && seen=1; done
        if [[ $seen -eq 0 ]]; then
            echo "FAIL: checksums file lists unknown artifact: $line"
            stale_entry=1
        fi
    done < "$dist/$checksums"
    if [[ $stale_entry -eq 1 ]]; then fail=1; fi
fi

if [[ $fail -eq 0 ]]; then
    echo "PASS: ${#found_files[@]} artifacts present, version $first"
    for f in "${found_files[@]}"; do echo "  $f"; done
    exit 0
fi
exit 1