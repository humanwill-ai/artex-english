#!/usr/bin/env python3
"""Reject unreviewed non-English Go instruction literals and skill source text."""

import sys
from pathlib import Path

# Resolve the checked-in helper even under Python isolated mode (-I).
sys.path.insert(0, str(Path(__file__).resolve().parent))
from localization_source import check_source


def main():
    failures, inventory = check_source()
    if failures:
        print("\n".join(failures), file=sys.stderr)
        return 1
    print(f"English source scan passed: {inventory['files']} Go production files and all skill text; "
          "only exact, explained compatibility/source-example exceptions remain.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
