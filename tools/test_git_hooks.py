"""Exercise real Git hook dispatch without creating commits or touching the user's index."""

import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import git_hooks

ROOT = Path(__file__).resolve().parent.parent
SPEC = importlib.util.spec_from_file_location("codex_notify", ROOT / "scripts/codex-notify.py")
notify = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(notify)


class GitHookTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix="arran-hook-test-")
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        # Do not inherit alternate-index/worktree variables from an outer Git hook.
        self.env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
        self.env.update(GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_NOSYSTEM="1")
        self.git("init", "-q")
        for name in (".githooks/pre-commit", "tools/git_hooks.py", "tools/mobile_architecture.py",
                     "scripts/install-git-hooks.sh"):
            dest = self.root / name
            dest.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(ROOT / name, dest)
        self.git("config", "--local", "core.hooksPath", ".githooks")

    def git(self, *args):
        return subprocess.run(["git", *args], cwd=self.root, env=self.env,
                              capture_output=True, check=True).stdout

    def stage(self, name, content):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)
        self.git("add", "--", name)
        return path

    def hook(self):
        return subprocess.run(["git", "hook", "run", "pre-commit"], cwd=self.root,
                              env=self.env, capture_output=True)

    def test_empty_index_passes_via_git(self):
        self.assertEqual(self.hook().returncode, 0)

    def test_go_format_restages_only_target_and_handles_spaces(self):
        path = self.stage("server/a space.go", "package a\nfunc f(){println(1)}\n")
        other = self.root / "server/unstaged.go"
        other.write_text("keep untouched")
        result = self.hook()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(b"func f()", path.read_bytes())
        self.assertEqual(path.read_bytes(), self.git("show", ":server/a space.go"))
        self.assertEqual(other.read_text(), "keep untouched")
        self.assertEqual(self.git("diff", "--cached", "--name-only"), b"server/a space.go\n")

    def test_editor_change_during_restage_is_never_added(self):
        path = self.stage("server/a.go", "package a\nfunc f(){}\n")
        real_git = git_hooks.git

        def edit_after_checks(root, *args, **kwargs):
            if args[0] == "hash-object":
                path.write_text(path.read_text() + "// late editor change\n")
            return real_git(root, *args, **kwargs)

        with patch.object(git_hooks, "git", side_effect=edit_after_checks):
            git_hooks.pre_commit(self.root)
        self.assertIn("late editor change", path.read_text())
        self.assertNotIn(b"late editor change", self.git("show", ":server/a.go"))
        self.assertIn(b"func f()", self.git("show", ":server/a.go"))

    def test_partial_stage_preserves_all_files_and_index(self):
        first = self.stage("server/a.go", "package a\nfunc f(){}\n")
        path = self.stage("server/b.go", "package a\nfunc b(){}\n")
        path.write_text("package a\nfunc b(){println(2)}\n")
        before = self.git("diff", "--cached")
        result = self.hook()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b"Partially staged", result.stderr)
        self.assertEqual(first.read_text(), "package a\nfunc f(){}\n")
        self.assertEqual(path.read_text(), "package a\nfunc b(){println(2)}\n")
        self.assertEqual(before, self.git("diff", "--cached"))

    def test_syntax_failure_does_not_apply_other_formatting(self):
        path = self.stage("server/a.go", "package a\nfunc f(){}\n")
        self.stage("scripts/bad.py", "def broken(\n")
        before = self.git("diff", "--cached")
        self.assertNotEqual(self.hook().returncode, 0)
        self.assertEqual(path.read_text(), "package a\nfunc f(){}\n")
        self.assertEqual(before, self.git("diff", "--cached"))

    def test_checks_index_not_unstaged_python(self):
        path = self.stage("scripts/good.py", "value = 1\n")
        path.write_text("def broken(\n")
        self.assertEqual(self.hook().returncode, 0)
        self.assertEqual(path.read_text(), "def broken(\n")

    def test_whitespace_failure_has_recovery_message(self):
        self.stage("note.md", "trailing  \n")
        result = self.hook()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b"trailing whitespace", result.stderr)
        self.assertIn(b"retry", result.stderr)

    def test_symlink_source_is_rejected_without_touching_target(self):
        target = self.root / "outside"
        target.write_text("keep")
        (self.root / "server").mkdir()
        (self.root / "server/link.go").symlink_to(target)
        self.git("add", "server/link.go")
        self.assertNotEqual(self.hook().returncode, 0)
        self.assertEqual(target.read_text(), "keep")

    def test_generated_receipt_is_bound_to_exact_mobile_snapshot(self):
        self.stage("mobile/test/example.freezed.dart", "// generated fixture\n")
        self.assertNotEqual(self.hook().returncode, 0)
        git_hooks.receipt_path(self.root).write_text(git_hooks.fingerprint(git_hooks.entries(self.root)))
        self.assertEqual(self.hook().returncode, 0)
        self.stage("mobile/test/example.freezed.dart", "// changed fixture\n")
        self.assertNotEqual(self.hook().returncode, 0)

    def test_install_remove_idempotence_and_existing_config(self):
        script = ["bash", "scripts/install-git-hooks.sh"]
        for args in (script, script, script + ["--uninstall"], script + ["--uninstall"]):
            result = subprocess.run(args, cwd=self.root, env=self.env, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
        self.git("config", "--local", "core.hooksPath", "other-hooks")
        result = subprocess.run(script, cwd=self.root, env=self.env, capture_output=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.git("config", "--local", "--get", "core.hooksPath"), b"other-hooks\n")

    def test_installed_dart_formats_staged_source(self):
        self.stage("mobile/.fvmrc", '{"flutter":"3.44.6"}\n')
        self.stage("mobile/pubspec.yaml", "name: sample\nenvironment:\n  sdk: ^3.12.2\n")
        path = self.stage("mobile/test/a space.dart", "void main(){print('ok');}\n")
        try:
            git_hooks.dart_sdk(self.root, git_hooks.entries(self.root))
        except git_hooks.HookError:
            self.skipTest("Pinned SDK not installed; no installation attempted")
        result = self.hook()
        # With no tracked generated files, formatting alone must succeed.
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(path.read_bytes(), self.git("show", ":mobile/test/a space.dart"))
        self.assertIn("void main() {", path.read_text())

    def generated_fixture(self):
        self.stage("mobile/lib/a.freezed.dart", "// generated fixture\n")
        self.stage("mobile/pubspec.yaml", "name: sample\n")
        self.stage("mobile/packages/build_runner/pubspec.yaml", "name: build_runner\n")
        self.stage("mobile/packages/build_runner/bin/build_runner.dart", "// test runner\n")
        config = self.root / "mobile/.dart_tool/package_config.json"
        config.parent.mkdir()
        config.write_text(json.dumps({"packages": [
            {"name": "build_runner", "rootUri": "../packages/build_runner"},
            {"name": "sample", "rootUri": "../"},
        ]}))

    def test_generated_comparison_detects_drift_without_writing_worktree(self):
        self.generated_fixture()
        before = self.git("diff", "--cached")
        real_run = git_hooks.run

        def fake_generator(args, cwd, **kwargs):
            if args[0] == "/test-dart":
                (cwd / "lib/a.freezed.dart").write_text("// regenerated drift\n")
                return b""
            return real_run(args, cwd, **kwargs)

        with patch.object(git_hooks, "dart_sdk", return_value="/test-dart"), \
                patch.object(git_hooks, "run", side_effect=fake_generator):
            with self.assertRaisesRegex(git_hooks.HookError, "Stale generated"):
                git_hooks.verify_generated(self.root)
        self.assertEqual(before, self.git("diff", "--cached"))
        self.assertEqual((self.root / "mobile/lib/a.freezed.dart").read_text(), "// generated fixture\n")
        self.assertFalse(git_hooks.receipt_path(self.root).exists())

    def test_generated_success_receipt_requires_matching_output(self):
        self.generated_fixture()
        real_run = git_hooks.run

        def fake_generator(args, cwd, **kwargs):
            return b"" if args[0] == "/test-dart" else real_run(args, cwd, **kwargs)

        with patch.object(git_hooks, "dart_sdk", return_value="/test-dart"), \
                patch.object(git_hooks, "run", side_effect=fake_generator):
            git_hooks.verify_generated(self.root)
        self.assertEqual(git_hooks.receipt_path(self.root).read_text().strip(),
                         git_hooks.fingerprint(git_hooks.entries(self.root)))

    def test_mobile_architecture_violation_is_rejected(self):
        self.stage("mobile/lib/core/network/api_client.dart", "// authorized boundary\n")
        self.stage("mobile/lib/bad.freezed.dart", "final client = Dio();\n")
        # The formatter is unrelated to this guard; leave the real Git/index path intact.
        with patch.object(git_hooks, "format_targets", return_value=[]):
            with self.assertRaisesRegex(git_hooks.HookError, "A-M07"):
                git_hooks.pre_commit(self.root)

    def test_missing_eslint_reports_manual_recovery(self):
        self.stage("infra/main.ts", "const value = 1;\n")
        result = self.hook()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b"ESLint is unavailable", result.stderr)
        self.assertFalse((self.root / "infra/node_modules").exists())

    def test_codex_installer_is_local_and_refuses_overwrite(self):
        for name in ("scripts/install-codex-hooks.sh", "scripts/codex-hooks.json"):
            shutil.copy2(ROOT / name, self.root / name)
        args = ["bash", "scripts/install-codex-hooks.sh"]
        for _ in range(2):
            result = subprocess.run(args, cwd=self.root, env=self.env, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
        path = self.root / ".codex/hooks.json"
        self.assertEqual(json.loads(path.read_text())["hooks"].keys(), {"Stop"})
        path.write_text('{"hooks": {}}')
        result = subprocess.run(args, cwd=self.root, env=self.env, capture_output=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(path.read_text(), '{"hooks": {}}')


class NotificationTests(unittest.TestCase):
    def test_only_stop_notifies_and_output_is_not_interpolated(self):
        with patch.object(notify.sys, "platform", "darwin"), patch.object(notify.subprocess, "run") as run:
            self.assertEqual(notify.notify({"hook_event_name": "PostToolUse"}), {})
            run.assert_not_called()
            self.assertEqual(notify.notify({"hook_event_name": "Stop", "last_assistant_message": "$(bad)"}), {})
            self.assertEqual(run.call_count, 1)
            self.assertNotIn("$(bad)", str(run.call_args))

    def test_notification_failure_does_not_continue_or_block_turn(self):
        with patch.object(notify.sys, "platform", "darwin"), \
                patch.object(notify.subprocess, "run", side_effect=OSError("unavailable")):
            output = notify.notify({"hook_event_name": "Stop"})
            self.assertIn("systemMessage", output)
            self.assertNotIn("decision", output)
            json.dumps(output)


if __name__ == "__main__":
    unittest.main()
