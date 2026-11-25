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

stamp() { date +%H:%M:%S; }
say() { printf '\n[%s] == %s\n' "$(stamp)" "$*"; }
step() { printf '[%s] -- %s\n' "$(stamp)" "$*"; }
note() { printf '[%s]    %s\n' "$(stamp)" "$*"; }
warn() { printf '[%s] !! %s\n' "$(stamp)" "$*" >&2; }
die() { printf '[%s] error: %s\n' "$(stamp)" "$*" >&2; exit 1; }
trap 'warn "gagal di baris $LINENO (exit $?)"' ERR

run() {
  note "\$ $*"
  if [ "${DRY_RUN:-0}" = "1" ]; then
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
  local start=$SECONDS
  say "build arm64 image $IMAGE"
  note "tahap 1/2: npm build + go build (biasanya 1-3 menit, output di bawah)"
  run docker buildx build --platform linux/arm64 -t "$IMAGE" --load -f backend/Dockerfile .
  note "build selesai dalam $((SECONDS - start))s"

  say "package image -> $TARBALL"
  note "tahap 2/2: docker save | gzip (biasanya 1-2 menit, tanpa output)"
  mkdir -p dist
  if [ "${DRY_RUN:-0}" = "1" ]; then
    note "\$ docker save $IMAGE | gzip > $TARBALL"
  else
    start=$SECONDS
    docker save "$IMAGE" | gzip > "$TARBALL"
    note "paket selesai dalam $((SECONDS - start))s"
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
  note "transfer $(du -h "$TARBALL" | cut -f1), progress bar muncul di bawah"
  run scp "$TARBALL" "$STB_USER@$STB_HOST:$REMOTE_TARBALL"
  note "transfer selesai"
}

pull_git() {
  say "git pull di STB"
  run git pull --ff-only
}

deploy() {
  [ -f "$REMOTE_TARBALL" ] || warn "tarball lokal tidak ada; pastikan image sudah di-scp ke STB"
  say "deploy $IMAGE di STB (container lama tetap jalan sampai image baru siap)"
  if ! ask_yes "Lanjut deploy?"; then
    printf 'Dibatalkan.\n'
    return 0
  fi
  # load first: the running container keeps the old image until up -d recreates it
  step "1/4 load image di STB (container lama masih jalan)"
  run docker load -i "$REMOTE_TARBALL"
  step "2/4 jalankan container baru"
  run docker compose up -d meteranadvanced
  step "3/4 bersihkan image dangling"
  run docker image prune -f
  step "4/4 hapus tarball di /tmp"
  run rm -f "$REMOTE_TARBALL"
  status
}

reset_import() {
  say "reset volume meteranadvanced-data lalu impor meters.db milik kwhmeter basic"
  warn "semua data meteranadvanced (readings/tokens/settings/PIN) akan hilang."
  ask_yes "Hapus volume dan impor ulang?" || {
    printf 'Dibatalkan.\n'
    return 0
  }
  step "1/5 hapus container + volume"
  remote "docker compose down -v"

  local src
  read -r -p "Path meters.db di STB (milik kwhmeter basic): " src
  [ -n "$src" ] || die "path kosong"
  step "2/5 salin database legacy ke /tmp"
  # copy first: the container user cannot rely on the host file's permissions
  remote "cp '$src' /tmp/legacy-meters.db" || die "gagal menyalin $src"
  step "3/5 impor (hanya jalan bila DB kosong)"
  # import runs against the fresh named volume; log output stays visible
  remote "docker run --rm -v /tmp/legacy-meters.db:/import/meters.db:ro -v meteranadvanced-data:/data '$IMAGE' import --from /import/meters.db"
  step "4/5 bersihkan berkas sementara"
  remote "rm -f /tmp/legacy-meters.db"
  step "5/5 nyalakan container"
  remote "docker compose up -d meteranadvanced"
  status
}

status() {
  local port
  port="$(awk -F= '/^METERAN_PORT=/{gsub(/[ \t]/, "", $2); print $2}' "$ROOT/.env")"
  port="${port:-8080}"
  say "status container"
  run docker compose ps meteranadvanced
  say "health http://localhost:$port/api/health"
  run curl -fsS http://localhost:$port/api/health" || warn "health belum OK
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
  start=$SECONDS
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
  step "menu $choice selesai dalam $((SECONDS - start))s"
done
