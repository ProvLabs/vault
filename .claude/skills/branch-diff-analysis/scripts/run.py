#!/usr/bin/env python3
"""branch-diff-analysis: scope work to the current branch's diff against a base ref.

Outputs a structured report: file categories with line counts, source-vs-test
pairing check, and function-level summary from diff hunk headers.
"""

from __future__ import annotations

import argparse
import fnmatch
import os
import re
import subprocess
import sys
from pathlib import Path

CATEGORIES = [
    "Go source",
    "Go tests",
    "Protobuf",
    "Generated",
    "Config / build",
    "Docs",
    "Other",
]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--base",
        default="main",
        help="Base ref to diff against. Default: main.",
    )
    return parser.parse_args()


def run_git(*args: str) -> str:
    return subprocess.run(
        ["git", *args],
        check=True,
        capture_output=True,
        text=True,
    ).stdout


def repo_toplevel() -> Path:
    try:
        return Path(run_git("rev-parse", "--show-toplevel").strip())
    except subprocess.CalledProcessError:
        print("Not in a git repository.", file=sys.stderr)
        sys.exit(2)


def verify_base(base: str) -> None:
    proc = subprocess.run(
        ["git", "rev-parse", "--verify", base],
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        print(f"Base ref not found locally: {base}", file=sys.stderr)
        print(f"Try: git fetch origin {base}:{base}", file=sys.stderr)
        sys.exit(2)


_CONFIG_PATTERNS = [
    "Makefile",
    "*.mk",
    "go.mod",
    "go.sum",
    ".gitignore",
    ".golangci.yml",
    ".golangci.yaml",
    ".github/workflows/*",
    ".claude/settings*.json",
]

_DOCS_PATTERNS = [
    "*.md",
    "spec/*",
    "docs/*",
]

_GENERATED_PATTERNS = [
    "*.pb.go",
    "*.pulsar.go",
    "*.pb.gw.go",
    "client/docs/statik/statik.go",
    "docs/static/openapi.yml",
]


def _matches_any(path: str, patterns: list[str]) -> bool:
    return any(fnmatch.fnmatchcase(path, p) for p in patterns)


def categorize(path: str) -> str:
    if _matches_any(path, _GENERATED_PATTERNS):
        return "Generated"
    if path.endswith("_test.go"):
        return "Go tests"
    if path.endswith(".go"):
        return "Go source"
    if path.endswith(".proto"):
        return "Protobuf"
    if _matches_any(path, _CONFIG_PATTERNS):
        return "Config / build"
    if _matches_any(path, _DOCS_PATTERNS):
        return "Docs"
    return "Other"


def parse_numstat(numstat: str) -> dict[str, str]:
    stats: dict[str, str] = {}
    for line in numstat.splitlines():
        parts = line.split("\t")
        if len(parts) < 3:
            continue
        added, removed, path = parts[0], parts[1], parts[2]
        stats[path] = f"+{added} / -{removed}"
    return stats


def print_category(title: str, files: list[str], stats: dict[str, str]) -> None:
    if not files:
        return
    print()
    print(f"## {title} ({len(files)})")
    for f in files:
        stat = stats.get(f, "—")
        print(f"  {f:<60}  {stat}")


_HUNK_FUNC_RE = re.compile(r"^@@.*@@.*func ")
_DIFF_HEADER_RE = re.compile(r"^diff --git a/(.+?) b/")


def extract_changed_functions(base: str) -> list[str]:
    diff = run_git("diff", f"{base}...HEAD", "--unified=0", "--", "*.go")
    seen: set[str] = set()
    current_file = ""
    for line in diff.splitlines():
        header = _DIFF_HEADER_RE.match(line)
        if header:
            current_file = header.group(1)
            continue
        if _HUNK_FUNC_RE.match(line):
            idx = line.find("func ")
            sig = line[idx:]
            brace = sig.find("{")
            if brace != -1:
                sig = sig[:brace]
            sig = sig.rstrip()
            seen.add(f"{current_file} :: {sig}")
    return sorted(seen)


def main() -> int:
    args = parse_args()

    root = repo_toplevel()
    os.chdir(root)

    verify_base(args.base)

    current = run_git("rev-parse", "--abbrev-ref", "HEAD").strip()
    merge_base = run_git("merge-base", args.base, "HEAD").strip()
    files_raw = run_git("diff", f"{args.base}...HEAD", "--name-only").strip()
    if not files_raw:
        print(f"No changes between {args.base} and HEAD.")
        return 0

    files = files_raw.splitlines()
    numstat = parse_numstat(run_git("diff", f"{args.base}...HEAD", "--numstat"))

    buckets: dict[str, list[str]] = {c: [] for c in CATEGORIES}
    for f in files:
        buckets[categorize(f)].append(f)

    print(f"# Branch diff: {current} vs {args.base}")
    print(f"Merge base:    {merge_base}")
    print(f"Files changed: {len(files)}")

    for cat in CATEGORIES:
        print_category(cat, buckets[cat], numstat)

    go_src = buckets["Go source"]
    go_test = buckets["Go tests"]

    if go_src:
        print()
        print("## Test coverage cross-check")
        test_dirs = {str(Path(t).parent) for t in go_test}
        untested = [s for s in go_src if str(Path(s).parent) not in test_dirs]
        if not untested:
            print("  All changed Go source files have a sibling *_test.go modification.")
        else:
            print("  Changed Go source files without a matching test change in the same package:")
            for f in untested:
                print(f"    - {f}")

    if go_src:
        print()
        print("## Changed functions (from diff hunk headers)")
        for entry in extract_changed_functions(args.base):
            print(entry)

    return 0


if __name__ == "__main__":
    sys.exit(main())
