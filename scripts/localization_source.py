"""Offline Go-AST and skill-text checks for application-owned English sources.

This detects East Asian source text, not language fluency or behavioral parity.
Comments/tests and custom runtime data are deliberately outside the Go scan.
"""

import collections
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parent.parent
EAST_ASIAN = re.compile(
    r"[\u1100-\u11ff\u3040-\u30ff\u3100-\u312f\u3130-\u318f"
    r"\u31a0-\u31bf\u3400-\u4dbf\u4e00-\u9fff\uac00-\ud7af"
    r"\uf900-\ufaff\U00020000-\U000323af]"
)


def digest(value):
    return hashlib.sha256(value.encode("utf-8")).hexdigest()


def inspect_go(root=ROOT):
    # Explicit-file go run includes the build-ignored helper, never the target
    # application's packages. No imports beyond the standard library are used.
    result = subprocess.run(
        ["go", "run", str(ROOT / "scripts/localization_ast.go"), str(root)],
        cwd=ROOT, check=True, text=True, capture_output=True,
        env={**os.environ, "GOTOOLCHAIN": "local", "GOWORK": "off", "GOPROXY": "off"},
    )
    return json.loads(result.stdout)


def inspect_skills(root=ROOT):
    rows = []
    for path in sorted((root / "skills").rglob("*")):
        if not path.is_file() or path.suffix not in {".md", ".py", ".js", ".mjs", ".json"}:
            continue
        for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
            if EAST_ASIAN.search(line):
                rows.append({"file": path.relative_to(root).as_posix(), "line": number,
                             "context": "skill source line", "value": line})
    return rows


def audit_exceptions(rows, exceptions, kind):
    """Exact file/context/value/count exceptions; stale exceptions also fail."""
    failures = []
    by_key = {}
    for item in exceptions:
        key = (item["file"], item["context"], item["sha256"])
        if key in by_key:
            failures.append(f"Duplicate {kind} exception: {item['file']}")
        if digest(item["value"]) != item["sha256"] or not item.get("reason", "").strip():
            failures.append(f"Invalid or unexplained {kind} exception: {item['file']}")
        if not isinstance(item.get("count"), int) or item["count"] < 1:
            failures.append(f"Invalid {kind} exception count: {item['file']}")
        by_key[key] = item
    seen = collections.Counter()
    for row in rows:
        key = (row["file"], row["context"], digest(row["value"]))
        seen[key] += 1
        if key not in by_key:
            preview = row["value"].replace("\n", " ")[:100]
            failures.append(f"Unreviewed {kind} text: {row['file']}:{row['line']} ({row['context']}): {preview}")
    for key, item in by_key.items():
        if seen[key] != item["count"]:
            failures.append(f"{kind} exception occurrence count changed: {item['file']} "
                            f"({item['context']}), expected {item['count']}, found {seen[key]}")
    return failures


def check_source(root=ROOT, inventory=None, manifest=None):
    inventory = inspect_go(root) if inventory is None else inventory
    if manifest is None:
        manifest = json.loads((root / "scripts/english-source-exceptions.json").read_text())
    failures = audit_exceptions(inventory["literals"], manifest["go_literals"], "Go literal")
    failures += audit_exceptions(inspect_skills(root), manifest["skill_lines"], "skill")
    return failures, inventory
