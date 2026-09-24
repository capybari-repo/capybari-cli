#!/usr/bin/env bash
# Replace pre-release `replace` directives with tagged module versions.
#   scripts/release-deps.sh v0.1.0
# Run in each repository in release order:
#   capybari-schemas -> capybari-core -> capybari-analyzer-* -> capybari-cli
set -euo pipefail
version="${1:?usage: scripts/release-deps.sh <version>}"
mods=$(go mod edit -json | python3 -c 'import json,sys; print("\n".join(r["Old"]["Path"] for r in json.load(sys.stdin).get("Replace") or [] if r["Old"]["Path"].startswith("github.com/capybari-repo/")))')
for m in $mods; do
  go mod edit -dropreplace="$m" -require="$m@$version"
done
GOFLAGS=-mod=mod go mod tidy
echo "Pinned: $mods"
