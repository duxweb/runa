#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<USAGE
Usage: scripts/release.sh <version> [--dry-run] [--push]

Prepares and tags a multi-module Runa release. Dry-run is the default.
Run only after the release tree is final and oro is already published.
USAGE
}

if [[ $# -lt 1 ]]; then
  usage
  exit 2
fi

version="$1"
shift
push=0
dry_run=1
for arg in "$@"; do
  case "$arg" in
    --dry-run) dry_run=1 ;;
    --push) push=1; dry_run=0 ;;
    *) usage; exit 2 ;;
  esac
done

if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
  echo "version must look like v0.1.0" >&2
  exit 2
fi

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

oro_version=$(rg -o 'github\.com/duxweb/oro\s+v[0-9]+\.[0-9]+\.[0-9]+' --glob 'go.mod' --glob '!docs/**' . | awk '{print $2}' | sort -u)
if [[ -n "$oro_version" ]]; then
  if [[ "$(echo "$oro_version" | wc -l | tr -d ' ')" != "1" ]]; then
    echo "multiple github.com/duxweb/oro versions found:" >&2
    echo "$oro_version" >&2
    exit 1
  fi
  if ! go list -m -versions github.com/duxweb/oro | grep -q " $oro_version\b"; then
    echo "github.com/duxweb/oro $oro_version is not visible to go list" >&2
    exit 1
  fi
fi

modules=$(go list -m -f '{{.Dir}} {{.Path}}')

echo "Checking release go.mod files"
if rg -n '^replace ' --glob 'go.mod' --glob '!docs/**' .; then
  echo "go.mod replace directives must be removed before release" >&2
  exit 1
fi
if rg -n 'github\.com/duxweb/(runa[^ ]*|oro)\s+v0\.0\.0' --glob 'go.mod' --glob '!docs/**' .; then
  echo "internal v0.0.0 requirements remain" >&2
  exit 1
fi
if rg -n '/Volumes/' go.work $(find . -name go.mod -not -path './docs/node_modules/*') 2>/dev/null; then
  echo "machine-specific paths remain" >&2
  exit 1
fi
if rg -n '=>\\s*/' go.work 2>/dev/null; then
  echo "go.work contains absolute replace paths" >&2
  exit 1
fi

echo "Running Go checks"
go test ./...
go vet ./...
while read -r dir path; do
  echo "==> $path"
  (cd "$dir" && go test ./... && go vet ./...)
done <<< "$modules"

tag_file=$(mktemp)
while read -r dir path; do
  rel=${dir#"$repo_root"}
  rel=${rel#/}
  case "$rel" in
    examples/*) continue ;;
  esac
  if [[ -z "$rel" || "$rel" == "." ]]; then
    echo "$version" >> "$tag_file"
  else
    echo "$rel/$version" >> "$tag_file"
  fi
done <<< "$modules"

echo "Tags to create:"
cat "$tag_file"

if [[ "$dry_run" -eq 1 ]]; then
  echo "Dry-run complete; no commit, tag, or push performed."
  rm -f "$tag_file"
  exit 0
fi

if [[ -n "$(git status --porcelain)" ]]; then
  git add -A
  git commit -m "chore: prepare release $version"
fi
while read -r tag; do
  git tag "$tag"
done < "$tag_file"

if [[ "$push" -eq 1 ]]; then
  git push origin main
  git push origin --tags
fi
rm -f "$tag_file"
