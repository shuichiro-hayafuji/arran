#!/usr/bin/env python3
"""Arran's explicit, non-formatting check entry point (Python stdlib only)."""

import argparse
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

from mobile_architecture import check_mobile_architecture
from server_architecture import check_server_architecture

ROOT = Path(__file__).resolve().parent.parent
SCOPES = ("docs", "mobile", "server", "infra", "postgres", "all", "mobile-architecture")


def commands(scope):
    if scope == "mobile":
        return [("mobile", command) for command in (
            ["fvm", "dart", "format", "--output=none", "--set-exit-if-changed", "lib", "test"],
            ["fvm", "flutter", "analyze"],
            ["fvm", "flutter", "test"],
        )]
    if scope == "server":
        return [(directory, ["go", operation, "./..."])
                for directory in ("server", "server/agent")
                for operation in ("vet", "test")]
    if scope == "infra":
        return [("infra", ["npm", "run", operation])
                for operation in ("build", "lint", "test")]
    if scope == "postgres":
        return [("server", ["go", "test", "-count=1", "-v", "-run",
                            "^TestPostgresOwnershipAndSessions$",
                            "./internal/infrastructure/persistence/postgres"])]
    return []


def check_docs(root):
    paths = [root / "AGENTS.md"]
    for directory in (root / "docs/harness", root / "tools"):
        paths.extend(sorted(directory.rglob("*.md")))
    errors = []
    for path in paths:
        if not path.is_file():
            errors.append(f"missing: {path.relative_to(root)}")
            continue
        for target in re.findall(r"\[[^\]]*\]\(([^)]+)\)", path.read_text()):
            if target.startswith(("https://", "http://", "#", "mailto:")):
                continue
            target = target.split("#", 1)[0]
            if target and not (path.parent / target).exists():
                errors.append(f"{path.relative_to(root)}: missing link {target}")
    for error in errors:
        print(error)
    return "FAIL" if errors else "PASS"


def run_command(directory, command, env):
    if not directory.is_dir() or shutil.which(command[0]) is None:
        return "BLOCKED"
    try:
        result = subprocess.run(command, cwd=directory, env=env, check=False)
    except OSError as error:
        print(f"cannot start {command[0]}: {error.strerror}")
        return "BLOCKED"
    return "PASS" if result.returncode == 0 else "FAIL"


def check_go_format(root):
    if shutil.which("gofmt") is None:
        return "BLOCKED"
    # Explicit source roots exclude runtime data, configuration and build artifacts.
    roots = [root / "server" / name for name in ("cmd", "internal", "migrations", "agent")]
    files = sorted(path for source in roots for path in source.rglob("*.go")
                   if not path.is_symlink() and ".git" not in path.parts)
    if not files:
        return "BLOCKED"
    result = subprocess.run(["gofmt", "-l", *map(str, files)], capture_output=True, text=True)
    if result.stdout:
        print(result.stdout, end="")
    if result.stderr:
        print(result.stderr, end="", file=sys.stderr)
    return "PASS" if result.returncode == 0 and not result.stdout.strip() else "FAIL"


def exit_code(results):
    if "FAIL" in results:
        return 1
    return 2 if "BLOCKED" in results else 0


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("check",))
    parser.add_argument("scope", choices=SCOPES)
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args(argv)
    scopes = SCOPES[:4] if args.scope == "all" else (args.scope,)
    results = []

    def record(label, status):
        results.append(status)
        print(f"[{status}] {label}", flush=True)

    for scope in scopes:
        if scope in ("mobile", "mobile-architecture"):
            if args.dry_run:
                print("[PLAN] mobile architecture: shared HTTP construction (A-M07)")
            else:
                record("mobile architecture", check_mobile_architecture(ROOT))
            if scope == "mobile-architecture":
                continue
        if scope == "docs":
            if args.dry_run:
                print("[PLAN] docs: local Markdown link existence")
            else:
                record("docs", check_docs(ROOT))
            continue
        if scope == "server":
            if args.dry_run:
                print("[PLAN] server architecture: Go package and Agent boundaries (A-S01-A-S05)")
                print("[PLAN] server: gofmt -l (server and Agent source files)")
            else:
                record("server architecture", check_server_architecture(ROOT))
                record("server format", check_go_format(ROOT))
            print("[NOT RUN] PostgreSQL integration; use postgres scope", flush=True)
        env = os.environ.copy()
        if scope != "postgres":
            env.pop("ARRAN_TEST_DATABASE_URL", None)
        elif not args.dry_run and not env.get("ARRAN_TEST_DATABASE_URL"):
            record("postgres: ARRAN_TEST_DATABASE_URL is required", "BLOCKED")
            continue
        for cwd, command in commands(scope):
            label = f"{cwd}: {' '.join(command)}"
            if args.dry_run:
                print(f"[PLAN] {label}")
            else:
                print(f"[RUN] {label}", flush=True)
                record(label, run_command(ROOT / cwd, command, env))
    return exit_code(results)


if __name__ == "__main__":
    sys.exit(main())
