#!/bin/bash
set -euo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd)"
export IOS_EXPORT_METHOD=app-store
exec "$script_dir/build-ios-ipa.sh"
