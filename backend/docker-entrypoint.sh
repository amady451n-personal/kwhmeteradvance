#!/bin/sh
# Bind-mounted /data often arrives owned by root (docker creates a missing
# folder as root, and a root import writes the db as root). Starting as root
# lets us fix ownership once, then drop to the host ids.
set -e
uid="${METERAN_UID:-10001}"
gid="${METERAN_GID:-10001}"
db="${METERAN_DB:-/data/meteran.db}"

check='touch /data/.write-check && rm -f /data/.write-check && { [ ! -e "$DB" ] || [ -w "$DB" ]; }'

if [ "$(id -u)" = "0" ]; then
  chown -R "$uid:$gid" /data || echo "warn: chown -R $uid:$gid /data gagal (mount read-only?)" >&2
  # Fail loudly instead of serving 500s: sqlite writes the folder and the file.
  if ! su-exec "$uid:$gid" env DB="$db" sh -c "$check"; then
    echo "error: /data atau $db tidak bisa ditulis $uid:$gid" >&2
    echo "       perbaiki di host: sudo chown -R $uid:$gid <data folder>" >&2
    exit 1
  fi
  exec su-exec "$uid:$gid" /meteran "$@"
fi

if ! env DB="$db" sh -c "$check"; then
  echo "error: /data atau $db tidak bisa ditulis uid $(id -u)" >&2
  exit 1
fi
exec /meteran "$@"