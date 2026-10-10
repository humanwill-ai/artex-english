"""Regression cases for the static English UI boundary (no network/build needed)."""

import contextlib
import importlib.util
import io
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("check_web_cjk", Path(__file__).with_name("check-web-cjk.py"))
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)


class EnglishHTMLCheckerTests(unittest.TestCase):
    def test_visible_han_and_hangul_are_detected_including_entities(self):
        issues = checker.inspect_html("<h1>작업</h1><button>&#x4E2D;&#x6587;</button><p>English</p>")
        self.assertEqual([item.characters for item in issues], ["업작", "中文"])

    def test_extended_han_and_hangul_jamo_are_detected(self):
        self.assertEqual(checker.inspect_html("<span>𠀀𰀀ᄀㄱ</span>")[0].count, 4)

    def test_accessibility_and_control_labels_are_detected(self):
        html = ('<img alt="이미지"><button aria-label="삭제" title="删除">Delete</button>'
                '<input placeholder="검색"><input type="submit" value="저장">'
                '<p aria-description="설명" aria-valuetext="하나" aria-roledescription="그림"></p>'
                '<optgroup label="분류"></optgroup><meta name="description" content="소개">')
        self.assertEqual(len(checker.inspect_html(html)), 10)

    def test_script_style_template_and_comments_are_not_ui(self):
        html = ('<script>self.__next_f.push(["中文 한국어"])</script>'
                '<style>.한국어{content:"中文"}</style><!-- 中文 -->'
                '<template><p aria-label="한국어">中文</p></template><p>Ready</p>')
        self.assertEqual(checker.inspect_html(html), [])

    def test_technical_attributes_and_user_input_values_are_not_labels(self):
        html = ('<a href="/中文" data-source="한국어" class="中文">Open</a>'
                '<input type="text" value="사용자 원문"><input type="hidden" value="中文">')
        self.assertEqual(checker.inspect_html(html), [])

    def test_explicit_evidence_prompt_and_code_regions_preserve_originals(self):
        html = ('<pre data-i18n-preserve="evidence"><code>HTTP 中文 한국어</code></pre>'
                '<textarea data-i18n-preserve="prompt" aria-label="Prompt">원문 中文</textarea>'
                '<pre data-i18n-preserve="user-content">고객 입력</pre>'
                '<code data-i18n-preserve="technical">命令</code><button>Copy</button>')
        self.assertEqual(checker.inspect_html(html), [])

    def test_raw_region_does_not_exempt_labels_or_following_siblings(self):
        html = ('<pre data-i18n-preserve="evidence" aria-label="증거">中文</pre>'
                '<button>복사</button><input placeholder="搜索">')
        self.assertEqual([item.surface for item in checker.inspect_html(html)],
                         ["pre[aria-label]", "text", "input[placeholder]"])

    def test_code_and_textarea_are_not_exempt_without_marker(self):
        self.assertEqual(len(checker.inspect_html('<code>中文</code><textarea>한국어</textarea>')), 2)

    def test_broad_or_unknown_preservation_markers_fail(self):
        issues = checker.inspect_html('<div data-i18n-preserve="evidence">中文</div>'
                                      '<pre data-i18n-preserve="anything">한국어</pre>')
        self.assertEqual(len(issues), 4)
        self.assertEqual(sum(item.surface == "invalid preservation marker" for item in issues), 2)

    def test_closing_parent_and_void_tags_do_not_leak_exemptions(self):
        html = ('<section><pre data-i18n-preserve="evidence"><code>中文</section>'
                '<img alt="Photo"><br><p>작업</p>')
        self.assertEqual(len(checker.inspect_html(html)), 1)

    def test_normal_english_with_technical_punctuation_passes(self):
        self.assertEqual(checker.inspect_html('<html lang="en"><title>ARTEX</title>'
                                             '<p>Task #42 · Input/output → 1,000</p></html>'), [])

    def test_cli_statuses_and_recursive_html_discovery(self):
        with tempfile.TemporaryDirectory() as tmp, contextlib.redirect_stdout(io.StringIO()) as output:
            root = Path(tmp)
            self.assertEqual(checker.main([str(root / "missing")]), 2)
            self.assertEqual(checker.main([str(root)]), 2)
            (root / "nested").mkdir()
            html = root / "nested" / "page.html"
            html.write_text('<button>Save</button>', encoding="utf-8")
            self.assertEqual(checker.main([str(root)]), 0)
            html.write_text('<button>저장</button>', encoding="utf-8")
            self.assertEqual(checker.main([str(root)]), 1)
            self.assertIn('nested/page.html:1:', output.getvalue())


if __name__ == "__main__":
    unittest.main()
