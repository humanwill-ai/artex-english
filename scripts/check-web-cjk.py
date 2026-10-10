#!/usr/bin/env python3
"""Check English static-export HTML for untranslated Han or Hangul UI text.

Inspect rendered text and user-facing/accessibility attributes, not JavaScript,
CSS, comments, inert templates, or Next.js hydration/Flight payloads. Serialized
source catalogs, historical evidence, and prompt bodies can legitimately contain
other languages and must not be translated merely to satisfy this check.

Preserved content requires a narrow, explicit marker on its content element:
  <pre data-i18n-preserve="evidence">original HTTP response</pre>
  <textarea data-i18n-preserve="prompt">original/custom prompt</textarea>
Supported reasons and elements are listed in PRESERVED_REGIONS below. The marker
only exempts text content; surrounding labels and accessibility attributes are
still checked. Do not mark whole panels or pages as preserved content.

This is a static HTML check, not a browser or language-quality test. Text rendered
only after hydration, API responses, CSS-generated content, and canvas labels need
source/catalog checks and browser review as well. Han/Hangul absence does not
establish that every string is English or that a translation is accurate.

Uses only the Python standard library. Run after the English static build:
  python3 -I scripts/check-web-cjk.py [export-directory]
"""

import argparse
from html.parser import HTMLParser
from pathlib import Path
import re
import sys
from typing import NamedTuple


NON_ENGLISH = re.compile(
    r"[\u3400-\u9fff\uf900-\ufaff\U00020000-\U0002ffff\U00030000-\U000323af"
    r"\u1100-\u11ff\u3130-\u318f\ua960-\ua97f\uac00-\ud7ff\uffa0-\uffdc]"
)
# These are explicit content-only exceptions, not a character allowlist.
PRESERVED_REGIONS = {
    "evidence": {"pre", "code"},
    "prompt": {"pre", "code", "textarea"},
    "user-content": {"pre", "code", "textarea"},
    "technical": {"code"},
}
INERT_ELEMENTS = {"script", "style", "template"}
VOID_ELEMENTS = {
    "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta",
    "param", "source", "track", "wbr",
}
DISPLAY_ATTRIBUTES = {
    "alt", "title", "placeholder", "aria-label", "aria-description",
    "aria-valuetext", "aria-roledescription",
}
DESCRIPTION_METADATA = {
    "description", "og:title", "og:description", "twitter:title", "twitter:description",
}


class Issue(NamedTuple):
    line: int
    column: int
    surface: str
    count: int
    characters: str


class EnglishHTMLChecker(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        # (element, inert subtree, explicitly preserved content subtree)
        self.stack = []
        self.issues = []

    def check_text(self, text, surface):
        found = NON_ENGLISH.findall(text)
        if found:
            line, column = self.getpos()
            self.issues.append(Issue(line, column + 1, surface, len(found), "".join(sorted(set(found)))))

    def handle_starttag(self, tag, attrs):
        values = dict(attrs)
        inherited_inert = bool(self.stack and self.stack[-1][1])
        inert = inherited_inert or tag in INERT_ELEMENTS
        preserved = bool(self.stack and self.stack[-1][2])
        marker = values.get("data-i18n-preserve")
        if not inert:
            if "data-i18n-preserve" in values:
                if tag in PRESERVED_REGIONS.get(marker, set()):
                    preserved = True
                else:
                    line, column = self.getpos()
                    self.issues.append(Issue(line, column + 1, "invalid preservation marker", 0, ""))
            # Labels remain English even inside a preserved evidence/prompt region.
            for name, value in attrs:
                if value is None:
                    continue
                if name in DISPLAY_ATTRIBUTES:
                    self.check_text(value, f"{tag}[{name}]")
                elif name == "label" and tag in {"option", "optgroup", "track"}:
                    self.check_text(value, f"{tag}[label]")
                elif name == "value" and (tag == "button" or (
                    tag == "input" and values.get("type", "text").lower() in {"button", "submit", "reset"}
                )):
                    self.check_text(value, f"{tag}[value]")
                elif name == "content" and tag == "meta" and (
                    values.get("name", "").lower() in DESCRIPTION_METADATA
                    or values.get("property", "").lower() in DESCRIPTION_METADATA
                ):
                    self.check_text(value, "meta[content]")
        if tag not in VOID_ELEMENTS:
            self.stack.append((tag, inert, preserved))

    def handle_startendtag(self, tag, attrs):
        self.handle_starttag(tag, attrs)
        if tag not in VOID_ELEMENTS:
            self.handle_endtag(tag)

    def handle_endtag(self, tag):
        # Match an ancestor rather than leaking an exemption into following nodes.
        for index in range(len(self.stack) - 1, -1, -1):
            if self.stack[index][0] == tag:
                del self.stack[index:]
                return

    def handle_data(self, data):
        if not self.stack or not (self.stack[-1][1] or self.stack[-1][2]):
            self.check_text(data, "text")


def inspect_html(text):
    checker = EnglishHTMLChecker()
    checker.feed(text)
    checker.close()
    return checker.issues


def check_directory(out_dir):
    if not out_dir.is_dir():
        print(f"Error: export directory does not exist: {out_dir}")
        print("Run the English static build first (npm run build:static).")
        return 2
    files = sorted(out_dir.rglob("*.html"))
    if not files:
        print(f"Error: no HTML files found in {out_dir}; check the static build.")
        return 2
    failures = 0
    for path in files:
        relative = path.relative_to(out_dir)
        try:
            issues = inspect_html(path.read_text(encoding="utf-8"))
        except (OSError, UnicodeError) as error:
            print(f"Error reading {relative}: {error}")
            return 2
        for issue in issues:
            failures += 1
            # Do not echo surrounding text: a preserved record may contain secrets.
            preview = issue.characters[:30]
            print(f"{relative}:{issue.line}:{issue.column}: {issue.surface}: "
                  f"{issue.count} Han/Hangul characters {preview}")
    print(f"Checked {len(files)} HTML files in {out_dir}.")
    if failures:
        print(f"Failed: {failures} untranslated UI surfaces or invalid preservation markers.")
        print("Translate application text; mark only original evidence, prompts, user content, "
              "or technical code with a supported data-i18n-preserve region.")
        return 1
    print("Passed: no Han/Hangul in checked application text or accessibility labels.")
    print("Runtime-rendered text still requires catalog checks and browser review.")
    return 0


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", nargs="?", type=Path,
                        default=Path(__file__).resolve().parents[1] / "web" / "out")
    return check_directory(parser.parse_args(argv).directory.resolve())


if __name__ == "__main__":
    sys.exit(main())
