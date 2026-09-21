#!/usr/bin/env bash
set -euo pipefail
trap 'echo "Git hook configuration failed. Run this script in a terminal with repository write access; see tools/HOOKS.md." >&2' ERR
cd "$(git rev-parse --show-toplevel)"
if [[ "${1:-}" == "--uninstall" ]]; then
  if [[ "$(git config --local --get core.hooksPath || true)" == .githooks ]]; then
    git config --local --unset-all core.hooksPath
    echo 'Removed repository core.hooksPath. Inherited Git hooks, if any, apply again.'
  else
    echo 'No Arran repository-local hooks setting to remove.'
  fi
  exit 0
fi
if [[ $# -ne 0 ]]; then
  echo 'Usage: scripts/install-git-hooks.sh [--uninstall]' >&2
  exit 1
fi
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
echo 'Enabled repository pre-commit checks (.githooks). See tools/HOOKS.md.'
