#!/bin/sh
set -eu

password_file=/run/postgres_runtime_password
if [ ! -r "$password_file" ]; then
  printf '%s\n' 'runtime password file unavailable' >&2
  exit 1
fi
runtime_password=$(cat "$password_file")
case "$runtime_password" in
  ''|*[!0123456789abcdef]*)
    printf '%s\n' 'runtime password has invalid local format' >&2
    exit 1
    ;;
esac

export ESPACIGO_RUNTIME_PASSWORD="$runtime_password"
export ESPACIGO_DATABASE_NAME="$POSTGRES_DB"
psql --no-psqlrc --quiet --set=ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<'SQL'
\getenv runtime_password ESPACIGO_RUNTIME_PASSWORD
\getenv database_name ESPACIGO_DATABASE_NAME
CREATE ROLE espacigo_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT PASSWORD :'runtime_password';
GRANT CONNECT ON DATABASE :"database_name" TO espacigo_runtime;
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS btree_gist;
SQL
unset ESPACIGO_RUNTIME_PASSWORD ESPACIGO_DATABASE_NAME runtime_password
