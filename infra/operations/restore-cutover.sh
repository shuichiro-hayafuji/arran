#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"
[ "$(id -u)" = 0 ] || { echo 'Cutover request not recorded: run as root.' >&2; exit 1; }
. ./release-state.sh
exec 8>state/release.lock
flock -n 8 || { echo 'Cutover request not recorded: release or backup is running; retry after it completes.' >&2; exit 1; }
# Independent authenticated and fresh withdrawal/consent evidence and its
# reconciliation protocol do not yet exist. No argument, file or environment
# variable is evidence. Keep a local deployment interlock after any attempt.
# This is not a guard against direct DB access, old scripts, or another host.
if [ ! -e state/restore-cutover-blocked ]; then
  ( set -o noclobber; printf '%s\n' 'restore cutover blocked: reconciliation unavailable' > state/restore-cutover-blocked )
fi
echo 'Deployment stop recorded; a running API remains active and must be stopped separately.' >&2
echo 'Restore cutover refused: independent reconciliation unavailable. This records a deployment stop; it does not stop a running API.' >&2
exit 1
