#!/bin/sh
set -eu

if [ "$(id -u)" -eq 0 ] && [ -r /run/secrets/runtime_password ]; then
  install -o postgres -g postgres -m 0400 /run/secrets/runtime_password /run/postgres_runtime_password
fi

exec /usr/local/bin/docker-entrypoint.sh "$@"
