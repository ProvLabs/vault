#!/usr/bin/env python3
"""vault-lint-test: run the local verification chain.

Stops at the first failure. Prints a per-step status table.
"""

from __future__ import annotations

import argparse
import hashlib
import os
import re
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path

STEPS = [
    "format",
    "lint",
    "test-unit",
    "test-sim-simple",
    "test-sim-nondeterminism",
]

LABEL_WIDTH = 27


@dataclass
class StepResult:
    status: str = "PENDING"
    detail: str = ""


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Runs the vault verification chain in order: "
            "1 format, 2 lint, 3 test-unit, 4 test-sim-simple, 5 test-sim-nondeterminism. "
            "Stops at the first failure and prints a status table."
        ),
    )
    parser.add_argument("--fast", action="store_true", help="Skip the two simulation steps (4 and 5).")
    parser.add_argument(
        "--from",
        dest="start_at",
        type=int,
        default=1,
        metavar="N",
        help="Resume at step N (1..5). Earlier steps are reported SKIPPED.",
    )
    args = parser.parse_args()
    if args.start_at < 1 or args.start_at > 5:
        parser.error(f"--from must be 1..5, got: {args.start_at}")
    return args


def repo_toplevel() -> Path:
    try:
        out = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            check=True,
            capture_output=True,
            text=True,
        ).stdout.strip()
    except (subprocess.CalledProcessError, FileNotFoundError):
        print("Not in a git repository.", file=sys.stderr)
        sys.exit(2)
    return Path(out)


def keeper_tree_hash(root: Path) -> str:
    keeper = root / "keeper"
    if not keeper.is_dir():
        return ""
    parts = []
    for path in sorted(keeper.rglob("*.go")):
        if not path.is_file():
            continue
        h = hashlib.sha1()
        with path.open("rb") as f:
            for chunk in iter(lambda: f.read(65536), b""):
                h.update(chunk)
        parts.append(f"{h.hexdigest()}  {path.relative_to(root)}")
    outer = hashlib.sha1("\n".join(parts).encode()).hexdigest()
    return outer


def extract_failure(target: str, log: Path) -> str:
    try:
        text = log.read_text(errors="replace")
    except OSError:
        return "(no log)"

    if target == "lint":
        for line in text.splitlines():
            if re.match(r"^\S+\.go:\d+:\d+: ", line):
                return line
        tail = text.splitlines()[-3:]
        return " ".join(tail)

    if target == "test-unit":
        match = re.search(r"^--- FAIL:\s+(\S+)", text, re.MULTILINE)
        if match:
            return match.group(1)
        return "(unknown — see log)"

    if target in ("test-sim-simple", "test-sim-nondeterminism"):
        seed = _first_int(re.search(r"seed\s*[:=]\s*(\d+)", text, re.IGNORECASE))
        block = _first_int(
            re.search(r"(?:block[ _]?height|height)\s*[:=]\s*(\d+)", text, re.IGNORECASE)
        )
        return f"seed={seed or '?'}, block={block or '?'}"

    tail = text.splitlines()[-3:]
    return " ".join(tail)


def _first_int(match: re.Match | None) -> str | None:
    return match.group(1) if match else None


def run_step(
    idx: int,
    target: str,
    start_at: int,
    fast: bool,
    log_dir: Path,
    root: Path,
    results: dict[str, StepResult],
) -> bool:
    log = log_dir / f"{target}.log"
    result = results[target]

    if idx < start_at:
        result.status = "SKIPPED"
        result.detail = f"--from={start_at}"
        return True
    if fast and idx >= 4:
        result.status = "SKIPPED"
        result.detail = "--fast"
        return True

    print(f"→ make {target}  (log: {log})")
    before = keeper_tree_hash(root) if target == "format" else ""

    with log.open("w") as f:
        proc = subprocess.run(["make", target], stdout=f, stderr=subprocess.STDOUT)

    if proc.returncode != 0:
        result.status = "FAIL"
        result.detail = extract_failure(target, log)
        return False

    if target == "format":
        after = keeper_tree_hash(root)
        if before and before != after:
            result.status = "MODIFIED"
            result.detail = "files reformatted in keeper/"
        else:
            result.status = "PASS"
            result.detail = "no changes needed"
    elif target == "test-unit":
        text = log.read_text(errors="replace")
        cov_match = list(re.finditer(r"^total:\s+\S+\s+(\S+)$", text, re.MULTILINE))
        cov = cov_match[-1].group(1) if cov_match else "?"
        tests = sum(1 for _ in re.finditer(r"^--- PASS:", text, re.MULTILINE))
        result.status = "PASS"
        result.detail = f"{tests} tests, coverage {cov}"
    else:
        result.status = "PASS"
        result.detail = ""
    return True


def main() -> int:
    args = parse_args()

    root = repo_toplevel()
    os.chdir(root)

    log_dir = Path(os.environ.get("TMPDIR", "/tmp")) / "vault-lint-test"
    log_dir.mkdir(parents=True, exist_ok=True)

    results: dict[str, StepResult] = {s: StepResult() for s in STEPS}

    overall = 0
    for i, step in enumerate(STEPS):
        idx = i + 1
        ok = run_step(idx, step, args.start_at, args.fast, log_dir, root, results)
        if not ok:
            overall = 1
            for later in STEPS[i + 1:]:
                results[later].status = "SKIPPED"
                results[later].detail = "prior step failed"
            break

    print()
    for step in STEPS:
        r = results[step]
        label = f"make {step:<{LABEL_WIDTH}}"
        if r.detail:
            print(f"{label} : {r.status:<9} — {r.detail}")
        else:
            print(f"{label} : {r.status}")

    if overall != 0:
        print()
        print(f"Logs:      {log_dir}")
        failed = next((s for s in STEPS if results[s].status == "FAIL"), None)
        if failed:
            print(f"Re-run:    make {failed}")

    return overall


if __name__ == "__main__":
    sys.exit(main())
