#!/usr/bin/env bash
# Kelola image Docker yang di-pull khusus proyek ini.
# Hanya membaca/menghapus ref di infra/images.pulled.txt.
# Image di infra/images.base.txt tidak pernah disentuh.
#
#   bash scripts/dev-images.sh list
#   bash scripts/dev-images.sh add <image:tag> [catatan]
#   bash scripts/dev-images.sh rm [--yes]
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PULLED_FILE="$ROOT_DIR/infra/images.pulled.txt"
BASE_FILE="$ROOT_DIR/infra/images.base.txt"

die() { printf 'error: %s\n' "$*" >&2; exit 1; }

[ -f "$PULLED_FILE" ] || die "file tidak ditemukan: $PULLED_FILE"

read_refs() {
  local file="$1"
  [ -f "$file" ] || return 0
  awk -F'|' '
    /^[[:space:]]*#/ { next }
    /^[[:space:]]*$/ { next }
    {
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", $1)
      if ($1 != "") print $1
    }
  ' "$file"
}

is_base() {
  read_refs "$BASE_FILE" | grep -Fxq "$1"
}

size_of() {
  docker image inspect "$1" --format '{{.Size}}' 2>/dev/null \
    | awk '{printf "%.1f MB", $1/1000000}'
}

cmd_list() {
  local found=0 ref
  printf '%-46s %-10s %s\n' "IMAGE" "SIZE" "STATUS"
  while IFS= read -r ref; do
    found=1
    if docker image inspect "$ref" >/dev/null 2>&1; then
      printf '%-46s %-10s %s\n' "$ref" "$(size_of "$ref")" "present"
    else
      printf '%-46s %-10s %s\n' "$ref" "-" "missing"
    fi
  done < <(read_refs "$PULLED_FILE")
  [ "$found" -eq 1 ] || printf '(belum ada image yang di-pull khusus proyek ini)\n'
}

cmd_rm() {
  local assume_yes="${1:-}" ref answer
  local -a refs=()
  while IFS= read -r ref; do refs+=("$ref"); done < <(read_refs "$PULLED_FILE")

  if [ "${#refs[@]}" -eq 0 ]; then
    printf 'Tidak ada image di %s\n' "$PULLED_FILE"
    return 0
  fi

  printf 'Image yang akan dihapus:\n'
  for ref in "${refs[@]}"; do
    if is_base "$ref"; then
      printf '  SKIP  %s (ada di images.base.txt)\n' "$ref"
    else
      printf '  hapus %s\n' "$ref"
    fi
  done

  if [ "$assume_yes" != "--yes" ]; then
    [ -t 0 ] || die "stdin bukan terminal; jalankan dengan --yes"
    read -r -p 'Lanjut? [y/N] ' answer
    case "$answer" in
      y|Y|yes|YES) ;;
      *) printf 'Dibatalkan.\n'; return 0 ;;
    esac
  fi

  for ref in "${refs[@]}"; do
    if is_base "$ref"; then continue; fi
    if docker image inspect "$ref" >/dev/null 2>&1; then
      if docker image rm "$ref"; then
        printf 'dihapus: %s\n' "$ref"
      else
        printf 'gagal: %s (mungkin masih dipakai container)\n' "$ref"
      fi
    else
      printf 'tidak ada: %s\n' "$ref"
    fi
  done
}

cmd_add() {
  local ref="${1:-}" note="${2:-dev}"
  [ -n "$ref" ] || die "usage: dev-images.sh add <image:tag> [catatan]"
  if is_base "$ref"; then
    die "$ref sudah ada di images.base.txt; tidak perlu dicatat sebagai pull"
  fi
  printf '%s | %s | %s\n' "$ref" "$(date +%F)" "$note" >> "$PULLED_FILE"
  printf 'dicatat: %s\n' "$ref"
}

case "${1:-list}" in
  list) cmd_list ;;
  rm) shift; cmd_rm "${1:-}" ;;
  add) shift; cmd_add "${1:-}" "${2:-dev}" ;;
  *) die "usage: dev-images.sh [list|rm [--yes]|add <image:tag> [catatan]]" ;;
esac
