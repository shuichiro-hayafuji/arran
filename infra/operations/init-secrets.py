#!/usr/bin/env python3
"""Run once as root in /opt/arran. Refuses to replace any existing secret."""
import os
from pathlib import Path
import secrets

if os.geteuid() != 0:
    raise SystemExit("Run as root; secrets must be readable by container service UIDs only")
directory = Path(__file__).resolve().parent / "secrets"
directory.mkdir(mode=0o700, exist_ok=True)
if any(directory.iterdir()):
    raise SystemExit("Secret directory is not empty. Do not regenerate passwords for an existing database")
passwords = {name: secrets.token_hex(32) for name in ("bootstrap", "migration", "runtime")}
for name, password in passwords.items():
    p = directory / (name + "_password")
    p.write_text(password)
    # PostgreSQL images use different UIDs. The root-only parent directory
    # protects host access; containers receive only individually mounted files.
    os.chown(p, 0, 0)
    p.chmod(0o444)
for name, role in (("migration", "arran_migrator"), ("runtime", "arran_runtime")):
    p = directory / (name + "_url")
    p.write_text("postgres://%s:%s@db:5432/arran?sslmode=disable" % (role, passwords[name]))
    os.chown(p, 10001, 10001)
    p.chmod(0o400)
p = directory / "openai_key"
p.touch(mode=0o400)
os.chown(p, 10001, 10001)
print("Created secret files. Enter the OpenAI key separately before releasing the API.")
