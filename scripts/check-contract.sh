#!/usr/bin/env bash
# Static check: every path+method in docs/api/openapi.yaml is registered in
# the Go router, and nothing extra is served under /api.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SPEC="$ROOT/docs/api/openapi.yaml"
ROUTER="$ROOT/backend/internal/httpapi/router.go"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

awk '
  /^  \/api\// { path=$1; sub(/:$/, "", path); next }
  /^    (get|post|put|patch|delete):/ {
    verb=toupper($1); sub(/:$/, "", verb); print verb, path
  }
' "$SPEC" | sort -u > "$tmp/spec"

grep -oE '"(GET|POST|PUT|PATCH|DELETE) /api/[^"]*"' "$ROUTER" \
  | tr -d '"' | grep -v '^\(GET\|POST\|PUT\|PATCH\|DELETE\) /api/$' \
  | sort -u > "$tmp/router"

if ! diff -u "$tmp/spec" "$tmp/router"; then
  echo "contract drift: openapi.yaml and router.go disagree" >&2
  exit 1
fi
echo "contract ok: $(wc -l < "$tmp/spec") routes match"