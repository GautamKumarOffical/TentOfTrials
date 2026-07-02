from __future__ import annotations

import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def test_java_refactor_auditor_flags_println(tmp_path) -> None:
    java = tmp_path / "Sample.java"
    java.write_text("class Sample { void run() { System.out.println(\"x\"); } }\n", encoding="utf-8")
    result = subprocess.run(
        [sys.executable, str(ROOT / "tools" / "java_refactor_auditor.py"), str(java)],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 1
    assert "System.out.println" in result.stdout


def test_java_refactor_auditor_passes_clean_file(tmp_path) -> None:
    java = tmp_path / "Clean.java"
    java.write_text("class Clean { int value() { return 1; } }\n", encoding="utf-8")
    result = subprocess.run(
        [sys.executable, str(ROOT / "tools" / "java_refactor_auditor.py"), str(java)],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0
    assert "passed" in result.stdout
