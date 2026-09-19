#!/usr/bin/env python3
"""Import Octop's builtin sub-agent role library into config/subagents/library.

Copies the zh/ and en/ trees from the Octop repository verbatim (byte for
byte, no content transformation — frontmatter stays Octop-native, CJK
quirks included), with one deliberate exception:

    zh/supply-chain/supply-chain-strategist.md is SKIPPED.

The scanner (internal/agent/subagents) rejects a slug — the filename stem —
that appears in more than one division of the same locale, and Octop ships
supply-chain-strategist under BOTH zh/specialized/ and zh/supply-chain/
with different content. en/ ships it only under specialized/, so the
specialized/ zh copy is kept and the supply-chain/ duplicate is dropped.

The import is a mirror: re-running converges the destination to the source
(minus the skip) — files removed upstream disappear here too. Every copied
file is verified byte-identical to its source before the script succeeds.
"""

from __future__ import annotations

import argparse
import hashlib
import shutil
import sys
from pathlib import Path

# Library files (relative to the library root, POSIX separators) that are
# never imported. See the module docstring for the duplicate-slug rationale.
SKIP = {"zh/supply-chain/supply-chain-strategist.md"}

LOCALES = ("zh", "en")

# Octop repository (read-only source of truth).
DEFAULT_SOURCE = Path(
    "/Users/wuyongjun/trea/Octop/src/octop/infra/agents/subagents/library"
)

# Destination: <repo>/config/subagents/library, matching
# LoadBuiltinSubagents which scans <ConfigDir()>/subagents/library.
REPO_ROOT = Path(__file__).resolve().parent.parent
DEFAULT_DEST = REPO_ROOT / "config" / "subagents" / "library"


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def plan_files(source: Path) -> dict[str, Path]:
    """Map every importable source file (POSIX relpath -> path), minus SKIP."""
    files: dict[str, Path] = {}
    for locale in LOCALES:
        locale_dir = source / locale
        if not locale_dir.is_dir():
            raise SystemExit(f"error: source locale dir missing: {locale_dir}")
        for path in sorted(locale_dir.rglob("*")):
            if path.is_file():
                rel = path.relative_to(source).as_posix()
                if rel not in SKIP:
                    files[rel] = path
    return files


def import_library(source: Path, dest: Path) -> dict[str, object]:
    """Mirror source into dest (minus SKIP); return a summary."""
    files = plan_files(source)

    # Copy verbatim. copy2 preserves bytes (and mtime); overwriting makes
    # the run idempotent.
    for rel, src in files.items():
        target = dest / rel
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(src, target)

    # Drop the skip (and any stale file from an earlier import) so repeated
    # runs converge. Directories are never pruned: an emptied division dir
    # is legal for the scanner and keeps diff -r against the source clean.
    removed = []
    for rel in SKIP:
        target = dest / rel
        if target.exists():
            target.unlink()
            removed.append(rel)
    if dest.is_dir():
        for path in sorted(dest.rglob("*"), reverse=True):
            if path.is_file() and path.relative_to(dest).as_posix() not in files:
                path.unlink()
                removed.append(path.relative_to(dest).as_posix())

    # Verify: every destination file must be byte-identical to its source.
    mismatches = [
        rel
        for rel, src in files.items()
        if not (dest / rel).is_file() or sha256(dest / rel) != sha256(src)
    ]
    if mismatches:
        raise SystemExit(f"error: files differ from source: {mismatches}")

    md_counts = {
        locale: sum(
            1 for rel in files if rel.startswith(f"{locale}/") and rel.endswith(".md")
        )
        for locale in LOCALES
    }
    return {
        "files": files,
        "md_counts": md_counts,
        "removed": sorted(set(removed)),
        "divisions": sorted(
            p.name
            for p in (source / "zh").iterdir()
            if p.is_dir()
        ),
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--source", type=Path, default=DEFAULT_SOURCE,
                        help="Octop library root containing zh/ and en/")
    parser.add_argument("--dest", type=Path, default=DEFAULT_DEST,
                        help="destination library root (default: config/subagents/library)")
    opts = parser.parse_args()

    summary = import_library(opts.source, opts.dest)
    files: dict[str, Path] = summary["files"]
    md: dict[str, int] = summary["md_counts"]

    print(f"source: {opts.source}")
    print(f"dest:   {opts.dest}")
    print(f"copied: {len(files)} files total "
          f"(zh md: {md['zh']}, en md: {md['en']}, "
          f"plus {len(files) - md['zh'] - md['en']} divisions.json)")
    print(f"divisions: {len(summary['divisions'])} ({', '.join(summary['divisions'])})")
    print(f"skipped: {sorted(SKIP)}")
    if summary["removed"]:
        print(f"removed stale/skipped files from dest: {summary['removed']}")
    print("verified: all copied files byte-identical to source (sha256)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
