"""Repository-only staged checks. Never install dependencies or run full tests."""

import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
from urllib.parse import unquote, urlparse

from mobile_architecture import check_mobile_architecture


class HookError(Exception):
    pass


def run(args, cwd, *, data=None, timeout=30):
    try:
        result = subprocess.run(args, cwd=cwd, input=data, capture_output=True,
                                timeout=timeout, check=False)
    except (OSError, subprocess.TimeoutExpired) as error:
        raise HookError(f"Cannot run {args[0]}: {error}. Check the installed tool and retry.") from error
    if result.returncode:
        detail = (result.stdout + result.stderr).decode(errors="replace")
        raise HookError(f"{' '.join(map(str, args))}\n{detail}\nFix the error and retry.")
    return result.stdout


def git(root, *args, **kwargs):
    return run(["git", *args], root, **kwargs)


def entries(root):
    result = {}
    for record in git(root, "ls-files", "--stage", "-z").split(b"\0"):
        if not record:
            continue
        metadata, name = record.split(b"\t", 1)
        mode, oid, stage = metadata.decode().split()
        if stage != "0":
            raise HookError("Unmerged index. Resolve conflicts and stage the resolution first.")
        result[os.fsdecode(name)] = (mode, oid)
    return result


def changed(root):
    return [os.fsdecode(p) for p in git(root, "diff", "--cached", "--name-only",
                                       "--no-renames", "-z").split(b"\0") if p]


def blob(root, entry):
    return git(root, "cat-file", "blob", entry[1])


def generated(path):
    return path.endswith((".freezed.dart", ".g.dart"))


def mobile_input(path):
    # Conservative: imported types and local packages can affect Freezed output too.
    return path.startswith("mobile/") and (
        path.endswith((".dart", ".yaml", ".lock")) or path == "mobile/.fvmrc")


def uses_codegen(root, index):
    return any(generated(p) for p in index) or (
        "mobile/pubspec.yaml" in index
        and b"build_runner:" in blob(root, index["mobile/pubspec.yaml"]))


def fingerprint(index):
    records = [(p, *value) for p, value in sorted(index.items()) if mobile_input(p)]
    return hashlib.sha256(json.dumps(records).encode()).hexdigest()


def receipt_path(root):
    return root / os.fsdecode(git(root, "rev-parse", "--git-path", "arran-generated-check").strip())


def copy_index(root, index, destination, paths):
    for name in paths:
        if name not in index:
            continue
        if index[name][0] not in ("100644", "100755"):
            raise HookError(f"Unsupported file type: {name}. Check this file manually; no files changed.")
        path = destination / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(blob(root, index[name]))


def dart_sdk(root, index):
    config = json.loads(blob(root, index["mobile/.fvmrc"]))
    version = config["flutter"]
    if not version or "/" in version or version in ("stable", "beta", "master"):
        raise HookError("mobile/.fvmrc must pin an installed Flutter version.")
    cache = Path(os.environ.get("FVM_CACHE_PATH", str(Path.home() / "fvm/versions")))
    candidates = [root / "mobile/.fvm/flutter_sdk", cache / version]
    for sdk in candidates:
        executable = sdk / "bin/cache/dart-sdk/bin/dart"
        # Do not invoke the Flutter/FVM bootstrap scripts: they can install dependencies.
        version_file = sdk / "bin/cache/flutter.version.json"
        if executable.is_file() and version_file.is_file():
            if json.loads(version_file.read_text()).get("flutterVersion") == version:
                return executable.resolve()
    raise HookError(f"Flutter {version} cached Dart SDK not found. Prepare it manually, then retry; "
                    "hooks never install SDKs or packages. See tools/HOOKS.md.")


def format_targets(paths):
    return [p for p in paths if (
        (p.startswith(("mobile/lib/", "mobile/test/")) and p.endswith(".dart") and not generated(p))
        or (p.startswith("server/") and not p.startswith("server/agent/") and p.endswith(".go")))]


def format_files(root, index, temp, targets):
    """Shared formatter invocation; callers own file selection and staging policy."""
    go = [str(temp / p) for p in targets if p.endswith(".go")]
    if go:
        run(["gofmt", "-w", *go], temp)
    dart = [str(temp / p) for p in targets if p.endswith(".dart")]
    if dart:
        run([str(dart_sdk(root, index)), "format", *dart], temp / "mobile")


def pre_commit(root):
    index = entries(root)
    changes = changed(root)
    existing = [p for p in changes if p in index and index[p][0] != "160000"]
    targets = format_targets(existing)
    originals = {}
    for name in targets:
        path = root / name
        if path.is_symlink() or not path.is_file() or index[name][0] not in ("100644", "100755"):
            raise HookError(f"Refusing missing/symlink source: {name}. Restore a regular file and retry.")
        originals[name] = path.read_bytes()
        if originals[name] != blob(root, index[name]):
            raise HookError(f"Partially staged file: {name}. Format manually and stage only the intended "
                            "changes before retrying. Index and working files were not changed.")
    # Whitespace/conflict markers are checked on the index, not unstaged edits.
    git(root, "diff", "--cached", "--check")
    with tempfile.TemporaryDirectory(prefix="arran-pre-commit-") as directory:
        temp = Path(directory)
        paths = {p for p in existing if p in targets or p.endswith((".py", ".json", ".sh"))
                 or p == ".githooks/pre-commit"}
        if any(p.startswith("mobile/lib/") for p in changes):
            paths.update(p for p in index if p.startswith("mobile/lib/") and p.endswith(".dart"))
        if any(p.endswith(".dart") for p in targets):
            paths.add("mobile/pubspec.yaml")
        infra = [p for p in existing if p.startswith("infra/") and p.endswith((".ts", ".js", ".cjs", ".mjs"))]
        if infra:
            paths.update(infra)
            paths.update(p for p in index if p.startswith("infra/") and p.endswith((".json", ".ts", ".cjs")))
        copy_index(root, index, temp, paths)
        format_files(root, index, temp, targets)
        for name in existing:
            path = temp / name
            if name.endswith(".py"):
                compile(path.read_bytes(), name, "exec")
            elif name.endswith(".json"):
                json.loads(path.read_text())
            elif name.endswith(".sh") or name == ".githooks/pre-commit":
                run(["bash", "-n", str(path)], temp)
        if any(p.startswith("mobile/lib/") for p in changes):
            if check_mobile_architecture(temp) != "PASS":
                raise HookError("A-M07 failed. Use the shared API client; see tools/CHECKS.md and retry.")
        if infra:
            deps = root / "infra/node_modules"
            if not (deps / ".bin/eslint").is_file():
                raise HookError("infra ESLint is unavailable. Prepare existing infra dependencies manually "
                                "and retry; no dependencies were installed.")
            (temp / "infra/node_modules").symlink_to(deps, target_is_directory=True)
            run([str(deps / ".bin/eslint"), *["./" + p[6:] for p in infra]], temp / "infra")
        updates = {p: (temp / p).read_bytes() for p in targets
                   if (temp / p).read_bytes() != originals[p]}
        # Check again after external tools, before any write or re-stage.
        if entries(root) != index or changed(root) != changes:
            raise HookError("Index changed while checks ran. Retry after other Git operations finish.")
        for name in targets:
            path = root / name
            if path.is_symlink() or path.read_bytes() != originals[name]:
                raise HookError(f"Working file changed during checks: {name}. Retry after saving edits.")
        for name, content in updates.items():
            (root / name).write_bytes(content)
        if updates:
            # Stage the bytes we formatted, never a new editor change read by `git add`.
            records = b""
            for name, content in updates.items():
                oid = git(root, "hash-object", "-w", "--stdin", data=content).strip()
                records += index[name][0].encode() + b" " + oid + b"\t" + os.fsencode(name) + b"\0"
            git(root, "update-index", "-z", "--index-info", data=records)
            print("[PASS] Formatted and re-staged: " + ", ".join(updates), flush=True)
        current = entries(root)
        if (uses_codegen(root, current) or any(generated(p) for p in changes)) and any(mobile_input(p) for p in changes):
            receipt = receipt_path(root)
            if not receipt.is_file() or receipt.read_text().strip() != fingerprint(current):
                raise HookError("Generated files are not verified for this staged mobile snapshot. "
                                "Run python3 tools/git_hooks.py verify-generated, then retry. "
                                "If stale, run the existing derry generate explicitly, review and stage its output.")
    print("[PASS] Staged lightweight checks (full tests/builds were not run).")


def verify_generated(root):
    index = entries(root)
    # A failed re-check must not leave an older success receipt usable.
    receipt_path(root).unlink(missing_ok=True)
    paths = [p for p in index if mobile_input(p)]
    expected = [p for p in paths if p.startswith("mobile/lib/") and generated(p)]
    if not expected and not uses_codegen(root, index):
        print("[NOT APPLICABLE] No tracked Freezed/generated Dart files.")
        return
    sdk = dart_sdk(root, index)
    package_file = root / "mobile/.dart_tool/package_config.json"
    if not package_file.is_file():
        raise HookError("Existing mobile package_config.json missing. Prepare dependencies manually first.")
    # A worktree package map is usable only for the exact staged dependency configuration.
    for name in paths:
        if name.endswith(("pubspec.yaml", "pubspec.lock")):
            if (root / name).read_bytes() != blob(root, index[name]):
                raise HookError(f"Dependency config differs from index: {name}. Align dependencies manually first.")
    with tempfile.TemporaryDirectory(prefix="arran-generated-") as directory:
        temp = Path(directory)
        copy_index(root, index, temp, paths)
        package_config = json.loads(package_file.read_text())
        runner = None
        for package in package_config["packages"]:
            uri = urlparse(package["rootUri"])
            if uri.scheme not in ("", "file"):
                raise HookError("Only existing local package paths are supported; prepare dependencies manually.")
            source = Path(unquote(uri.path))
            if not source.is_absolute():
                source = (package_file.parent / source).resolve()
            try:
                source = temp / source.relative_to(root)
            except ValueError:
                pass
            if not (source / "pubspec.yaml").is_file():
                raise HookError(f"Package not available: {package['name']}. Prepare dependencies manually.")
            package["rootUri"] = source.as_uri()
            if package["name"] == "build_runner":
                runner = source / "bin/build_runner.dart"
        if runner is None or not runner.is_file():
            raise HookError("Cached build_runner missing. Prepare dependencies manually first.")
        config = temp / "mobile/.dart_tool/package_config.json"
        config.parent.mkdir(parents=True)
        config.write_text(json.dumps(package_config))
        before = {p: (temp / p).read_bytes() for p in expected}
        # Direct Dart execution avoids `dart run` performing implicit pub resolution.
        run([str(sdk), f"--packages={config}", str(runner), "build", "--delete-conflicting-outputs"],
            temp / "mobile", timeout=180)
        actual = {p.relative_to(temp).as_posix(): p.read_bytes()
                  for p in (temp / "mobile/lib").rglob("*.dart") if generated(str(p))}
        if before != actual:
            names = sorted(p for p in before.keys() | actual.keys() if before.get(p) != actual.get(p))
            raise HookError("Stale generated files: " + ", ".join(names) +
                            "\nRun cd mobile && fvm dart run derry generate explicitly after protecting "
                            "your edits, review and stage the results, then repeat verify-generated. "
                            "Verification did not change your working files or index.")
    if entries(root) != index:
        raise HookError("Index changed during generation. Re-run verify-generated.")
    receipt_path(root).write_text(fingerprint(index) + "\n")
    print("[PASS] Staged generated files match build_runner; repository-local verification receipt saved.")


def main():
    try:
        root = Path(os.fsdecode(run(["git", "rev-parse", "--show-toplevel"], Path.cwd())).strip())
        if sys.argv[1:] == ["pre-commit"]:
            pre_commit(root)
        elif sys.argv[1:] == ["verify-generated"]:
            verify_generated(root)
        else:
            raise HookError("Usage: python3 tools/git_hooks.py {pre-commit|verify-generated}")
    except (HookError, ValueError, KeyError, SyntaxError, OSError) as error:
        print(f"[FAIL] {error}\nSee tools/HOOKS.md for fixes and retry instructions.", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
