#!/usr/bin/env python3
"""Regression tests for the Win7 binary vulnerability gate."""
from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("verify_binary_govuln.py")


class VerifyBinaryGovulnTests(unittest.TestCase):
    def test_accepts_pretty_printed_json_event_stream(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            report = root / "report.json"
            allowlist = root / "allowlist.txt"
            report.write_text(
                "\n".join(
                    (
                        json.dumps({"config": {"scanner": "govulncheck"}}, indent=2),
                        json.dumps(
                            {"finding": {"osv": "GO-2026-5024"}},
                            indent=2,
                        ),
                    )
                ),
                encoding="utf-8",
            )
            allowlist.write_text("GO-2026-5024\n", encoding="utf-8")

            result = subprocess.run(
                [sys.executable, str(SCRIPT), str(report), str(allowlist)],
                check=False,
                capture_output=True,
                text=True,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("1 observed IDs reviewed", result.stdout)

    def test_rejects_unreviewed_id_in_pretty_printed_event_stream(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            report = root / "report.json"
            allowlist = root / "allowlist.txt"
            report.write_text(
                json.dumps(
                    {"finding": {"osv": "GO-2026-9999"}},
                    indent=2,
                ),
                encoding="utf-8",
            )
            allowlist.write_text("GO-2026-5024\n", encoding="utf-8")

            result = subprocess.run(
                [sys.executable, str(SCRIPT), str(report), str(allowlist)],
                check=False,
                capture_output=True,
                text=True,
            )

            self.assertEqual(result.returncode, 1, result.stderr)
            self.assertIn("new unreviewed Win7 binary vulnerabilities", result.stderr)


if __name__ == "__main__":
    unittest.main()
