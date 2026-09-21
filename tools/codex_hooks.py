"""Codex turn-scoped working-file formatting. Never modify the Git index."""

import hashlib
import json
import os
from pathlib import Path
import sys
import tempfile

from git_hooks import HookError, changed, copy_index, entries, format_files, format_targets, git, run


def regular(root, name):
    path = root / name
    return path.is_file() and not any((root.joinpath(*Path(name).parts[:i])).is_symlink()
                                     for i in range(1, len(Path(name).parts) + 1))


def working_files(root):
    names = git(root, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
    return {name: (root / name).read_bytes()
            for name in format_targets({os.fsdecode(p) for p in names.split(b"\0") if p})
            if regular(root, name)}


def digest(content):
    return hashlib.sha256(content).hexdigest()


def state_path(root, payload):
    ids = [payload.get("session_id"), payload.get("turn_id")]
    if not all(isinstance(value, str) and value for value in ids):
        raise HookError("Missing session_id/turn_id; formatting skipped. Start a new turn with trusted hooks.")
    key = digest(json.dumps(ids).encode())
    directory = root / os.fsdecode(git(root, "rev-parse", "--git-path", "arran-codex-format").strip())
    directory.mkdir(parents=True, exist_ok=True)
    return directory / (key + ".json")


def capture(root, payload):
    path = state_path(root, payload)
    # Never replace the baseline if the same event is delivered again.
    try:
        with path.open("x") as stream:
            json.dump({p: digest(data) for p, data in working_files(root).items()}, stream)
    except FileExistsError:
        pass


def finish(root, payload):
    path = state_path(root, payload)
    if not path.exists():
        raise HookError("No turn baseline; formatting skipped. Trust both hooks and start a new turn.")
    before = json.loads(path.read_text())
    index = entries(root)
    staged = set(changed(root))
    current = working_files(root)
    candidates = {p: data for p, data in current.items() if before.get(p) != digest(data)}
    selected = {p: data for p, data in candidates.items() if p not in staged}
    with tempfile.TemporaryDirectory(prefix="arran-codex-format-") as directory:
        temp = Path(directory)
        for name, data in selected.items():
            target = temp / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(data)
        if any(p.endswith(".dart") for p in selected):
            copy_index(root, index, temp, ["mobile/pubspec.yaml"])
        format_files(root, index, temp, selected)
        if entries(root) != index or set(changed(root)) != staged:
            raise HookError("Index changed during formatting; nothing applied. Retry formatting manually.")
        for name, data in selected.items():
            if not regular(root, name) or (root / name).read_bytes() != data:
                raise HookError(f"Concurrent edit: {name}; nothing applied. Save edits and format manually.")
        for name, data in selected.items():
            formatted = (temp / name).read_bytes()
            if formatted != data:
                (root / name).write_bytes(formatted)
    path.unlink()
    skipped = sorted(candidates.keys() & staged)
    return ("Codex formatting skipped staged files: " + ", ".join(skipped) +
            ". The Git pre-commit hook handles them; no staging was performed.") if skipped else ""


def handle(root, payload):
    event = payload.get("hook_event_name")
    warnings = []
    try:
        if event == "UserPromptSubmit":
            capture(root, payload)
        elif event == "Stop":
            warning = finish(root, payload)
            if warning:
                warnings.append(warning)
    except (HookError, OSError, ValueError, KeyError) as error:
        warnings.append(f"Arran formatting: {error} See tools/HOOKS.md. No automatic staging occurred.")
    # Keep the existing notification, but run it after the formatter, even on failure.
    if event == "Stop":
        try:
            result = json.loads(run([sys.executable, str(root / "scripts/codex-notify.py")], root,
                                    data=json.dumps(payload).encode(), timeout=10))
            if result.get("systemMessage"):
                warnings.append(result["systemMessage"])
        except (HookError, ValueError) as error:
            warnings.append(f"Arran notification: {error}")
    return {"systemMessage": "\n".join(warnings)} if warnings else {}


def main():
    try:
        payload = json.load(sys.stdin)
        if not isinstance(payload, dict):
            raise ValueError("Expected a JSON object")
        root = Path(os.fsdecode(git(Path.cwd(), "rev-parse", "--show-toplevel")).strip()).resolve()
        output = handle(root, payload)
    except (HookError, ValueError, OSError) as error:
        output = {"systemMessage": f"Arran hook error: {error}. See tools/HOOKS.md."}
    # Never ask Codex to continue the turn because formatting or notification failed.
    print(json.dumps(output, ensure_ascii=False))


if __name__ == "__main__":
    main()
