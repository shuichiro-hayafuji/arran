#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
if [[ $# -ne 0 ]]; then
  echo 'Usage: scripts/install-codex-hooks.sh; see tools/HOOKS.md for removal.' >&2
  exit 1
fi
if [[ -L .codex || -L .codex/hooks.json ]]; then
  echo 'Refusing symlink .codex: configuration must remain inside this repository.' >&2
  exit 1
fi
if [[ -e .codex/hooks.json || -L .codex/hooks.json ]]; then
  if cmp -s scripts/codex-hooks.json .codex/hooks.json; then
    echo 'Arran Codex hook definition is already installed. Trust status must be checked in Codex.'
    exit 0
  fi
  echo 'Existing .codex/hooks.json found. Review and merge its hooks manually; nothing was overwritten.' >&2
  exit 1
fi
if [[ -f .codex/config.toml ]] && grep -Eq '^[[:space:]]*(\[.*hooks|hooks[.[:space:]])' .codex/config.toml; then
  echo 'Inline hooks found in .codex/config.toml. Review them first to avoid duplicate notifications.' >&2
  exit 1
fi
if ! mkdir -p .codex || ! (set -o noclobber; cat scripts/codex-hooks.json > .codex/hooks.json); then
  echo 'Cannot create repository .codex/hooks.json. Run this script in a terminal with repository write access.' >&2
  exit 1
fi
echo 'Installed repository Codex Stop hook. Review/trust the hook in Codex; see tools/HOOKS.md.'
