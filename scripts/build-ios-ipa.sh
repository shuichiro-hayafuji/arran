#!/bin/bash
set -euo pipefail

# Set API_BASE_URL to override the production endpoint when needed.
api_base_url="${API_BASE_URL:-https://getarran.com}"
export_method="${IOS_EXPORT_METHOD:-development}"

python3 - "$api_base_url" <<'PY'
import sys
from urllib.parse import urlsplit

url = sys.argv[1]
try:
    parsed = urlsplit(url)
    valid = (parsed.scheme == 'https' and parsed.hostname and parsed.port != 0
             and not parsed.username and not parsed.password
             and not parsed.query and not parsed.fragment
             and not any(c.isspace() for c in url))
except ValueError:
    valid = False
if not valid:
    sys.exit('API_BASE_URL must be an HTTPS URL without credentials, query, or fragment.')
PY

case "$export_method" in
  development|ad-hoc|app-store|enterprise) ;;
  *) echo "Unsupported IOS_EXPORT_METHOD: $export_method" >&2; exit 1 ;;
esac

root_dir="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root_dir/mobile"
command -v fvm >/dev/null || { echo 'FVM is required.' >&2; exit 1; }
command -v pod >/dev/null || { echo 'CocoaPods is required.' >&2; exit 1; }
xcodebuild -version >/dev/null
printf 'Building iOS IPA (API: %s, export: %s)\n' "$api_base_url" "$export_method"
marker="$(mktemp "${TMPDIR:-/tmp}/arran-ipa.XXXXXXXX")"
trap 'rm -f "$marker"' EXIT
fvm flutter build ipa --release --export-method "$export_method" --dart-define="API_BASE_URL=$api_base_url"

# Flutter can return success when exporting the archive fails. Do not report an
# older IPA or an IPA exported for a different distribution method as success.
python3 - "$marker" "$root_dir/mobile/build/ios/ipa" "$export_method" <<'PY'
import pathlib
import plistlib
import sys

marker = pathlib.Path(sys.argv[1])
ipa_dir = pathlib.Path(sys.argv[2])
requested = sys.argv[3]
ipa = ipa_dir / 'spendable_today.ipa'
options = ipa_dir / 'ExportOptions.plist'
methods = {
    'development': ('development', 'debugging'),
    'ad-hoc': ('ad-hoc', 'release-testing'),
    'app-store': ('app-store', 'app-store-connect'),
    'enterprise': ('enterprise',),
}
try:
    if any(not p.is_file() or p.stat().st_mtime_ns <= marker.stat().st_mtime_ns for p in (ipa, options)):
        raise ValueError('IPA export did not produce a new IPA and ExportOptions.plist.')
    with options.open('rb') as file:
        method = plistlib.load(file).get('method')
    if method not in methods[requested]:
        raise ValueError(f'Unexpected IPA export method: {method}')
except (OSError, ValueError, plistlib.InvalidFileException) as error:
    sys.exit(str(error))
print(f'IPA: {ipa}')
PY
