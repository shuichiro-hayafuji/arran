#!/bin/bash
set -euo pipefail

root_dir="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root_dir/mobile"
command -v fvm >/dev/null || { echo 'FVM is required.' >&2; exit 1; }
command -v pod >/dev/null || { echo 'CocoaPods is required.' >&2; exit 1; }
xcodebuild -version >/dev/null

# Flutter may return success after an export failure. Require a newly exported IPA
# and its App Store export metadata before reporting success.
marker="$(mktemp "${TMPDIR:-/tmp}/arran-ipa.XXXXXXXX")"
trap 'rm -f "$marker"' EXIT
fvm flutter build ipa --release --export-method app-store \
  --dart-define=API_BASE_URL=https://getarran.com

ipa_dir="$root_dir/mobile/build/ios/ipa"
ipa="$ipa_dir/spendable_today.ipa"
options="$ipa_dir/ExportOptions.plist"
python3 - "$marker" "$ipa" "$options" <<'PY_CHECK'
import pathlib
import plistlib
import sys

marker, ipa, options = map(pathlib.Path, sys.argv[1:])
try:
    if any(not p.is_file() or p.stat().st_mtime_ns <= marker.stat().st_mtime_ns for p in (ipa, options)):
        raise ValueError('IPA export did not produce a new IPA and ExportOptions.plist.')
    with options.open('rb') as file:
        method = plistlib.load(file).get('method')
    if method not in ('app-store', 'app-store-connect'):
        raise ValueError(f'Unexpected IPA export method: {method}')
except (OSError, ValueError, plistlib.InvalidFileException) as error:
    sys.exit(str(error))
PY_CHECK
printf 'App Store IPA: %s\n' "$ipa"
