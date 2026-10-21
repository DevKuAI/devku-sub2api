import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


class AuditReportTest(unittest.TestCase):
    def check_report(self, report):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'audit.json').write_text(json.dumps(report))
            (root / 'exceptions.yml').write_text('version: 1\nexceptions:\n')
            return subprocess.run(
                [sys.executable, str(Path(__file__).with_name('check_pnpm_audit_exceptions.py')),
                 '--audit', str(root / 'audit.json'), '--exceptions', str(root / 'exceptions.yml')],
                capture_output=True, text=True, check=False,
            )

    def test_rejects_registry_error_even_with_empty_advisories(self):
        result = self.check_report({'error': {'code': 'ERR_PNPM_AUDIT_ENDPOINT_NOT_EXISTS'}, 'advisories': {}})
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Audit failed', result.stderr)

    def test_rejects_missing_report(self):
        for report in ({}, [], {'advisories': []}):
            with self.subTest(report=report):
                self.assertNotEqual(self.check_report(report).returncode, 0)

    def test_accepts_clean_supported_reports(self):
        for key in ('advisories', 'vulnerabilities'):
            with self.subTest(key=key):
                self.assertEqual(self.check_report({key: {}}).returncode, 0)

    def test_rejects_unexcepted_high_advisory(self):
        result = self.check_report({'advisories': {'1': {
            'module_name': 'follow-redirects', 'severity': 'high',
            'github_advisory_id': 'GHSA-q3jh-wrc6-jgq8',
        }}})
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('GHSA-q3jh-wrc6-jgq8', result.stderr)


if __name__ == '__main__':
    unittest.main()
