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
  run ssh "$STB_USER@$STB_HOST" "cd '$STB_DIR' && $cmd"
}

# Shell snippet run on the STB: resolve METERAN_DATA_DIR from .env into $d and
# make sure the host folder exists (docker would otherwise create it as root).
DATA_CMD='set -a; . ./.env; set +a; d="${METERAN_DATA_DIR:-./data}"; mkdir -p "$d"; d=$(cd "$d" && pwd)'

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
    warn "lalu isi STB_HOST/STB_USER/STB_DIR."
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
  remote "git pull --ff-only"
}

deploy() {
  [ -f "$TARBALL" ] || warn "tarball lokal tidak ada; pastikan image sudah di-scp ke STB"
  say "deploy $IMAGE di STB (container lama tetap jalan sampai image baru siap)"
  if ! ask_yes "Lanjut deploy?"; then
    printf 'Dibatalkan.\n'
    return 0
  fi
  step "1/5 siapkan folder data di host"
  remote "$DATA_CMD"
  # load first: the running container keeps the old image until up -d recreates it
  step "2/5 load image di STB (container lama masih jalan)"
  remote "docker load -i '$REMOTE_TARBALL'"
  step "3/5 jalankan container baru"
  remote "docker compose up -d meteranadvanced"
  step "4/5 bersihkan image dangling"
  remote "docker image prune -f"
  step "5/5 hapus tarball di /tmp"
  remote "rm -f '$REMOTE_TARBALL'"
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
  # import writes into the bind-mounted folder as the host uid
  remote "$DATA_CMD; docker run --rm --user \"\$(id -u):\$(id -g)\" -v /tmp/legacy-meters.db:/import/meters.db:ro -v \"\$d:/data\" '$IMAGE' import --from /import/meters.db"
  step "5/6 bersihkan berkas sementara"
  remote "rm -f /tmp/legacy-meters.db"
  step "6/6 nyalakan container"
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
