"""Offline regression tests for English-source and provenance boundaries."""

import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

from localization_source import audit_exceptions, digest, inspect_go, inspect_skills

HERE = Path(__file__).parent
spec = importlib.util.spec_from_file_location("boundaries", HERE / "check-localization-boundaries.py")
boundaries = importlib.util.module_from_spec(spec)
spec.loader.exec_module(boundaries)


class EnglishSourceTests(unittest.TestCase):
    def test_ast_decodes_escapes_ignores_comments_tests_and_assembles_full_constants(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "prompt.go").write_text(
                'package sample\n// 中文 comment is not a runtime instruction\n'
                'const Header = "English heading"\n'
                'const Prompt = Header + ` and ` + "\\u4e2d\\u6587"\n'
                'func F() string { return "\\ud55c\\uae00" }\n'
            )
            (root / "prompt_test.go").write_text('package sample\nconst Fixture = "原始证据"\n')
            (root / "work").mkdir()
            (root / "work/temporary.go").write_text('not valid go; ignored task output')
            result = inspect_go(root)
            self.assertEqual(result["files"], 1)
            self.assertEqual(result["constants"]["prompt.go::Prompt"], "English heading and 中文")
            self.assertEqual({row["value"] for row in result["literals"]}, {"中文", "한글"})
            self.assertEqual(next(row["context"] for row in result["literals"] if row["value"] == "한글"), "F")

    def test_new_unreviewed_prose_cannot_hide_behind_an_allowed_provider_token(self):
        row = {"file": "agent/provider.go", "line": 12, "context": "IsQuotaExhaustedMessage", "value": "余额不足"}
        exception = {k: row[k] for k in ("file", "context", "value")}
        exception.update(sha256=digest(row["value"]), count=1, reason="Exact external provider token")
        self.assertEqual(audit_exceptions([row], [exception], "test"), [])
        changed = dict(row, value="余额不足：请充值")
        self.assertTrue(audit_exceptions([changed], [exception], "test"))
        self.assertTrue(audit_exceptions([dict(row, context="NewPrompt")], [exception], "test"))
        self.assertTrue(audit_exceptions([row, row], [exception], "test"))
        self.assertTrue(audit_exceptions([], [exception], "test"))

    def test_exception_content_hash_and_rationale_are_required(self):
        row = {"file": "x.go", "line": 1, "context": "F", "value": "原文"}
        exception = dict(file="x.go", context="F", value="原文", sha256="bad", count=1, reason="")
        failures = audit_exceptions([row], [exception], "test")
        self.assertTrue(any("unexplained" in failure for failure in failures))

    def test_skill_prose_and_executable_sources_both_checked(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "skills/demo").mkdir(parents=True)
            (root / "skills/demo/SKILL.md").write_text("# English\nNew instruction 中文\n")
            (root / "skills/demo/check.js").write_text('const externalPattern = /未授权/;\n')
            rows = inspect_skills(root)
            self.assertEqual(len(rows), 2)
            self.assertEqual({row["file"] for row in rows}, {"skills/demo/SKILL.md", "skills/demo/check.js"})
            self.assertEqual(len(audit_exceptions(rows, [], "skill")), 2)


class BoundaryTests(unittest.TestCase):
    def manifest(self):
        return {
            "version": 2, "authorization": "Reviewed English-first instruction translation",
            "historical_boundaries": {"files": {"LICENSE": "old"}, "templates": {"x.go::Prompt": "old"}},
            "files": {}, "reviewed_files": {}, "compatibility_files": {},
            "templates": {"x.go::Prompt": {"sha256": digest("Header and protected ending")}},
        }

    def test_full_concatenated_prompt_ending_is_protected(self):
        manifest = self.manifest()
        self.assertEqual(boundaries.audit_boundaries(Path("."), manifest, {"x.go::Prompt": "Header and protected ending"}), [])
        failures = boundaries.audit_boundaries(Path("."), manifest, {"x.go::Prompt": "Header and changed ending"})
        self.assertTrue(any("Reviewed instruction changed" in failure for failure in failures))

    def test_immutable_source_and_exact_legacy_snapshot_both_protected(self):
        import hashlib
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "LICENSE").write_text("original license")
            (root / "legacy.json").write_text('{"old":"旧提示"}')
            manifest = self.manifest()
            manifest["files"] = {"LICENSE": hashlib.sha256((root / "LICENSE").read_bytes()).hexdigest()}
            manifest["compatibility_files"] = {"legacy.json": {"sha256": hashlib.sha256((root / "legacy.json").read_bytes()).hexdigest()}}
            constants = {"x.go::Prompt": "Header and protected ending"}
            self.assertEqual(boundaries.audit_boundaries(root, manifest, constants), [])
            (root / "LICENSE").write_text("changed license")
            (root / "legacy.json").write_text('{"old":"rewritten"}')
            failures = boundaries.audit_boundaries(root, manifest, constants)
            self.assertIn("files: LICENSE", failures)
            self.assertIn("compatibility_files: legacy.json", failures)

    def test_missing_history_or_authorization_fails(self):
        manifest = self.manifest()
        manifest["historical_boundaries"] = {}
        manifest["authorization"] = ""
        failures = boundaries.audit_boundaries(Path("."), manifest, {"x.go::Prompt": "Header and protected ending"})
        self.assertEqual(len(failures), 2)

    def test_manifest_keeps_original_v1_hashes_as_provenance(self):
        manifest = json.loads((HERE / "localization-boundaries.json").read_text())
        history = manifest["historical_boundaries"]
        import hashlib
        canonical = json.dumps({key: history[key] for key in ("files", "templates")}, sort_keys=True, separators=(",", ":"))
        self.assertEqual(hashlib.sha256(canonical.encode()).hexdigest(), "7cf71875bb5780e7f52af9da411670e4e62eff3461de9b9e4d39be4db7d66627")
        self.assertEqual(len(history["files"]), 8)
        self.assertEqual(len(history["templates"]), 9)
        self.assertEqual(history["templates"]["agent/promptcatalog.go::ReporterDefaultPrompt"], "77f74afbc6ba1090a898c4e2cbf9dcadaa11736a44e25e8b05fb59ec7679d413")
        self.assertIn("first raw-string segment", history["hash_semantics"])


if __name__ == "__main__":
    unittest.main()
