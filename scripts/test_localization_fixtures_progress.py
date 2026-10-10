"""Post-hoc completion diagnosis; offline checks never contact a provider."""
import hashlib
import json
from pathlib import Path
import unittest
from run_localization_eval import build_request, script_counts
from test_localization_fixtures import EVIDENCE_MARKER

class ProgressDiagnosticTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.root = Path(__file__).parent
        cls.document = json.loads((cls.root / 'localization_fixtures_progress.json').read_text())
        cls.fixtures = cls.document['fixtures']
        cls.source = json.loads((cls.root / cls.document['source_fixture_file']).read_text())
        cls.original = next(f for f in cls.source['fixtures'] if f['id'] == cls.document['source_fixture_id'])

    def test_original_is_frozen_and_control_request_identical(self):
        self.assertEqual(hashlib.sha256((self.root / self.document['source_fixture_file']).read_bytes()).hexdigest(), self.document['source_fixture_sha256'])
        export = {'prompts': {'mainagent': 'Unchanged application prompt.'}}
        self.assertEqual(build_request(self.original, export), build_request(self.fixtures[0], export))
        self.assertEqual(self.original['expected'], self.fixtures[0]['expected'])
        self.assertNotIn(self.fixtures[0]['id'], {f['id'] for f in self.source['fixtures']})

    def test_defined_variant_adds_only_dictionary(self):
        control, defined, _ = self.fixtures
        self.assertEqual(defined['user'].replace(self.document['dictionary'], '', 1), control['user'])
        self.assertEqual(defined['transcript'], control['transcript'])
        self.assertEqual(defined['expected'], control['expected'])

    def test_negative_control_removes_completion_evidence_only(self):
        _, defined, missing = self.fixtures
        self.assertEqual(missing['user'], defined['user'].replace(
            'completed=read_saved_status; status=ready; detail=check completed;',
            'completed=<missing>; status=ready; detail=<missing>;'))
        self.assertEqual(missing['expected']['assessment'], 'inconclusive')
        self.assertEqual(missing['expected']['scope_status'], 'in_scope')
        self.assertEqual(missing['expected']['schema'], defined['expected']['schema'])
        self.assertEqual(missing['transcript'], defined['transcript'])

    def test_exact_evidence_english_provenance_and_distinct_ids(self):
        self.assertIn('Assistant-authored', self.document['provenance'])
        self.assertEqual(len({f['id'] for f in self.fixtures}), 3)
        self.assertEqual(self.document['editions'], ['english'])
        counts = script_counts(json.dumps(self.document, ensure_ascii=False))
        self.assertEqual(counts['han'] + counts['hangul'], 0)
        for f in self.fixtures:
            self.assertEqual(json.loads(f['user'].split(EVIDENCE_MARKER)[1]), f['expected']['evidence'])
            self.assertIn('do not browse, connect to any host, run commands', f['user'])
            self.assertEqual(f['role'], 'mainagent')
            self.assertEqual(f['output_type'], 'json')

if __name__ == '__main__':
    unittest.main()
