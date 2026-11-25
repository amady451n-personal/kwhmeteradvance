#!/usr/bin/env bash
# Interactive release driver. No CLI params: pick a menu number.
#
# Config lives in scripts/release.env (untracked):
#   STB_HOST=192.168.1.10
#   STB_USER=armbian
#   STB_DIR=/home/armbian/dipsik
#   IMAGE=meteranadvanced:latest
#
# Set DRY_RUN=1 to print every command instead of running it.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

TARBALL="dist/meteranadvanced-image.tar.gz"
REMOTE_TARBALL="/tmp/meteranadvanced-image.tar.gz"

say() { printf '\n== %s\n' "$*"; }
warn() { printf '!! %s\n' "$*" >&2; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

run() {
  if [ "${DRY_RUN:-0}" = "1" ]; then
    printf '   $ %s\n' "$*"
    return 0
  fi
  "$@"
}

remote() {
  local cmd="$1"
  run ssh "$STB_USER@$STB_HOST" "cd '$STB_DIR' && $cmd"
}

ask_yes() {
  local prompt="$1" answer
  [ -t 0 ] || die "stdin bukan terminal"
  read -r -p "$prompt [y/N] " answer
  case "$answer" in
    y | Y | yes | YES) return 0 ;;
    *) return 1 ;;
  esac
}

load_config() {
  local file="$ROOT/scripts/release.env"
  if [ ! -f "$file" ]; then
    warn "scripts/release.env belum ada."
    warn "salin dari scripts/release.env.example lalu isi STB_HOST/STB_USER/STB_DIR."
    exit 1
  fi
  # shellcheck disable=SC1090
  . "$file"
  : "${STB_HOST:?STB_HOST kosong}" "${STB_USER:?STB_USER kosong}" "${STB_DIR:?STB_DIR kosong}"
  IMAGE="${IMAGE:-meteranadvanced:latest}"
}

build_image() {
  say "build arm64 image $IMAGE"
  run docker buildx build --platform linux/arm64 -t "$IMAGE" --load -f backend/Dockerfile .
  say "package image -> $TARBALL"
  mkdir -p dist
  if [ "${DRY_RUN:-0}" = "1" ]; then
    printf '   $ docker save %s | gzip > %s\n' "$IMAGE" "$TARBALL"
  else
    docker save "$IMAGE" | gzip > "$TARBALL"
    ls -lh "$TARBALL"
  fi
}

push_git() {
  say "git push (lokal)"
  run git push origin HEAD
}

scp_image() {
  [ -f "$TARBALL" ] || die "tarball belum ada: $TARBALL (jalankan menu 1)"
  say "scp image -> $STB_HOST:$REMOTE_TARBALL"
  run scp "$TARBALL" "$STB_USER@$STB_HOST:$REMOTE_TARBALL"
}

pull_git() {
  say "git pull di STB"
  remote "git pull --ff-only"
}

deploy() {
  [ -f "$TARBALL" ] || warn "tarball lokal tidak ada; pastikan image sudah di-scp ke STB"
  say "deploy $IMAGE di STB (container lama tetap jalan sampai image baru siap)"
  if ! ask_yes "Lanjut deploy?"; then
    printf 'Dibatalkan.\n'
    return 0
  fi
  # load first: the running container keeps the old image until up -d recreates it
  remote "docker load -i '$REMOTE_TARBALL'"
  remote "docker compose up -d meteranadvanced"
  remote "docker image prune -f"
  remote "rm -f '$REMOTE_TARBALL'"
  status
}

reset_import() {
  say "reset volume meteranadvanced-data lalu impor meters.db milik kwhmeter basic"
  warn "semua data meteranadvanced (readings/tokens/settings/PIN) akan hilang."
  ask_yes "Hapus volume dan impor ulang?" || {
    printf 'Dibatalkan.\n'
    return 0
  }
  remote "docker compose down -v meteranadvanced"

  local src
  read -r -p "Path meters.db di STB (milik kwhmeter basic): " src
  [ -n "$src" ] || die "path kosong"
  remote "[ -f '$src' ]"
  # copy first: the container user cannot rely on the host file's permissions
  remote "cp '$src' /tmp/legacy-meters.db"
  # import runs against the fresh named volume; log output stays visible
  remote "docker run --rm -v /tmp/legacy-meters.db:/import/meters.db:ro -v meteranadvanced-data:/data '$IMAGE' import --from /import/meters.db"
  remote "rm -f /tmp/legacy-meters.db"
  remote "docker compose up -d meteranadvanced"
  status
}

status() {
  local port
  port="$(awk -F= '/^METERAN_PORT=/{gsub(/[ \t]/, "", $2); print $2}' "$ROOT/.env")"
  port="${port:-8080}"
  say "status container"
  remote "docker compose ps meteranadvanced"
  say "health http://localhost:$port/api/health"
  remote "curl -fsS http://localhost:$port/api/health" || warn "health belum OK"
}

menu() {
  cat <<'EOF'

  meteranadvanced release
  -----------------------
  1) build & package   (buildx arm64 + docker save | gzip)
  2) git push          (lokal)
  3) scp image         (ke STB)
  4) git pull          (di STB)
  5) deploy            (load -> compose up -d -> prune -> rm /tmp)
  6) reset & import    (down -v -> impor meters.db kwhmeter basic)
  7) status / health
  0) keluar
EOF
}

load_config
menu
while true; do
  read -r -p 'Pilih: ' choice
  case "$choice" in
    1) build_image ;;
    2) push_git ;;
    3) scp_image ;;
    4) pull_git ;;
    5) deploy ;;
    6) reset_import ;;
    7) status ;;
    0 | q | Q) exit 0 ;;
    *) printf 'Pilihan tidak dikenal.\n' ;;
  esac
done