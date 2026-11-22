#!/bin/sh
set -eu
export DATABASE_URL="$(cat /run/secrets/database_url)"
if [ -f /run/secrets/openai_key ]; then
  export OPENAI_API_KEY="$(cat /run/secrets/openai_key)"
fi
exec /app/spendable-today "$@"
