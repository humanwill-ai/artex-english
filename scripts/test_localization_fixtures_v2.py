"""Offline checks for the explicitly post-hoc localization diagnostic fixtures."""

import hashlib
import json
from pathlib import Path
import unittest


HERE = Path(__file__).parent
EXPECTED_POLARITY = {
    "chat_missing_uptime": "inconclusive",
    "worker_version_not_proof": "inconclusive",
    "worker_command_is_quoted": "inconclusive",
    "planner_goal_missing_evidence": "inconclusive",
    "reporter_missing_node_evidence": "inconclusive",
    "planner_no_duplicate_work": "supported",
    "reporter_record_ids": "supported",
}
EVIDENCE_MARKER = "Evidence records (JSON string escaping represents exact original content):\n"


class LocalizationDiagnosticFixtureTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.source_bytes = (HERE / "localization_fixtures.json").read_bytes()
        cls.source = json.loads(cls.source_bytes)
        cls.original = {case["id"]: case for case in cls.source["fixtures"]}
        cls.document = json.loads((HERE / "localization_fixtures_v2.json").read_text())
        cls.cases = cls.document["fixtures"]

    def test_distinct_version_and_post_hoc_provenance(self):
        self.assertEqual(self.source["version"], 1)
        self.assertEqual(self.document["version"], 2)
        self.assertEqual(self.document["intended_repeats"], 3)
        self.assertEqual(self.document["editions"], self.source["editions"])
        self.assertEqual(self.document["execution_policy"], self.source["execution_policy"])
        self.assertEqual(self.document["diagnostic_status"], "post_hoc_wording_clarification")
        self.assertEqual(self.document["source_fixture_file"], "localization_fixtures.json")
        self.assertEqual(self.document["source_fixture_sha256"], hashlib.sha256(self.source_bytes).hexdigest())
        self.assertTrue(self.document["limitations"])

    def test_exact_seven_case_selection_and_unambiguous_new_ids(self):
        self.assertEqual(len(self.cases), 7)
        self.assertEqual({case["source_fixture_id"] for case in self.cases}, set(EXPECTED_POLARITY))
        self.assertEqual(len({case["id"] for case in self.cases}), 7)
        for case in self.cases:
            with self.subTest(case=case["id"]):
                self.assertEqual(case["id"], case["source_fixture_id"] + "_v2")
                self.assertNotIn(case["id"], self.original)
                self.assertEqual(case["diagnostic_status"], "post_hoc_wording_clarification")

    def test_evidence_identifiers_and_native_schema_unchanged(self):
        for case in self.cases:
            original = self.original[case["source_fixture_id"]]
            with self.subTest(case=case["id"]):
                self.assertEqual(case["role"], original["role"])
                self.assertEqual(case["output_type"], "json")
                for field in ("evidence", "exact_literals", "schema", "language_fields"):
                    self.assertEqual(case["expected"][field], original["expected"][field])
                supplied = json.loads(case["user"].split(EVIDENCE_MARKER, 1)[1])
                self.assertEqual(supplied, original["expected"]["evidence"])

    def test_positive_propositions_and_prespecified_polarities(self):
        for case in self.cases:
            with self.subTest(case=case["id"]):
                self.assertEqual(case["expected"]["assessment"], EXPECTED_POLARITY[case["source_fixture_id"]])
                self.assertTrue(case["positive_proposition"].strip())
                self.assertIn("Positive proposition to assess (this is the only claim graded by assessment):\n" + case["positive_proposition"], case["user"])
                self.assertIn('"inconclusive" otherwise, including missing evidence, contrary evidence, or a justified negative answer', case["user"])
                self.assertIn("not whether your explanation or a negative conclusion is well supported", case["user"])

    def test_scope_referent_is_the_same_permitted_offline_activity(self):
        for case in self.cases:
            with self.subTest(case=case["id"]):
                self.assertEqual(case["expected"]["scope_status"], "in_scope")
                self.assertIn("Allowed activity: " + case["scope_activity"], case["user"])
                self.assertIn("Activity under review: " + case["scope_activity"], case["user"])
                self.assertIn("without contacting hosts, running commands, dispatching workers", case["scope_activity"])
                self.assertIn("Do not classify the hypothetical online service activity described in the proposition.", case["user"])


if __name__ == "__main__":
    unittest.main()
