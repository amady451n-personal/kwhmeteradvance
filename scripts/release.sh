#!/usr/bin/env bash
# Interactive release driver. No CLI params: pick a menu number.
#
# Config lives in ~/.config/meteran/release.env (outside the repo, so no update
# can remove it) or $RELEASE_ENV; an in-repo scripts/release.env is migrated
# once. See scripts/release.env.example for the fields.
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
  if [ "$MODE" = "local" ]; then
    run bash -c "cd '$WORKDIR' && $cmd"
  else
    run ssh "$STB_USER@$STB_HOST" "cd '$WORKDIR' && $cmd"
  fi
}

# Shell snippet run on the STB: make sure .env exists, resolve METERAN_DATA_DIR
# from it into $d, and create the host folder (docker would create it as root).
DATA_CMD='[ -f .env ] || cp .env.example .env; set -a; . ./.env; set +a; d="${METERAN_DATA_DIR:-./data}"; mkdir -p "$d"; d=$(cd "$d" && pwd)'

ask_yes() {
  local prompt="$1" answer
  if [ "${DRY_RUN:-0}" = "1" ]; then
    note "[dry-run] dianggap '$prompt' = ya"
    return 0
  fi
  [ -t 0 ] || die "stdin bukan terminal"
  read -r -p "$prompt [y/N] " answer
  case "$answer" in
    y | Y | yes | YES) return 0 ;;
    *) return 1 ;;
  esac
}

# Config lives outside the repo so no checkout/update can delete it.
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/meteran"
CONFIG_FILE="$CONFIG_DIR/release.env"

load_config() {
  local legacy="$ROOT/scripts/release.env"
  if [ -z "${RELEASE_ENV:-}" ] && [ ! -f "$CONFIG_FILE" ] && [ -f "$legacy" ]; then
    # one-time move of the old in-repo file, which stays in place as a backup
    mkdir -p "$CONFIG_DIR"
    cp "$legacy" "$CONFIG_FILE"
    chmod 600 "$CONFIG_FILE"
    note "config lama dipindahkan ke $CONFIG_FILE"
  fi

  local file="${RELEASE_ENV:-$CONFIG_FILE}"
  if [ ! -f "$file" ]; then
    warn "$file belum ada."
    warn "mkdir -p $CONFIG_DIR && cp scripts/release.env.example $CONFIG_FILE"
    warn "lalu isi STB_DIR (wajib) dan STB_HOST/STB_USER (hanya mode remote)."
    exit 1
  fi
  # shellcheck disable=SC1090
  . "$file"
  IMAGE="${IMAGE:-meteranadvanced:latest}"

  # auto: local when this checkout is the deploy folder (running on the host),
  # remote when it only exists on the other machine.
  MODE="${STB_MODE:-auto}"
  if [ "$MODE" = "auto" ]; then
    if [ -n "${STB_DIR:-}" ] && [ -d "$STB_DIR" ]; then
      MODE=local
    else
      MODE=remote
    fi
  fi
  case "$MODE" in
    local)
      WORKDIR="$ROOT"
      note "mode lokal: semua perintah dijalankan langsung di mesin ini"
      ;;
    remote)
      : "${STB_HOST:?STB_HOST kosong}" "${STB_USER:?STB_USER kosong}" "${STB_DIR:?STB_DIR kosong}"
      WORKDIR="$STB_DIR"
      note "mode remote: perintah dijalankan lewat ssh $STB_USER@$STB_HOST"
      ;;
    *) die "STB_MODE harus auto|local|remote (sekarang: $MODE)" ;;
  esac
}

build_image() {
  local start=$SECONDS
  say "build arm64 image $IMAGE"
  note "tahap 1/2: npm build + go build (biasanya 1-3 menit, output di bawah)"
  if [ "$(uname -m)" = "aarch64" ]; then
    # already on arm64: native build, no buildx/qemu needed
    run docker build -t "$IMAGE" -f backend/Dockerfile .
  else
    run docker buildx build --platform linux/arm64 -t "$IMAGE" --load -f backend/Dockerfile .
  fi
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
  if [ "$MODE" = "local" ]; then
    note "mode lokal: image sudah ada di mesin ini, scp dilewati"
    return 0
  fi
  [ -f "$TARBALL" ] || die "tarball belum ada: $TARBALL (jalankan menu 1)"
  say "scp image -> $STB_HOST:$REMOTE_TARBALL"
  note "transfer $(du -h "$TARBALL" | cut -f1), progress bar muncul di bawah"
  run scp "$TARBALL" "$STB_USER@$STB_HOST:$REMOTE_TARBALL"
  note "transfer selesai"
}

pull_git() {
  say "git pull --ff-only"
  remote "git pull --ff-only"
}

deploy() {
  if [ "$MODE" = "remote" ] && [ ! -f "$TARBALL" ]; then
    warn "tarball lokal tidak ada; pastikan image sudah di-scp ke STB"
  fi
  say "deploy $IMAGE di STB (container lama tetap jalan sampai image baru siap)"
  if ! ask_yes "Lanjut deploy?"; then
    printf 'Dibatalkan.\n'
    return 0
  fi
  step "1/5 siapkan .env dan folder data di host"
  remote "$DATA_CMD"
  if [ "$MODE" = "local" ]; then
    step "2/5 image hasil build lokal, docker load dilewati"
  else
    # load first: the running container keeps the old image until up -d recreates it
    step "2/5 load image di STB (container lama masih jalan)"
    remote "docker load -i '$REMOTE_TARBALL'"
  fi
  step "3/5 jalankan container baru"
  remote "docker compose up -d meteranadvanced"
  step "4/5 bersihkan image dangling"
  remote "docker image prune -f"
  if [ "$MODE" = "local" ]; then
    step "5/5 tidak ada tarball di /tmp yang perlu dihapus"
  else
    step "5/5 hapus tarball di /tmp"
    remote "rm -f '$REMOTE_TARBALL'"
  fi
  status
}

reset_import() {
  say "kosongkan folder data host lalu impor meters.db milik kwhmeter basic"
  warn "semua data meteranadvanced (readings/tokens/settings/PIN) akan hilang."
  warn "folder data diambil dari METERAN_DATA_DIR di .env milik STB."
  ask_yes "Hapus isi folder data dan impor ulang?" || {
    printf 'Dibatalkan.\n'
    return 0
  }
  step "1/6 hentikan container"
  remote "docker compose down"

  local src
  read -r -p "Path meters.db di STB (milik kwhmeter basic): " src
  [ -n "$src" ] || die "path kosong"
  step "2/6 kosongkan folder data"
  remote "$DATA_CMD; find \"\$d\" -mindepth 1 -delete"
  step "3/6 salin database legacy ke /tmp"
  # copy first: the container user cannot rely on the host file's permissions
  remote "cp '$src' /tmp/legacy-meters.db" || die "gagal menyalin $src"
  step "4/6 impor (importer menolak bila DB tidak kosong)"
  # import writes into the bind-mounted folder; the entrypoint drops to these ids
  remote "$DATA_CMD; docker run --rm -e METERAN_UID=\"\${METERAN_UID:-1000}\" -e METERAN_GID=\"\${METERAN_GID:-1000}\" -v /tmp/legacy-meters.db:/import/meters.db:ro -v \"\$d:/data\" '$IMAGE' import --from /import/meters.db"
  step "5/6 bersihkan berkas sementara"
  remote "rm -f /tmp/legacy-meters.db"
  step "6/6 nyalakan container"
  remote "docker compose up -d meteranadvanced"
  status
}

status() {
  say "status container"
  remote "docker compose ps meteranadvanced"
  say "health /api/health"
  # port comes from the .env of the machine that runs compose
  remote 'set -a; . ./.env 2>/dev/null; set +a; curl -fsS "http://localhost:${METERAN_PORT:-8080}/api/health"' || warn "health belum OK"
}

menu() {
  cat <<'EOF'

  meteranadvanced release   (mode: STB_MODE=auto|local|remote)
  -----------------------
  1) build & package   (native di arm64, buildx di amd64 + docker save)
  2) git push          (dari repo ini)
  3) scp image         (hanya mode remote)
  4) git pull          (di folder deploy)
  5) deploy            (load bila remote -> compose up -d -> prune -> rm /tmp)
  6) reset & import    (down -> kosongkan folder data -> impor meters.db basic)
  7) status / health
  0) keluar
EOF
}

load_config
menu
while true; do
  read -r -p 'Pilih: ' choice || exit 0
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
