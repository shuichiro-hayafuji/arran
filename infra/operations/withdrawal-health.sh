#!/bin/bash
set -euo pipefail
. "$(dirname "$0")/common.sh"
[ "${WITHDRAWAL_BATCH_ENABLED:-false}" = true ] || exit 1
systemctl is-active --quiet arran-withdrawal.timer
# Only a completed successful batch is healthy. In particular, systemd's
# auto-restart interval is often "activating", not "failed".
state=$(systemctl show arran-withdrawal.service --property=ActiveState --value)
result=$(systemctl show arran-withdrawal.service --property=Result --value)
completed=$(systemctl show arran-withdrawal.service --property=ExecMainExitTimestampMonotonic --value)
[[ "$state" = inactive && "$result" = success && "$completed" =~ ^[0-9]+$ && "$completed" -gt 0 ]] || exit 1
dc exec -T api /bin/sh /app/entrypoint.sh withdrawal-batch-health
