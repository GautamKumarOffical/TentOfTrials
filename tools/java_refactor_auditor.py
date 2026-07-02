#!/usr/bin/env python3
"""Lightweight Java refactor compliance checks for TentOfTrials."""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

FORBIDDEN_PATTERNS = (
    (re.compile(r"\bSystem\.out\.println\("), "use structured logging instead of System.out.println"),
    (re.compile(r"\bprintStackTrace\s*\("), "avoid printStackTrace in production code"),
    (re.compile(r"\b@SuppressWarnings\(\s*\"unchecked\"\s*\)"), "document why unchecked suppression is required"),
)


def audit_file(path: Path) -> list[str]:
    text = path.read_text(encoding="utf-8", errors="replace")
    issues: list[str] = []
    for index, line in enumerate(text.splitlines(), start=1):
        for pattern, message in FORBIDDEN_PATTERNS:
            if pattern.search(line):
                issues.append(f"{path}:{index}: {message}")
    return issues


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Audit Java sources for refactor compliance.")
    parser.add_argument("paths", nargs="*", default=["."], help="Files or directories to scan")
    args = parser.parse_args(argv)

    java_files: list[Path] = []
    for raw in args.paths:
        path = Path(raw)
        if path.is_dir():
            java_files.extend(sorted(path.rglob("*.java")))
        elif path.suffix == ".java" and path.is_file():
            java_files.append(path)

    if not java_files:
        print("No Java files found to audit")
        return 0

    issues: list[str] = []
    for path in java_files:
        issues.extend(audit_file(path))

    if issues:
        print("Java refactor compliance issues:")
        for issue in issues:
            print(f"- {issue}")
        return 1

    print(f"Java refactor compliance passed ({len(java_files)} files)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
