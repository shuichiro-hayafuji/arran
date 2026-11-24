"""Exercise the real health script with isolated systemctl/Docker doubles."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

OPERATIONS = Path(__file__).resolve().parents[1] / 'operations'

class WithdrawalHealthTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        for name in ('common.sh', 'withdrawal-health.sh'):
            shutil.copy(OPERATIONS / name, self.root / name)
        (self.root / 'deploy.env').write_text(
            'DOMAIN=example.org\nAPI_IMAGE=api\nPOSTGRES_IMAGE=postgres\n'
            'CADDY_IMAGE=caddy\nBACKUP_BUCKET=bucket\nWITHDRAWAL_BATCH_ENABLED=true\n')
        self.bin = self.root / 'bin'
        self.bin.mkdir()
        self._command('id', '#!/bin/sh\necho 0\n')
        self._command('docker', '#!/bin/sh\nprintf "%s\\n" "$*" >> "$TEST_LOG"\n')
        self._command('systemctl', '''#!/bin/sh
case "$1" in
  is-active) [ "${TIMER_STATE:-active}" = active ] ;;
  show)
    case "$3" in
      --property=ActiveState) echo "${SERVICE_STATE:-inactive}" ;;
      --property=Result) echo "${SERVICE_RESULT:-success}" ;;
      --property=ExecMainExitTimestampMonotonic) echo "${SERVICE_EXIT:-100}" ;;
      *) exit 2 ;;
    esac ;;
  *) exit 2 ;;
esac
''')
        self.env = dict(os.environ, PATH=str(self.bin) + os.pathsep + os.environ['PATH'],
                        TEST_LOG=str(self.root / 'docker.log'))

    def _command(self, name, text):
        path = self.bin / name
        path.write_text(text)
        path.chmod(0o755)

    def invoke(self, **overrides):
        result = subprocess.run(['bash', str(self.root / 'withdrawal-health.sh')],
                                env=dict(self.env, **overrides), capture_output=True, text=True)
        called = (self.root / 'docker.log').exists()
        return result.returncode, called

    def test_only_completed_successful_service_refreshes_health(self):
        code, called = self.invoke()
        self.assertEqual(code, 0)
        self.assertTrue(called)
        self.assertIn('withdrawal-batch-health', (self.root / 'docker.log').read_text())

    def test_unhealthy_or_unstarted_service_never_refreshes(self):
        for override in (
            {'TIMER_STATE': 'inactive'}, {'SERVICE_STATE': 'activating'},
            {'SERVICE_STATE': 'active'}, {'SERVICE_STATE': 'failed'},
            {'SERVICE_RESULT': 'exit-code'}, {'SERVICE_EXIT': '0'},
            {'SERVICE_EXIT': 'invalid'},
        ):
            with self.subTest(override=override):
                code, called = self.invoke(**override)
                self.assertNotEqual(code, 0)
                self.assertFalse(called)

if __name__ == '__main__':
    unittest.main()
