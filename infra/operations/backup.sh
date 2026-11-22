#!/bin/bash
set -euo pipefail
. "$(dirname "$0")/common.sh"
: "${AWS_PROFILE:?Set a dedicated backup-writer profile}"
. "$(dirname "$0")/release-state.sh"
release_lock "${1:-}"
if [ -f state/pending-release ] && [ "${1:-}" != --deployment-lock-held ]; then
  echo "Unfinished release: resume deployment before scheduled backup" >&2; exit 1
fi
[ -s state/applied-release ] || { echo "Missing database applied-release record; refusing an ambiguous backup" >&2; exit 1; }
umask 077
install -d -m 0700 backups
exec 9>backups/backup.lock
flock -n 9 || { echo "Another backup is running" >&2; exit 1; }
tmp=$(mktemp -d backups/work.XXXXXXXX)
trap 'rm -rf -- "$tmp"' EXIT
key="$(date -u +%Y/%m/%d/%Y%m%dT%H%M%SZ)-$(cat /proc/sys/kernel/random/uuid)"
dc exec -T db pg_dump -U arran_migrator -d arran --format=custom > "$tmp/database.dump"
[ -s "$tmp/database.dump" ]
# Parse the archive before upload; a successful upload alone is not a valid backup.
dc exec -T db pg_restore --list < "$tmp/database.dump" >/dev/null
(cd "$tmp" && sha256sum database.dump > database.sha256)
cat state/applied-release > "$tmp/release.txt"
printf 'BACKUP_UTC=%s\n' "$(date -u +%FT%TZ)" >> "$tmp/release.txt"
for file in database.dump database.sha256 release.txt; do
  aws --profile "$AWS_PROFILE" --region ap-northeast-1 s3 cp "$tmp/$file" "s3://$BACKUP_BUCKET/postgres/$key/$file" --sse AES256 --only-show-errors
done
printf '%s\n' "$key" > backups/last-success-key
date -u +%FT%TZ > backups/last-success-at
echo "Backup complete: postgres/$key (verify latest success daily; restore test still required)"
