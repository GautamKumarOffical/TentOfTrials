"""Ensure React UI files remain functional components."""

from __future__ import annotations

import re
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
FRONTEND_SRC = ROOT / "frontend" / "src"

CLASS_COMPONENT_PATTERNS = (
    re.compile(r"extends\s+React\.Component"),
    re.compile(r"extends\s+Component\b"),
    re.compile(r"class\s+\w+\s+extends\s+PureComponent"),
)


class FunctionalReactComponentTests(unittest.TestCase):
    def test_no_class_components_in_frontend_src(self) -> None:
        offenders: list[str] = []
        for path in FRONTEND_SRC.rglob("*"):
            if path.suffix not in {".tsx", ".jsx"}:
                continue
            source = path.read_text(encoding="utf-8")
            for pattern in CLASS_COMPONENT_PATTERNS:
                if pattern.search(source):
                    offenders.append(str(path.relative_to(ROOT)))
        self.assertEqual(offenders, [])


if __name__ == "__main__":
    unittest.main()
