"""Focused lexical guard for the shared mobile HTTP construction boundary."""

import re


# Keep offsets/newlines for diagnostics; this is not a Dart AST parser.
NON_CODE = re.compile(
    r"//[^\n]*|/\*.*?\*/|r?(?:\"\"\".*?\"\"\"|'''.*?'''|"
    r"\"(?:\\.|[^\"\\])*\"|'(?:\\.|[^'\\])*')",
    re.DOTALL,
)
FORBIDDEN = re.compile(r"\bDio\b|\bApiClient\s*(?:\(|\.\s*(?:init|new)\b)")
CLIENT = "mobile/lib/core/network/api_client.dart"


def check_mobile_architecture(root):
    source = root / "mobile/lib"
    if not source.is_dir() or not (root / CLIENT).is_file():
        return "BLOCKED"
    errors = []
    for path in sorted(source.rglob("*.dart")):
        relative = path.relative_to(root).as_posix()
        if relative == CLIENT:
            continue
        if path.is_symlink():
            errors.append(f"{relative}: A-M07: symlink cannot be checked")
            continue
        text = path.read_text(encoding="utf-8")
        code = NON_CODE.sub(lambda match: re.sub(r"[^\n]", " ", match[0]), text)
        for match in FORBIDDEN.finditer(code):
            line = code.count("\n", 0, match.start()) + 1
            errors.append(
                f"{relative}:{line}: A-M07: use publicApiClientProvider or "
                "authorizedApiClientProvider; HTTP construction belongs in " + CLIENT
            )
    for error in errors:
        print(error)
    return "FAIL" if errors else "PASS"
