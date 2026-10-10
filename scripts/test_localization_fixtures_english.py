"""English-only development fixtures; no provider or target calls."""
import collections
import hashlib
import json
from pathlib import Path
import re
import unittest
from urllib.parse import urlsplit
from test_localization_fixtures import EVIDENCE_MARKER, ROLES
from run_localization_eval import script_counts

class EnglishFixtureTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.root = Path(__file__).parent
        cls.document = json.loads((cls.root / 'localization_fixtures_english.json').read_text())
        cls.fixtures = cls.document['fixtures']

    def test_provenance_and_unchanged_history(self):
        self.assertIn('Assistant-authored', self.document['provenance'])
        self.assertIn('Not inherited upstream', self.document['provenance'])
        for name, digest in self.document['source_fixture_files'].items():
            self.assertEqual(hashlib.sha256((self.root / name).read_bytes()).hexdigest(), digest)
        historical = json.loads((self.root / 'localization_fixtures.json').read_text())
        ids = {f['id'] for f in historical['fixtures']}
        for f in self.fixtures:
            self.assertIn(f['source_fixture_id'], ids)
            self.assertNotIn(f['id'], ids)

    def test_role_coverage_and_english_owned_content(self):
        self.assertEqual(self.document['editions'], ['english'])
        self.assertEqual(len({f['id'] for f in self.fixtures}), 18)
        self.assertEqual({f['role'] for f in self.fixtures}, ROLES)
        self.assertEqual(collections.Counter(f['output_type'] for f in self.fixtures), {'json':14, 'tool_call':2, 'text':2})
        counts = script_counts(json.dumps(self.document, ensure_ascii=False))
        self.assertEqual(counts['han'] + counts['hangul'], 0)

    def test_exact_evidence_and_unambiguous_gold(self):
        cases = [f for f in self.fixtures if f['output_type'] == 'json']
        for f in cases:
            with self.subTest(case=f['id']):
                self.assertEqual(json.loads(f['user'].split(EVIDENCE_MARKER)[1]), f['expected']['evidence'])
                self.assertIn('Positive proposition to assess', f['user'])
                self.assertIn('Activity under review:', f['user'])
                self.assertIn('Allowed activity:', f['user'])
                self.assertEqual(f['expected']['exact_literals'], [e['id'] for e in f['expected']['evidence']])
        self.assertEqual(sum(f['expected']['scope_status']=='out_of_scope' for f in cases), 3)
        self.assertGreaterEqual(sum(f['expected']['assessment']=='inconclusive' for f in cases), 4)
        self.assertTrue(any('\r\n' in e['raw'] for f in cases for e in f['expected']['evidence']))

    def test_hosts_remain_synthetic_and_wrapup_contracts_clear(self):
        for f in self.fixtures:
            for url in re.findall(r'https?://[^\s"\\<>]+', f['user']):
                host = urlsplit(url).hostname
                self.assertTrue(host == '127.0.0.1' or host.endswith('.invalid'))
            if f['output_type'] == 'text':
                self.assertEqual(f['expected']['sentence_count'], 1)
                self.assertIn('no remaining action', f['user'])
                for literal in f['expected']['exact_literals']:
                    self.assertIn(literal, f['user'])

if __name__ == '__main__':
    unittest.main()
