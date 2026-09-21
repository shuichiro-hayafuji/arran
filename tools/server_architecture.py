"""Static guard for the Go server and independent Agent architecture."""

from dataclasses import dataclass
from pathlib import Path
import re


SERVER_MODULE = "github.com/shuichirohayafuji/spendable-today/server"
INTERNAL_PREFIX = SERVER_MODULE + "/internal/"
AGENT_MODULE = "github.com/shuichiro-hayafuji/arran_agent"
TRANSACTION_FIELDS = {"TransactionDate", "Amount", "Category"}


@dataclass(frozen=True)
class DependencyException:
    """A narrow, reviewable exception mirrored in docs/harness/WORKFLOW.md."""

    rule: str
    importer: str
    imported: str
    issue: str


# Keep this empty when the implementation follows the documented boundaries.
# Any addition must name the same exception ID and resolution condition in WORKFLOW.md.
DEPENDENCY_EXCEPTIONS = ()


ALLOWED_INTERNAL = {
    "internal/agentadapter": {"internal/domain"},
    "internal/app": {
        "internal/agentadapter",
        "internal/application",
        "internal/auth",
        "internal/config",
        "internal/handler",
        "internal/infrastructure/openai",
        "internal/infrastructure/persistence/postgres",
    },
    "internal/application": {"internal/csvimport", "internal/domain", "internal/identity"},
    "internal/auth": {"internal/identity"},
    "internal/config": set(),
    "internal/csvimport": {"internal/domain"},
    "internal/domain": set(),
    "internal/handler": {"internal/application", "internal/domain"},
    "internal/identity": set(),
    "internal/infrastructure/openai": set(),
    "internal/infrastructure/persistence/postgres": {
        "internal/auth", "internal/domain", "internal/identity",
    },
    "internal/infrastructure/persistence/sqlite": {"internal/domain"},
}

AGENT_IMPORTERS = {
    "internal/agentadapter",
    "internal/app",
    "internal/infrastructure/openai",
}


def _strip_comments(source):
    """Remove Go comments while preserving strings and line positions."""
    output = []
    index = 0
    quote = None
    while index < len(source):
        current = source[index]
        following = source[index + 1] if index + 1 < len(source) else ""
        if quote:
            output.append(current)
            if quote != "`" and current == "\\" and following:
                output.append(following)
                index += 2
                continue
            if current == quote:
                quote = None
            index += 1
            continue
        if current in ('"', "'", "`"):
            quote = current
            output.append(current)
            index += 1
            continue
        if current == "/" and following == "/":
            while index < len(source) and source[index] != "\n":
                output.append(" ")
                index += 1
            continue
        if current == "/" and following == "*":
            output.extend("  ")
            index += 2
            while index < len(source):
                if source[index:index + 2] == "*/":
                    output.extend("  ")
                    index += 2
                    break
                output.append("\n" if source[index] == "\n" else " ")
                index += 1
            continue
        output.append(current)
        index += 1
    return "".join(output)


def _imports(source):
    code = _strip_comments(source)
    # Imports precede declarations in valid Go. Limiting the scan avoids treating
    # import-shaped text inside a later raw string as a dependency.
    declaration = re.search(r"(?m)^\s*(?:const|func|type|var)\b", code)
    if declaration:
        code = code[:declaration.start()]
    found = []
    single = re.compile(r'(?m)^\s*import\s+(?:[\w.]+\s+)?"([^"]+)"')
    block = re.compile(r'(?ms)^\s*import\s*\((.*?)^\s*\)')
    spans = []
    for match in block.finditer(code):
        spans.append(match.span())
        content = match.group(1)
        base = match.start(1)
        for item in re.finditer(r'(?m)^\s*(?:[\w.]+\s+)?"([^"]+)"', content):
            offset = base + item.start(1)
            found.append((item.group(1), code.count("\n", 0, offset) + 1))
    for match in single.finditer(code):
        if any(start <= match.start() < end for start, end in spans):
            continue
        found.append((match.group(1), code.count("\n", 0, match.start(1)) + 1))
    return found


def _package(source):
    match = re.search(r"(?m)^\s*package\s+([A-Za-z_]\w*)", _strip_comments(source))
    return match.group(1) if match else None


def _package_key(server_root, path):
    return path.parent.relative_to(server_root).as_posix()


def _expected_package(key):
    if key.startswith("cmd/"):
        return "main"
    return key.rsplit("/", 1)[-1]


def _is_excepted(exceptions, rule, importer, imported):
    return any(
        item.rule == rule and item.importer == importer and item.imported == imported
        and item.issue
        for item in exceptions
    )


def _agent_transaction_fields(agent_root):
    for path in sorted(agent_root.rglob("*.go")):
        if path.name.endswith("_test.go") or path.is_symlink():
            continue
        source = _strip_comments(path.read_text(encoding="utf-8"))
        match = re.search(r"\btype\s+Transaction\s+struct\s*\{(.*?)\}", source, re.DOTALL)
        if not match:
            continue
        fields = set()
        for line in match.group(1).splitlines():
            item = re.match(r"\s*([A-Za-z_]\w*(?:\s*,\s*[A-Za-z_]\w*)*)\s+", line)
            if item:
                fields.update(name.strip() for name in item.group(1).split(","))
        return path, fields
    return None, set()


def check_server_architecture(root, exceptions=DEPENDENCY_EXCEPTIONS):
    server_root = root / "server"
    agent_root = server_root / "agent"
    if not (server_root / "internal").is_dir() or not agent_root.is_dir():
        return "BLOCKED"

    errors = []
    go_files = sorted(server_root.rglob("*.go"))
    if not go_files:
        return "BLOCKED"
    for path in go_files:
        relative = path.relative_to(root).as_posix()
        if path.is_symlink():
            errors.append(f"{relative}: A-S01: symlink cannot be checked")
            continue
        if path.name.endswith("_test.go"):
            continue
        source = path.read_text(encoding="utf-8")
        key = _package_key(server_root, path)
        package = _package(source)
        expected = _expected_package(key)
        if package != expected:
            errors.append(
                f"{relative}:1: A-S01: package {package!r} does not match directory {expected!r}"
            )
        if key.startswith("internal/") and key not in ALLOWED_INTERNAL:
            errors.append(
                f"{relative}:1: A-S01: {key} has no registered dependency policy"
            )

        for imported, line in _imports(source):
            rule = None
            message = None
            if key == "internal/domain" and (
                imported.startswith(INTERNAL_PREFIX)
                or imported in {"net/http", "database/sql", "database/sql/driver", AGENT_MODULE}
                or imported.startswith(("github.com/jackc/pgx", "github.com/openai/"))
            ):
                rule = "A-S01"
                message = "domain must not depend on HTTP, DB, LLM, or another internal package"
            elif key.startswith("agent") and imported.startswith(INTERNAL_PREFIX):
                rule = "A-S04"
                message = "the independent Agent must not import parent server/internal"
            elif (imported == AGENT_MODULE or imported.startswith(AGENT_MODULE + "/")) \
                    and key not in AGENT_IMPORTERS:
                rule = "A-S04"
                message = "only app, agentadapter, and infrastructure/openai may import Agent"
            elif imported.startswith("database/sql") and key not in {
                "internal/infrastructure/persistence/postgres",
                "internal/infrastructure/persistence/sqlite",
            }:
                rule = "A-S05"
                message = "database/sql belongs in infrastructure/persistence"
            elif imported.startswith("net/http") and key == "internal/application":
                rule = "A-S02"
                message = "application must remain independent from HTTP"
            elif imported.startswith("github.com/jackc/pgx") \
                    and key != "internal/infrastructure/persistence/postgres":
                rule = "A-S05"
                message = "the PostgreSQL driver belongs in infrastructure/persistence/postgres"
            elif imported.startswith("github.com/openai/") \
                    and key != "internal/infrastructure/openai":
                rule = "A-S05"
                message = "OpenAI SDK imports belong in infrastructure/openai"
            elif imported.startswith(INTERNAL_PREFIX):
                target = "internal/" + imported[len(INTERNAL_PREFIX):]
                if target.startswith((
                    "internal/infrastructure/openai",
                    "internal/infrastructure/persistence/postgres",
                )) and key != "internal/app":
                    rule = "A-S05"
                    message = "OpenAI and PostgreSQL implementations are composed only in internal/app"
                elif key in ALLOWED_INTERNAL and target not in ALLOWED_INTERNAL[key]:
                    rule = "A-S01" if key != "internal/application" else "A-S02"
                    message = f"{key} must not depend on {target}"
            if rule and not _is_excepted(exceptions, rule, key, imported):
                errors.append(f"{relative}:{line}: {rule}: {message}")

    transaction_path, fields = _agent_transaction_fields(agent_root)
    if transaction_path is None:
        errors.append("server/agent: A-S03: Agent Transaction DTO was not found")
    elif fields != TRANSACTION_FIELDS:
        relative = transaction_path.relative_to(root).as_posix()
        errors.append(
            f"{relative}: A-S03/B-S08: Agent Transaction fields must be exactly "
            + ", ".join(sorted(TRANSACTION_FIELDS))
        )

    for error in errors:
        print(error)
    return "FAIL" if errors else "PASS"
