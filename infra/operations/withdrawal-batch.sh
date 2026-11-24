#!/bin/bash
set -euo pipefail
. "$(dirname "$0")/common.sh"
[ "${WITHDRAWAL_BATCH_ENABLED:-false}" = true ] || { echo "Withdrawal batch is disabled" >&2; exit 1; }
dc exec -T api /bin/sh /app/entrypoint.sh withdrawal-batch
