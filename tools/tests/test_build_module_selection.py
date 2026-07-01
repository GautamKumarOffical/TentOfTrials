"""Tests for build.py module selection helpers."""

from __future__ import annotations

import importlib.util
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BUILD_PATH = ROOT / "build.py"


def load_build_module():
    spec = importlib.util.spec_from_file_location("tot_build", BUILD_PATH)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    sys.modules["tot_build"] = module
    spec.loader.exec_module(module)
    return module


class BuildModuleSelectionTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.build = load_build_module()

    def test_parse_module_names_allows_spaces(self) -> None:
        names = self.build.parse_module_names("backend, frontend ,market")
        self.assertEqual(names, ["backend", "frontend", "market"])

    def test_validate_module_names_returns_unknown(self) -> None:
        selected, unknown = self.build.validate_module_names(["backend", "not-real"])
        self.assertEqual(unknown, ["not-real"])
        self.assertEqual([module.name for module in selected], ["backend"])

    def test_select_modules_rejects_invalid_names(self) -> None:
        selected, code = self.build.select_modules("backend, fake-module")
        self.assertEqual(code, 1)
        self.assertEqual(selected, [])


if __name__ == "__main__":
    unittest.main()
