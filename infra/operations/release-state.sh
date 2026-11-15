#!/bin/bash
# Source after common.sh. State is local to this database/host, never copied from
# another host without verifying the database and matching release manifest.
umask 077
install -d -m 0700 state
release_lock() {
  if [ "${1:-}" = --deployment-lock-held ]; then
    # Internal child invocation: use the deployment's inherited lock descriptor.
    { true >&8; } 2>/dev/null || { echo "Missing deployment lock" >&2; exit 1; }
    flock -n 8 || exit 1
  else
    exec 8>state/release.lock
    flock -n 8 || { echo "Another release or backup is running" >&2; exit 1; }
  fi
}
atomic_copy() {
  local temporary
  temporary=$(mktemp "${2}.XXXXXXXX")
  cat "$1" > "$temporary"
  mv -f "$temporary" "$2"
}
