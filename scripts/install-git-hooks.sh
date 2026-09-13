#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
existing=$(git config --get core.hooksPath || true)
if [[ -n "$existing" && "$existing" != .githooks ]]; then
  printf 'Existing core.hooksPath=%s; integrate the hooks before changing it.\n' "$existing" >&2
  exit 1
fi
if [[ -z "$existing" && -f "$(git rev-parse --git-path hooks/pre-commit)" ]]; then
  echo 'An existing pre-commit hook was found; integrate it first.' >&2
  exit 1
fi
git config --local core.hooksPath .githooks
echo 'Enabled Dart pre-commit formatting (.githooks).'
