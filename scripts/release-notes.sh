#!/usr/bin/env bash
set -euo pipefail

tag="${1:?usage: scripts/release-notes.sh v0.1.0}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

extract_section() {
  local file="$1"
  local heading="$2"

  awk -v heading="$heading" '
    /^## / {
      if (found) {
        exit
      }
      if ($0 ~ "^## " heading "([[:space:]]|-|$)") {
        found = 1
        next
      }
    }
    found {
      print
    }
  ' "$file"
}

english="$(extract_section "$root/CHANGELOG.md" "$tag")"
chinese="$(extract_section "$root/CHANGELOG.zh-CN.md" "$tag")"

if [ -z "$english" ]; then
  echo "CHANGELOG.md does not contain a section for $tag" >&2
  exit 1
fi

if [ -z "$chinese" ]; then
  echo "CHANGELOG.zh-CN.md does not contain a section for $tag" >&2
  exit 1
fi

cat <<EOF
# runa $tag

## English

$english

## 简体中文

$chinese
EOF
