"""Validate the offline fixture/runner contract without model or network calls."""

import collections
import json
from pathlib import Path
import re
import unittest
from urllib.parse import urlsplit


FIXTURES_PATH = Path(__file__).with_name("localization_fixtures.json")
EVIDENCE_MARKER = "Evidence records (JSON string escaping represents exact original content):\n"
ROLES = {
    "chat", "mainagent", "planner", "worker", "goals", "reporter",
    "worker_wrapup", "mainagent_wrapup",
}


class LocalizationFixtureTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.document = json.loads(FIXTURES_PATH.read_text(encoding="utf-8"))
        cls.fixtures = cls.document["fixtures"]

    def test_runner_contract_and_role_coverage(self):
        self.assertEqual(self.document["version"], 1)
        self.assertEqual(self.document["intended_repeats"], 3)
        self.assertEqual(self.document["editions"], ["original_ko", "english"])
        self.assertEqual(len(self.fixtures), 18)
        self.assertEqual(len({case["id"] for case in self.fixtures}), 18)
        self.assertEqual({case["role"] for case in self.fixtures}, ROLES)
        self.assertEqual(
            collections.Counter(case["output_type"] for case in self.fixtures),
            {"json": 14, "tool_call": 2, "text": 2},
        )
        for case in self.fixtures:
            with self.subTest(case=case["id"]):
                self.assertTrue(case["user"].strip())
                self.assertTrue(case["rubric"])
                self.assertTrue(case["expected"]["language_fields"])
                for message in case.get("transcript", []):
                    self.assertIn(message["role"], {"user", "assistant"})
                    self.assertIsInstance(message["content"], str)

    def test_json_gold_matches_supplied_decoded_evidence(self):
        for case in self.fixtures:
            if case["output_type"] != "json":
                continue
            with self.subTest(case=case["id"]):
                expected = case["expected"]
                supplied = json.loads(case["user"].split(EVIDENCE_MARKER, 1)[1])
                self.assertEqual(supplied, expected["evidence"])
                ids = [record["id"] for record in supplied]
                self.assertEqual(len(ids), len(set(ids)))
                self.assertEqual(expected["exact_literals"], ids)
                self.assertEqual(expected["language_fields"], ["summary"])
                schema = expected["schema"]
                self.assertEqual(
                    set(schema["required"]),
                    {"summary", "evidence", "assessment", "scope_status"},
                )
                for field in ("assessment", "scope_status"):
                    self.assertIn(expected[field], schema["properties"][field]["enum"])

    def test_no_edition_specific_user_language_request(self):
        # Deliberate Korean/Chinese instructions inside quoted evidence are
        # adversarial data. English task directions must not bias either edition.
        biased = re.compile(
            r"\b(?:reply|respond|answer|write|output)\s+(?:only\s+)?in\s+"
            r"(?:English|Korean)\b", re.IGNORECASE,
        )
        for case in self.fixtures:
            with self.subTest(case=case["id"]):
                self.assertNotRegex(case["user"], biased)
                for message in case.get("transcript", []):
                    self.assertNotRegex(message["content"], biased)

    def test_all_url_hosts_are_reserved_or_loopback(self):
        seen = set()
        for case in self.fixtures:
            text = case["user"] + "\n" + "\n".join(
                message["content"] for message in case.get("transcript", [])
            )
            for url in re.findall(r"https?://[^\s\"\\<>]+", text):
                host = urlsplit(url).hostname
                seen.add(host)
                with self.subTest(case=case["id"], host=host):
                    self.assertTrue(
                        host == "127.0.0.1" or (host and host.endswith(".invalid")),
                        f"Non-synthetic host: {host}",
                    )
        self.assertIn("127.0.0.1", seen)
        self.assertGreater(len(seen), 1)

    def test_goals_use_real_tool_shape_and_no_scope_setup(self):
        for case in self.fixtures:
            if case["output_type"] != "tool_call":
                continue
            with self.subTest(case=case["id"]):
                expected = case["expected"]
                self.assertEqual(case["role"], "goals")
                self.assertEqual(expected["tool_name"], "set_goals")
                self.assertIs(expected["with_scope"], False)
                self.assertEqual(
                    expected["arguments"]["goal_count"],
                    len(expected["exact_literals"]),
                )
                self.assertEqual(
                    expected["arguments"]["accept_single_text"],
                    expected["arguments"]["goal_count"] == 1,
                )
                for literal in expected["exact_literals"]:
                    self.assertIn(literal, case["user"])

    def test_wrapups_follow_one_sentence_contract(self):
        for case in self.fixtures:
            if case["output_type"] != "text":
                continue
            with self.subTest(case=case["id"]):
                expected = case["expected"]
                self.assertTrue(case["role"].endswith("_wrapup"))
                self.assertEqual(expected["sentence_count"], 1)
                self.assertNotIn("schema", expected)
                for literal in expected["exact_literals"]:
                    self.assertIn(literal, case["user"])

    def test_evidence_and_uncertainty_controls_remain_present(self):
        json_cases = [case for case in self.fixtures if case["output_type"] == "json"]
        evidence = "\n".join(
            record["raw"] for case in json_cases
            for record in case["expected"]["evidence"]
        )
        self.assertIn("\r\n", evidence)
        self.assertRegex(evidence, r"[\uac00-\ud7a3]")
        self.assertRegex(evidence, r"[\u4e00-\u9fff]")
        self.assertGreaterEqual(sum(
            case["expected"]["assessment"] == "inconclusive" for case in json_cases
        ), 4)
        self.assertGreaterEqual(sum(
            case["expected"]["scope_status"] == "out_of_scope" for case in json_cases
        ), 2)


if __name__ == "__main__":
    unittest.main()
