"""Regression tests for bypassing the shared HTTP providers."""

import contextlib
import io
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import harness
from mobile_architecture import CLIENT, check_mobile_architecture


class MobileArchitectureTests(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.root = Path(directory.name)
        self.write(CLIENT, 'final client = Dio();')

    def write(self, name, text):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)

    def check(self):
        with contextlib.redirect_stdout(io.StringIO()) as output:
            result = check_mobile_architecture(self.root)
        return result, output.getvalue()

    def test_independent_clients_and_aliases_fail_with_location(self):
        path = 'mobile/lib/features/session/provider/session_provider.dart'
        for expression in ('Dio()', 'http.Dio()', 'Dio.new', 'Dio client',
                           'ApiClient(dio)', 'api.ApiClient.init(ref)',
                           'ApiClient.new', 'Dio /* comment */ ()'):
            with self.subTest(expression=expression):
                self.write(path, '// context\nfinal client = ' + expression + ';')
                result, output = self.check()
                self.assertEqual(result, 'FAIL')
                self.assertIn(path + ':2: A-M07', output)

    def test_injection_options_comments_and_test_doubles_are_allowed(self):
        self.write('mobile/lib/features/session/provider/session_provider.dart', '''
// Dio() and ApiClient.init(ref) are not executable here.
/* Dio() */
final description = "Dio()";
final raw = r'Dio()';
final ApiClient client = ref.watch(publicApiClientProvider);
final options = Options(headers: {});
final data = FormData();
''')
        self.write('mobile/test/session_test.dart', 'final dio = Dio();')
        self.write('mobile/packages/example/lib/test.dart', 'final dio = Dio();')
        self.assertEqual(self.check()[0], 'PASS')

    def test_missing_source_is_blocked(self):
        (self.root / CLIENT).unlink()
        self.assertEqual(self.check()[0], 'BLOCKED')

    def test_guard_failure_propagates_through_mobile_and_all(self):
        for scope in ('mobile', 'all', 'mobile-architecture'):
            with self.subTest(scope=scope), \
                    patch('harness.check_mobile_architecture', return_value='FAIL') as guard, \
                    patch('harness.check_docs', return_value='PASS'), \
                    patch('harness.check_go_format', return_value='PASS'), \
                    patch('harness.run_command', return_value='PASS'), \
                    contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(harness.main(['check', scope]), 1)
                guard.assert_called_once()

    def test_dry_run_does_not_read_sources(self):
        with patch('harness.check_mobile_architecture') as guard, \
                contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(harness.main(['check', 'mobile-architecture', '--dry-run']), 0)
            guard.assert_not_called()


if __name__ == '__main__':
    unittest.main()
