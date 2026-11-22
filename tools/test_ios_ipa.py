import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


class IosIpaScriptTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        source = Path(__file__).resolve().parent.parent / 'scripts'
        shutil.copytree(source, self.root / 'scripts')
        (self.root / 'mobile').mkdir()
        binaries = self.root / 'bin'
        binaries.mkdir()
        for name in ('pod', 'xcodebuild'):
            file = binaries / name
            file.write_text('#!/bin/sh\nexit 0\n')
            file.chmod(0o755)
        fvm = binaries / 'fvm'
        fvm.write_text('''#!/usr/bin/env python3
import os, pathlib, plistlib, sys, time
root = pathlib.Path.cwd()
(root / 'arguments').write_text('\\n'.join(sys.argv[1:]))
if os.environ.get('FAKE_EXPORT') == 'none':
    sys.exit(0)
out = root / 'build/ios/ipa'
out.mkdir(parents=True, exist_ok=True)
ipa = out / 'spendable_today.ipa'
ipa.write_bytes(b'fake')
options = out / 'ExportOptions.plist'
with options.open('wb') as f:
    plistlib.dump({'method': os.environ.get('FAKE_METHOD', 'app-store-connect')}, f)
for path in (ipa, options):
    os.utime(path, (time.time() + 2, time.time() + 2))
''')
        fvm.chmod(0o755)
        self.env = dict(os.environ, PATH=f'{binaries}:{os.environ["PATH"]}')
        self.env.pop('API_BASE_URL', None)
        self.env.pop('IOS_EXPORT_METHOD', None)

    def run_script(self, name, **env):
        return subprocess.run(
            [str(self.root / 'scripts' / name)],
            env={**self.env, **env}, capture_output=True, text=True,
        )

    def test_app_store_entry_uses_production_url_and_export_method(self):
        result = self.run_script('build-ios-release-ipa.sh')
        self.assertEqual(result.returncode, 0, result.stderr)
        args = (self.root / 'mobile/arguments').read_text().splitlines()
        self.assertIn('--dart-define=API_BASE_URL=https://api.getarran.com', args)
        self.assertEqual(args[args.index('--export-method') + 1], 'app-store')

    def test_development_entry_uses_production_url(self):
        result = self.run_script('build-ios-ipa.sh', FAKE_METHOD='development')
        self.assertEqual(result.returncode, 0, result.stderr)
        args = (self.root / 'mobile/arguments').read_text().splitlines()
        self.assertIn('--dart-define=API_BASE_URL=https://api.getarran.com', args)
        self.assertEqual(args[args.index('--export-method') + 1], 'development')

    def test_custom_https_url_is_passed_to_flutter(self):
        result = self.run_script('build-ios-release-ipa.sh', API_BASE_URL='https://example.com')
        self.assertEqual(result.returncode, 0, result.stderr)
        args = (self.root / 'mobile/arguments').read_text().splitlines()
        self.assertIn('--dart-define=API_BASE_URL=https://example.com', args)

    def test_unsafe_urls_are_rejected_before_build(self):
        for url in ('http://localhost:8080', 'https://user:pass@example.com', 'https://example.com/?token=x'):
            with self.subTest(url=url):
                result = self.run_script('build-ios-release-ipa.sh', API_BASE_URL=url)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse((self.root / 'mobile/arguments').exists())

    def test_missing_or_wrong_export_is_not_reported_as_success(self):
        self.assertNotEqual(self.run_script('build-ios-release-ipa.sh', FAKE_EXPORT='none').returncode, 0)
        self.assertNotEqual(self.run_script('build-ios-release-ipa.sh', FAKE_METHOD='development').returncode, 0)


if __name__ == '__main__':
    unittest.main()
