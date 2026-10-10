#!/usr/bin/env python3
"""Protect immutable provenance and explicitly reviewed English instructions.

Historical setup hashes remain recorded separately. Current prompt hashes cover
complete decoded Go string constants, including concatenation, using Go ASTs.
Do not regenerate hashes merely to silence failures; review the semantic change.
"""

import hashlib
import json
from pathlib import Path
import sys

# Resolve the checked-in helper even under Python isolated mode (-I).
sys.path.insert(0, str(Path(__file__).resolve().parent))
from localization_source import ROOT, check_source, digest, inspect_go


def audit_boundaries(root, manifest, constants):
    failures = []
    if manifest.get("version") != 2 or not manifest.get("authorization"):
        failures.append("Missing explicit English-first review authorization")
    history = manifest.get("historical_boundaries", {})
    if not history.get("files") or not history.get("templates"):
        failures.append("Historical source hashes must remain recorded")
    for section in ("files", "reviewed_files", "compatibility_files"):
        for filename, expected in manifest.get(section, {}).items():
            expected = expected if isinstance(expected, str) else expected["sha256"]
            path = root / filename
            if not path.is_file() or hashlib.sha256(path.read_bytes()).hexdigest() != expected:
                failures.append(f"{section}: {filename}")
    for identifier, review in manifest["templates"].items():
        if identifier not in constants or digest(constants[identifier]) != review["sha256"]:
            failures.append(f"Reviewed instruction changed: {identifier}")
    return failures


def check():
    manifest = json.loads((ROOT / "scripts/localization-boundaries.json").read_text())
    inventory = inspect_go()
    failures = audit_boundaries(ROOT, manifest, inventory["constants"])
    source_failures, _ = check_source(inventory=inventory)
    failures += source_failures
    if failures:
        print("Localization boundary checks failed:\n" + "\n".join(failures), file=sys.stderr)
        return 1
    print(f"Localization boundaries passed: {len(manifest['files'])} immutable source files, "
          f"{len(manifest['reviewed_files'])} reviewed instruction files, "
          f"{len(manifest['templates'])} complete English constants, "
          f"{len(manifest['compatibility_files'])} exact legacy snapshots, and source-language scan. "
          "Original setup hashes retained as historical provenance.")
    return 0


if __name__ == "__main__":
    sys.exit(check())
