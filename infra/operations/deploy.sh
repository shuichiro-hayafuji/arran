#!/bin/bash
set -euo pipefail
. "$(dirname "$0")/common.sh"
: "${1:?Usage: deploy.sh initial|update}"
[[ "$1" = initial || "$1" = update ]] || exit 1
# Pin base images by digest. API is an offline-loaded release tag and is not pulled.
[[ "$POSTGRES_IMAGE" = *@sha256:* && "$CADDY_IMAGE" = *@sha256:* ]] || {
  echo "Pin PostgreSQL and Caddy images by digest before deployment" >&2; exit 1;
}
[ -s secrets/openai_key ] || echo "OpenAI key is empty: consultation responses use the server fallback/unavailable behavior" >&2
. "$(dirname "$0")/release-state.sh"
release_lock
image_id=$(docker image inspect --format '{{.Id}}' "$API_IMAGE")
[ -n "$image_id" ] || { echo "Missing API image identity" >&2; exit 1; }
candidate=$(mktemp state/candidate.XXXXXXXX)
trap 'rm -f -- "$candidate"' EXIT
printf 'MODE=%s\nAPI_IMAGE=%s\nAPI_IMAGE_ID=%s\nPOSTGRES_IMAGE=%s\n' "$1" "$API_IMAGE" "$image_id" "$POSTGRES_IMAGE" > "$candidate"
resuming=false
if [ -f state/pending-release ]; then
  cmp -s "$candidate" state/pending-release || {
    echo "Unfinished release: restore its mode and exact image settings before resuming; do not delete state" >&2; exit 1;
  }
  resuming=true
fi
dc config -q
dc up -d --wait db
if [ "$resuming" = false ]; then
  if [ "$1" = initial ]; then
    count=$(dc exec -T db psql -XAt -U postgres -d arran -c "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'")
    [ "$count" = 0 ] && [ ! -f state/applied-release ] || {
      echo "Existing schema without matching initial checkpoint: refuse to overwrite or infer its release" >&2; exit 1;
    }
  else
    [ -s state/applied-release ] || { echo "Missing DB applied-release record; verify the database release before update" >&2; exit 1; }
    dc stop caddy api
    ./backup.sh --deployment-lock-held
  fi
  # Reserve the exact target before the first schema change, including initial deploy.
  atomic_copy "$candidate" state/pending-release
fi
# A failed initial deploy needs no old API container. Re-run the same target's
# idempotent migration; never infer the DB version from a stopped container.
dc stop caddy api
dc run --rm --no-deps migrate
# Persist immediately after migration success, before grants, backup or startup.
atomic_copy state/pending-release state/applied-release
dc exec -T db psql -X -v ON_ERROR_STOP=1 -U postgres -d arran < grant-runtime.sql
./backup.sh --deployment-lock-held
dc up -d --wait api caddy
curl --fail --silent --show-error --retry 6 --retry-delay 5 "https://$DOMAIN/health"
rm -f state/pending-release
printf '\nRelease ready; verify authentication and main API flows on a real client.\n'
