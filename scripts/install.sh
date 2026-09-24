#!/bin/sh
# Install the latest capybari release.
#   curl -fsSL https://raw.githubusercontent.com/capybari-repo/capybari-cli/main/scripts/install.sh | sh
# Options (environment): CAPYBARI_VERSION=v0.1.0  CAPYBARI_INSTALL_DIR=/usr/local/bin
set -eu
repo="capybari-repo/capybari-cli"
version="${CAPYBARI_VERSION:-}"
dir="${CAPYBARI_INSTALL_DIR:-/usr/local/bin}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in linux|darwin) ;; *) echo "unsupported OS: $os (use the Windows zip from the releases page)" >&2; exit 1 ;; esac
arch=$(uname -m)
case "$arch" in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) echo "unsupported architecture: $arch" >&2; exit 1 ;; esac

if [ -z "$version" ]; then
  version=$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)
fi
[ -n "$version" ] || { echo "could not determine the latest version" >&2; exit 1; }
num=${version#v}
archive="capybari_${num}_${os}_${arch}.tar.gz"
base="https://github.com/$repo/releases/download/$version"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "Downloading capybari $version for $os/$arch"
curl -fsSL -o "$tmp/$archive" "$base/$archive"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt"
( cd "$tmp" && grep " $archive\$" checksums.txt | sha256sum -c - >/dev/null 2>&1 || grep " $archive\$" checksums.txt | shasum -a 256 -c - >/dev/null ) \
  || { echo "checksum verification failed" >&2; exit 1; }
tar -xzf "$tmp/$archive" -C "$tmp" capybari
if [ -w "$dir" ]; then mv "$tmp/capybari" "$dir/capybari"; else sudo mv "$tmp/capybari" "$dir/capybari"; fi
echo "Installed $("$dir/capybari" version)"
echo "Try: capybari analyze ."
