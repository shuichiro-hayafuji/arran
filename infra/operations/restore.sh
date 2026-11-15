#!/bin/bash
set -euo pipefail
. "$(dirname "$0")/common.sh"
# Download via an operator profile to a root-only directory first. The backup
# writer deliberately has no S3 read or delete permission.
: "${1:?Usage: restore.sh absolute-backup-directory arran_restore_NAME}"
: "${2:?Choose a new arran_restore_NAME database}"
[[ "$1" = /* && "$2" =~ ^arran_restore_[a-z0-9_]+$ && ${#2} -le 63 ]] || {
  echo "Require an absolute directory and a fresh arran_restore_* database" >&2; exit 1;
}
dir=$1
(cd "$dir" && sha256sum --check database.sha256)
dc exec -T db createdb -U postgres --owner=arran_migrator "$2"
# No --clean and no DROP: an existing database causes createdb to fail.
dc exec -T db pg_restore -U arran_migrator -d "$2" --no-owner --exit-on-error --single-transaction < "$dir/database.dump"
dc exec -T db psql -X -v ON_ERROR_STOP=1 -U postgres -d "$2" -c "REVOKE ALL ON DATABASE $2 FROM PUBLIC; GRANT CONNECT ON DATABASE $2 TO arran_runtime;"
dc exec -T db psql -X -v ON_ERROR_STOP=1 -U postgres -d "$2" < grant-runtime.sql
dc exec -T db psql -X -v ON_ERROR_STOP=1 -U postgres -d "$2" -c 'SELECT version, applied_at FROM schema_migrations ORDER BY version'
echo "Restored into $2. Production connection has NOT changed. Verify data before switching."
