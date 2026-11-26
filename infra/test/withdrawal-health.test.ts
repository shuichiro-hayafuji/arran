import { execFileSync } from 'child_process';
import * as path from 'path';

test('withdrawal health only refreshes after a healthy scheduler and completed batch', () => {
  execFileSync('python3', [path.join(__dirname, 'withdrawal_health_test.py')], { stdio: 'pipe' });
});
