import { execFileSync } from 'node:child_process';
import * as path from 'node:path';

test('release scripts preserve checkpoints and database backup provenance across failures', () => {
  execFileSync('python3', [path.join(__dirname, 'release_workflow_test.py')], { stdio: 'pipe' });
}, 60000);
