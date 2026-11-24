"""Run the actual release scripts with isolated Docker/AWS command doubles.

No root, Docker daemon, network, or real credentials are used. flock's command
interface is adapted to Python fcntl so the scripts' real advisory lock is tested
on developer machines as well as Linux.
"""
import fcntl
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

OPERATIONS = Path(__file__).resolve().parents[1] / 'operations'
DOUBLE = r'''#!/usr/bin/env python3
import fcntl, json, os, pathlib, sys, time, uuid
name = pathlib.Path(sys.argv[0]).name
args = sys.argv[1:]
root = pathlib.Path(os.environ['TEST_ROOT'])
file = root / 'fixture.json'
data = json.loads(file.read_text())
def save(): file.write_text(json.dumps(data))
if name == 'id': print('0')
elif name == 'flock':
    try: fcntl.flock(int(args[-1]), fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError: sys.exit(1)
elif name == 'cat':
    if args == ['/proc/sys/kernel/random/uuid']: print(uuid.uuid4())
    else: os.execv('/bin/cat', ['cat'] + args)
elif name == 'curl': print('{"status":"ok"}')
elif name == 'aws':
    data['upload_attempts'] += 1
    if data['upload_attempts'] == data.get('fail_upload_at'):
        save(); sys.exit(1)
    source = pathlib.Path(args[args.index('cp') + 1])
    if source.name == 'release.txt': data['manifests'].append(source.read_text())
    save()
elif name == 'docker':
    if args[:2] == ['image', 'inspect']:
        print(data.get('image_override', 'sha256:' + args[-1])); sys.exit(0)
    assert args[0] == 'compose', args
    args = args[args.index('-f') + 2:]
    if args[:1] == ['config']: pass
    elif args[:1] == ['up']:
        if 'api' in args: data['public'] = True; save()
    elif args[:1] == ['stop']: data['public'] = False; save()
    elif args[:1] == ['run']:
        assert args[-1] == 'migrate'
        data['schema'] = 1
        data['migrations'] += 1
        failing = data.pop('fail_migration', False)
        save()
        if failing: sys.exit(1)
    elif args[:3] == ['exec', '-T', 'db']:
        command = args[3:]
        if command[0] == 'psql':
            if '-c' in command: print(data['schema'])
            else: sys.stdin.read()
        elif command[0] == 'pg_dump':
            if (root / 'pause-dump').exists():
                (root / 'dump-paused').touch()
                while not (root / 'resume-dump').exists(): time.sleep(0.01)
            print('synthetic consistent dump')
        elif command[0] == 'pg_restore': sys.stdin.read()
        else: raise AssertionError(command)
    else: raise AssertionError(args)
else: raise AssertionError(name)
'''


class ReleaseWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        for name in ('common.sh', 'release-state.sh', 'deploy.sh', 'backup.sh', 'grant-runtime.sql', 'restore-cutover.sh'):
            shutil.copy(OPERATIONS / name, self.root / name)
        (self.root / 'secrets').mkdir()
        (self.root / 'secrets/openai_key').touch()
        (self.root / 'bin').mkdir()
        for name in ('docker', 'aws', 'id', 'curl', 'cat', 'flock'):
            tool = self.root / 'bin' / name
            tool.write_text(DOUBLE)
            tool.chmod(0o755)
        self.fixture = self.root / 'fixture.json'
        self.fixture.write_text(json.dumps(dict(schema=0, migrations=0, upload_attempts=0, manifests=[], public=False)))
        self.env = dict(os.environ, TEST_ROOT=str(self.root), PATH=str(self.root / 'bin') + ':' + os.environ['PATH'])
        self.configure('release1')

    def configure(self, image):
        (self.root / 'deploy.env').write_text(
            f'API_IMAGE={image}\nPOSTGRES_IMAGE=postgres@sha256:fixture\nCADDY_IMAGE=caddy@sha256:fixture\n'
            'DOMAIN=api.arran-test.net\nBACKUP_BUCKET=fixture\nAWS_PROFILE=fixture\n')

    def data(self):
        return json.loads(self.fixture.read_text())

    def change(self, **values):
        data = self.data()
        data.update(values)
        self.fixture.write_text(json.dumps(data))

    def run_script(self, script, *args, success=True):
        result = subprocess.run(['bash', str(self.root / script), *args], env=self.env,
                                capture_output=True, text=True, timeout=20)
        self.assertEqual(result.returncode == 0, success, result.stdout + result.stderr)
        return result

    def test_restore_cutover_rejects_unverified_evidence_and_blocks_deploy(self):
        self.run_script('deploy.sh', 'initial')
        for argument in ('', 'missing', 'invalid', 'old-head', 'verified'):
            args = (argument,) if argument else ()
            self.run_script('restore-cutover.sh', *args, success=False)
            self.assertTrue((self.root / 'state/restore-cutover-blocked').exists())
            self.run_script('deploy.sh', 'update', success=False)
            self.assertTrue(self.data()['public'])  # existing process is not stopped

    def test_restore_request_during_deploy_is_not_accepted_and_can_retry(self):
        self.run_script('deploy.sh', 'initial')
        (self.root / 'pause-dump').touch()
        proc = subprocess.Popen(['bash', str(self.root / 'deploy.sh'), 'update'], env=self.env,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        import time
        deadline = time.monotonic() + 10
        while not (self.root / 'dump-paused').exists() and time.monotonic() < deadline:
            time.sleep(0.01)
        try:
            self.assertTrue((self.root / 'dump-paused').exists())
            result = self.run_script('restore-cutover.sh', success=False)
            self.assertIn('not recorded', result.stderr)
            self.assertFalse((self.root / 'state/restore-cutover-blocked').exists())
        finally:
            (self.root / 'resume-dump').touch()
            out, err = proc.communicate(timeout=20)
        self.assertEqual(proc.returncode, 0, out + err)
        self.assertTrue(self.data()['public'])
        self.run_script('restore-cutover.sh', success=False)
        self.assertTrue((self.root / 'state/restore-cutover-blocked').exists())
        self.run_script('deploy.sh', 'update', success=False)

    def test_restore_request_is_recorded_without_deploy_env(self):
        (self.root / 'deploy.env').unlink()
        self.run_script('restore-cutover.sh', success=False)
        self.assertTrue((self.root / 'state/restore-cutover-blocked').exists())
        self.configure('release1')
        self.run_script('deploy.sh', 'initial', success=False)

    def test_initial_backup_failure_resumes_without_old_api_or_secret_changes(self):
        self.change(fail_upload_at=1)
        secret_before = (self.root / 'secrets/openai_key').read_bytes()
        self.run_script('deploy.sh', 'initial', success=False)
        self.assertFalse(self.data()['public'])
        self.assertTrue((self.root / 'state/pending-release').exists())
        self.assertIn('API_IMAGE=release1\n', (self.root / 'state/applied-release').read_text())
        self.run_script('deploy.sh', 'initial')
        self.assertTrue(self.data()['public'])
        self.assertEqual(self.data()['migrations'], 2)
        self.assertFalse((self.root / 'state/pending-release').exists())
        self.assertEqual((self.root / 'secrets/openai_key').read_bytes(), secret_before)

    def test_pending_release_rejects_changed_target_or_retagged_image(self):
        self.change(fail_upload_at=1)
        self.run_script('deploy.sh', 'initial', success=False)
        self.configure('release2')
        self.run_script('deploy.sh', 'initial', success=False)
        self.configure('release1')
        self.change(image_override='sha256:different')
        self.run_script('deploy.sh', 'initial', success=False)
        self.assertEqual(self.data()['migrations'], 1)
        self.assertFalse(self.data()['public'])

    def test_daily_backup_uses_applied_not_planned_image(self):
        self.run_script('deploy.sh', 'initial')
        self.configure('release2')
        self.run_script('backup.sh')
        self.assertIn('API_IMAGE=release1\n', self.data()['manifests'][-1])
        self.assertNotIn('release2', self.data()['manifests'][-1])

    def test_update_post_migration_backup_failure_preserves_new_applied_release(self):
        self.run_script('deploy.sh', 'initial')
        self.configure('release2')
        self.change(fail_upload_at=7)  # initial three + pre-update three; fail post-migrate upload
        self.run_script('deploy.sh', 'update', success=False)
        self.assertFalse(self.data()['public'])
        self.assertIn('API_IMAGE=release2\n', (self.root / 'state/applied-release').read_text())
        self.assertIn('API_IMAGE=release1\n', self.data()['manifests'][-1])
        self.run_script('backup.sh', success=False)  # pending deployment is not a completed release
        self.run_script('deploy.sh', 'update')
        self.assertIn('API_IMAGE=release2\n', self.data()['manifests'][-1])
        self.assertTrue(self.data()['public'])

    def test_failed_migration_blocks_daily_backup_and_can_resume_same_target(self):
        self.run_script('deploy.sh', 'initial')
        self.configure('release2')
        self.change(fail_migration=True)
        self.run_script('deploy.sh', 'update', success=False)
        self.assertIn('API_IMAGE=release1\n', (self.root / 'state/applied-release').read_text())
        self.run_script('backup.sh', success=False)
        self.run_script('deploy.sh', 'update')
        self.assertIn('API_IMAGE=release2\n', self.data()['manifests'][-1])

    def test_existing_untracked_schema_and_concurrent_release_fail_closed(self):
        self.change(schema=1)
        self.run_script('deploy.sh', 'initial', success=False)
        self.run_script('deploy.sh', 'update', success=False)
        self.assertEqual(self.data()['migrations'], 0)
        lock = self.root / 'state/release.lock'
        with lock.open('w') as held:
            fcntl.flock(held, fcntl.LOCK_EX)
            self.run_script('deploy.sh', 'initial', success=False)
            self.run_script('backup.sh', success=False)
        self.assertEqual(self.data()['upload_attempts'], 0)


if __name__ == '__main__':
    unittest.main()
