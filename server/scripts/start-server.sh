#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "${SCRIPT_DIR}/.." && pwd)"
GO_BIN="${GO_BIN:-go}"

if ! command -v "${GO_BIN}" >/dev/null 2>&1; then
  echo "error: Go command not found: ${GO_BIN}" >&2
  exit 1
fi

export API_ADDR="${API_ADDR:-0.0.0.0:8080}"
export DATABASE_URL="${DATABASE_URL:-postgres://spendable_today:local-only-password@127.0.0.1:5432/spendable_today?sslmode=disable}"

cd "${ROOT_DIR}"
echo "Starting Spendable Today API at http://${API_ADDR}"
echo "Database: PostgreSQL at ${DATABASE_URL%%\?*}"
echo "LLM configuration: ${USE_MOCK_LLM:-load from .env or use mock default}"

if [ "${API_ADDR%%:*}" = "0.0.0.0" ] && command -v ipconfig >/dev/null 2>&1; then
  port="${API_ADDR##*:}"
  for interface in en0 en1; do
    lan_ip="$(ipconfig getifaddr "${interface}" 2>/dev/null || true)"
    if [ -n "${lan_ip}" ]; then
      echo "LAN access: http://${lan_ip}:${port}"
    fi
  done
fi

case "${API_ADDR}" in
  127.0.0.1:*|localhost:*) tailscale_access_enabled=false ;;
  *) tailscale_access_enabled=true ;;
esac

if [ "${tailscale_access_enabled}" = true ] && command -v tailscale >/dev/null 2>&1; then
  tailscale_ip="$(tailscale ip -4 2>/dev/null || true)"
  if [ -n "${tailscale_ip}" ]; then
    echo "VPN access: http://${tailscale_ip}:${port:-${API_ADDR##*:}}"
  fi
fi

exec "${GO_BIN}" run ./cmd/api
