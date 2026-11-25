#!/bin/sh
# Bind-mounted /data often arrives owned by root (docker creates a missing
# folder as root). Starting as root lets us fix the owner once, then drop.
set -e
uid="${METERAN_UID:-10001}"
gid="${METERAN_GID:-10001}"

if [ "$(id -u)" = "0" ]; then
  if [ -d /data ] && ! chown -R "$uid:$gid" /data 2>/dev/null; then
    echo "warn: cannot chown /data to $uid:$gid (read-only mount?)" >&2
  fi
  exec su-exec "$uid:$gid" /meteran "$@"
fi

exec /meteran "$@"