#!/usr/bin/env python3
"""macOS local notification for the documented Codex Stop event only."""

import json
import subprocess
import sys

SCRIPT = '''display notification "応答が完了しました。検証結果は回答を確認してください。" with title "Arran / Codex"'''


def notify(payload):
    if payload.get("hook_event_name") != "Stop":
        return {}
    # Fixed text only: never interpolate assistant output, paths or user input into AppleScript.
    if sys.platform != "darwin":
        return {"systemMessage": "Arran notification supports macOS only; see tools/HOOKS.md."}
    try:
        subprocess.run(["/usr/bin/osascript", "-e", SCRIPT], capture_output=True,
                       check=True, timeout=5)
    except (OSError, subprocess.SubprocessError) as error:
        detail = getattr(error, "stderr", None)
        if isinstance(detail, bytes):
            detail = detail.decode(errors="replace").strip()
        return {"systemMessage": f"Arran notification failed: {detail or error}. Check macOS notification "
                "permissions and retry with the test command in tools/HOOKS.md. Validation status is unchanged."}
    return {}


def main():
    try:
        payload = json.load(sys.stdin)
        if not isinstance(payload, dict):
            raise ValueError("Expected a JSON object")
        result = notify(payload)
    except (ValueError, OSError) as error:
        result = {"systemMessage": f"Arran notification input error: {error}. See tools/HOOKS.md."}
    # Stop expects JSON. Never return decision:block or exit 2, which would continue the turn.
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
