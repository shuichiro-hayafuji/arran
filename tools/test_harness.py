"""Checks for false success, side effects and module coverage in the runner."""

import contextlib
import io
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import harness


class HarnessTests(unittest.TestCase):
    def setUp(self):
        output = contextlib.redirect_stdout(io.StringIO())
        output.__enter__()
        self.addCleanup(output.__exit__, None, None, None)

    def test_exit_status_preserves_failure_and_blocked(self):
        self.assertEqual(harness.exit_code(["PASS"]), 0)
        self.assertEqual(harness.exit_code(["PASS", "BLOCKED"]), 2)
        self.assertEqual(harness.exit_code(["FAIL", "BLOCKED"]), 1)

    def test_broken_document_link_fails(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "AGENTS.md").write_text("[bad](missing.md)")
            self.assertEqual(harness.check_docs(root), "FAIL")

    def test_missing_executable_is_blocked(self):
        with patch("harness.shutil.which", return_value=None):
            self.assertEqual(harness.run_command(Path.cwd(), ["missing"], {}), "BLOCKED")

    def test_nonzero_command_fails(self):
        with patch("harness.shutil.which", return_value="tool"), \
                patch("harness.subprocess.run", return_value=SimpleNamespace(returncode=1)):
            self.assertEqual(harness.run_command(Path.cwd(), ["tool"], {}), "FAIL")

    def test_gofmt_output_fails_without_writing(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "server/internal").mkdir(parents=True)
            source = root / "server/internal/example.go"
            source.write_text("package example\n")
            result = SimpleNamespace(returncode=0, stdout=str(source), stderr="")
            with patch("harness.shutil.which", return_value="gofmt"), \
                    patch("harness.subprocess.run", return_value=result) as run:
                self.assertEqual(harness.check_go_format(root), "FAIL")
                self.assertEqual(run.call_args.args[0][:2], ["gofmt", "-l"])
                self.assertEqual(source.read_text(), "package example\n")

    def test_server_includes_agent_and_clears_database(self):
        with patch("harness.run_command", return_value="PASS") as run, \
                patch("harness.check_go_format", return_value="PASS"), \
                patch.dict("os.environ", {"ARRAN_TEST_DATABASE_URL": "test-only"}):
            self.assertEqual(harness.main(["check", "server"]), 0)
            self.assertEqual(run.call_count, 4)
            self.assertTrue(any(call.args[0].name == "agent" for call in run.call_args_list))
            self.assertTrue(all("ARRAN_TEST_DATABASE_URL" not in call.args[2]
                                for call in run.call_args_list))

    def test_dry_run_never_executes(self):
        with patch("harness.subprocess.run") as run, patch("harness.check_docs") as docs:
            self.assertEqual(harness.main(["check", "all", "--dry-run"]), 0)
            run.assert_not_called()
            docs.assert_not_called()

    def test_postgres_missing_configuration_is_not_success(self):
        with patch.dict("os.environ", {}, clear=True), patch("harness.run_command") as run:
            self.assertEqual(harness.main(["check", "postgres"]), 2)
            run.assert_not_called()


if __name__ == "__main__":
    unittest.main()
