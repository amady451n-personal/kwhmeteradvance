#!/usr/bin/env bash
# Backup the meteranadvanced SQLite database on the STB, keep the last N files.
# Reads the data folder from .env (METERAN_DATA_DIR), same as compose.yml.
# No CLI params. Config from scripts/release.env (STB_HOST, STB_USER, STB_DIR,
# IMAGE, optional BACKUP_DIR and BACKUP_KEEP).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck disable=SC1091
. "$ROOT/scripts/release.env"

: "${STB_HOST:?}" "${STB_USER:?}" "${STB_DIR:?}"
IMAGE="${IMAGE:-meteranadvanced:latest}"
BACKUP_DIR="${BACKUP_DIR:-$STB_DIR/backups}"
BACKUP_KEEP="${BACKUP_KEEP:-14}"
STAMP="$(date +%F-%H%M)"

cmd="set -a; . ./.env; set +a
d=\"\${METERAN_DATA_DIR:-./data}\"
mkdir -p \"\$d\"; d=\$(cd \"\$d\" && pwd)
mkdir -p '$BACKUP_DIR'
docker run --rm --user \"\$(id -u):\$(id -g)\" -v \"\$d:/data\" -v '$BACKUP_DIR:/backups' '$IMAGE' backup '/backups/meteran-$STAMP.db'
ls -1t '$BACKUP_DIR'/meteran-*.db | tail -n +$((BACKUP_KEEP + 1)) | xargs -r rm -f
ls -lh '$BACKUP_DIR' | tail -n 5"

if [ "${DRY_RUN:-0}" = "1" ]; then
  printf '$ ssh %s@%s %q\n' "$STB_USER" "$STB_HOST" "$cmd"
  exit 0
fi

ssh "$STB_USER@$STB_HOST" "$cmd"