"""Codex formatting preserves index, unrelated work and failures."""

import json
from pathlib import Path
import shutil
import subprocess
import unittest
from unittest.mock import patch

import codex_hooks
import test_git_hooks


class CodexFormatTests(unittest.TestCase):
    setUp = test_git_hooks.GitHookTests.setUp
    git = test_git_hooks.GitHookTests.git
    stage = test_git_hooks.GitHookTests.stage
    payload = {"session_id": "session", "turn_id": "turn"}

    def source(self, name="server/a.go", content="package a\nfunc f(){}\n"):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)
        return path

    def test_changed_file_formatted_unrelated_and_index_preserved(self):
        source = self.source()
        other = self.source("server/other.go")
        codex_hooks.capture(self.root, self.payload)
        source.write_text("package a\nfunc f(){println(1)}\n")
        before = self.git("ls-files", "--stage")
        codex_hooks.finish(self.root, self.payload)
        self.assertIn("func f()", source.read_text())
        self.assertEqual(other.read_text(), "package a\nfunc f(){}\n")
        self.assertEqual(before, self.git("ls-files", "--stage"))

    def test_new_file_is_formatted(self):
        codex_hooks.capture(self.root, self.payload)
        source = self.source("server/a space.go")
        codex_hooks.finish(self.root, self.payload)
        self.assertIn("func f()", source.read_text())
        self.assertEqual(self.git("ls-files", "--stage"), b"")

    def test_partial_stage_is_skipped(self):
        source = self.stage("server/a.go", "package a\nfunc f(){}\n")
        codex_hooks.capture(self.root, self.payload)
        source.write_text("package a\nfunc f(){println(1)}\n")
        before = source.read_bytes()
        index = self.git("ls-files", "--stage")
        warning = codex_hooks.finish(self.root, self.payload)
        self.assertIn("skipped staged", warning)
        self.assertEqual(source.read_bytes(), before)
        self.assertEqual(index, self.git("ls-files", "--stage"))

    def test_bad_syntax_does_not_apply_other_formatting(self):
        codex_hooks.capture(self.root, self.payload)
        good = self.source()
        self.source("server/b.go", "not valid go\n")
        before = good.read_bytes()
        with self.assertRaises(codex_hooks.HookError):
            codex_hooks.finish(self.root, self.payload)
        self.assertEqual(good.read_bytes(), before)

    def test_missing_baseline_never_formats_existing_work(self):
        source = self.source()
        with self.assertRaisesRegex(codex_hooks.HookError, "No turn baseline"):
            codex_hooks.finish(self.root, self.payload)
        self.assertEqual(source.read_text(), "package a\nfunc f(){}\n")

    def test_symlinks_and_generated_files_are_excluded(self):
        codex_hooks.capture(self.root, self.payload)
        source = self.source()
        link = self.root / "server/link.go"
        link.symlink_to(source)
        generated = self.source("mobile/lib/a.freezed.dart", "// untouched\n")
        self.assertNotIn("server/link.go", codex_hooks.working_files(self.root))
        codex_hooks.finish(self.root, self.payload)
        self.assertEqual(generated.read_text(), "// untouched\n")

    def test_concurrent_edit_is_preserved(self):
        codex_hooks.capture(self.root, self.payload)
        source = self.source()
        with patch.object(codex_hooks, "format_files", side_effect=lambda *args: source.write_text("late edit")):
            with self.assertRaisesRegex(codex_hooks.HookError, "Concurrent edit"):
                codex_hooks.finish(self.root, self.payload)
        self.assertEqual(source.read_text(), "late edit")

    def test_event_command_returns_json_and_notifies_after_formatting(self):
        for name in ("tools/codex_hooks.py", "scripts/codex-notify.py"):
            shutil.copy2(test_git_hooks.ROOT / name, self.root / name)
        command = ["python3", "tools/codex_hooks.py"]
        payload = dict(self.payload, hook_event_name="UserPromptSubmit")
        first = subprocess.run(command, input=json.dumps(payload), text=True, capture_output=True,
                               cwd=self.root, env=self.env, check=True)
        self.assertEqual(json.loads(first.stdout), {})
        source = self.source()
        # Stub only the OS notification, while invoking the real hook command.
        (self.root / "scripts/codex-notify.py").write_text('print("{}")\n')
        payload["hook_event_name"] = "Stop"
        last = subprocess.run(command, input=json.dumps(payload), text=True, capture_output=True,
                              cwd=self.root, env=self.env, check=True)
        self.assertEqual(json.loads(last.stdout), {})
        self.assertIn("func f()", source.read_text())
        order = []
        with patch.object(codex_hooks, "finish", side_effect=lambda *args: order.append("format")), \
                patch.object(codex_hooks, "run", side_effect=lambda *args, **kwargs: order.append("notify") or b"{}"):
            self.assertEqual(codex_hooks.handle(self.root, payload), {})
        self.assertEqual(order, ["format", "notify"])


if __name__ == "__main__":
    unittest.main()
