#!/usr/bin/env python3
"""Generate build module reference documentation.

This script imports the MODULES list from build.py and generates
a comprehensive reference document for all build modules.

Usage:
    python3 tools/generate_build_reference.py
"""

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))

from build import MODULES


def generate_markdown() -> str:
    """Generate markdown documentation for all build modules."""
    lines = [
        "# Build Modules Reference",
        "",
        "This document is auto-generated. Do not edit manually.",
        "Run `python3 tools/generate_build_reference.py` to regenerate.",
        "",
        "## Modules",
        "",
        "| Module | Language | Directory | Build Command | Clean Command | Build Artifacts |",
        "|--------|----------|-----------|---------------|---------------|-----------------|",
    ]

    for module in MODULES:
        build_cmd = " ".join(module.build_cmd)
        clean_cmd = " ".join(module.clean_cmd)
        build_dir = str(module.build_dir.relative_to(ROOT)) if module.build_dir else "N/A"
        dir_str = str(module.dir.relative_to(ROOT))
        lines.append(
            f"| {module.name} | {module.language} | `{dir_str}` | `{build_cmd}` | `{clean_cmd}` | `{build_dir}` |"
        )

    lines.extend([
        "",
        "## Usage",
        "",
        "### Building a specific module",
        "",
        "```bash",
        "python3 build.py --module <module-name>",
        "```",
        "",
        "### Cleaning a specific module",
        "",
        "```bash",
        "python3 build.py --module <module-name> --clean",
        "```",
        "",
        "### Release build (all modules)",
        "",
        "```bash",
        "python3 build.py --release",
        "```",
        "",
        "### List available modules",
        "",
        "```bash",
        "python3 build.py --list",
        "```",
    ])

    return "\n".join(lines) + "\n"


def main():
    output_path = ROOT / "docs" / "BUILD_MODULES.md"
    content = generate_markdown()
    output_path.write_text(content, encoding="utf-8")
    print(f"Generated {output_path}")


if __name__ == "__main__":
    main()
