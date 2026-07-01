"""Tests for build.py structured timing helpers."""

from __future__ import annotations

import importlib.util
import json
import sys
import tempfile
import unittest
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BUILD_PATH = ROOT / "build.py"


def load_build_module():
    spec = importlib.util.spec_from_file_location("tot_build_timings", BUILD_PATH)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    sys.modules["tot_build_timings"] = module
    spec.loader.exec_module(module)
    return module


class BuildTimingTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.build = load_build_module()
        cls.module = cls.build.MODULES[0]

    def test_build_module_timing_entry_fields(self) -> None:
        started = datetime(2026, 6, 29, 12, 0, 0, tzinfo=timezone.utc)
        finished = datetime(2026, 6, 29, 12, 0, 2, tzinfo=timezone.utc)
        entry = self.build.build_module_timing_entry(
            self.module,
            started_at=started,
            finished_at=finished,
            elapsed=2.0,
            success=True,
            command=["cargo", "build"],
        )
        self.assertEqual(entry["module"], self.module.name)
        self.assertEqual(entry["status"], "PASS")
        self.assertEqual(entry["exit_code"], 0)
        self.assertEqual(entry["command"], ["cargo", "build"])

    def test_write_timings_json(self) -> None:
        timings = [{"module": "compliance", "elapsed_seconds": 1.2, "status": "PASS"}]
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "timings.json"
            self.build.write_timings_json(path, timings)
            payload = json.loads(path.read_text(encoding="utf-8"))
            self.assertEqual(payload["module_timings"], timings)


if __name__ == "__main__":
    unittest.main()
