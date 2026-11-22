#!/bin/bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
[ "$(id -u)" = 0 ] || { echo "Run as root" >&2; exit 1; }
[ -f deploy.env ] || { echo "Create deploy.env using deployment.conf.example" >&2; exit 1; }
set -a
# Operator-owned, non-secret settings; never include tokens or passwords here.
. ./deploy.env
set +a
: "${API_IMAGE:?}" "${POSTGRES_IMAGE:?}" "${CADDY_IMAGE:?}" "${DOMAIN:?}" "${BACKUP_BUCKET:?}"
[[ "$DOMAIN" =~ ^[a-z0-9]([a-z0-9.-]*[a-z0-9])?\.[a-z]{2,}$ ]] || { echo "Invalid DOMAIN" >&2; exit 1; }
[[ "$DOMAIN" != example.com && "$DOMAIN" != *.invalid && "$DOMAIN" != *.example ]] || exit 1
dc() { docker compose --env-file deploy.env -f compose.yaml "$@"; }
