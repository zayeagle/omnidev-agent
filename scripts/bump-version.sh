#!/usr/bin/env bash
# Bump semver patch in VERSION (0.0.0 -> 0.0.1). Usage: scripts/bump-version.sh [patch|minor|major]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FILE="$ROOT/VERSION"
PART="${1:-patch}"

if [[ ! -f "$FILE" ]]; then
  echo "0.0.0" > "$FILE"
fi

ver="$(tr -d ' \t\r\n' < "$FILE")"
ver="${ver#v}"
IFS=. read -r major minor patch _ <<< "$ver"
major=${major:-0}
minor=${minor:-0}
patch=${patch:-0}

case "$PART" in
  major)
    major=$((major + 1))
    minor=0
    patch=0
    ;;
  minor)
    minor=$((minor + 1))
    patch=0
    ;;
  patch|*)
    patch=$((patch + 1))
    ;;
esac

new="${major}.${minor}.${patch}"
printf '%s\n' "$new" > "$FILE"
echo "v$new"
