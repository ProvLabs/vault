#!/usr/bin/env python3
"""pr-review: fetch all unresolved PR review threads, group by author, render markdown.

Known bots (CodeRabbit, Greptile) are listed first; other bots, then humans alphabetical.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from collections import defaultdict
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Fetch all unresolved review-thread discussions on a PR and group "
            "them by the thread originator."
        ),
    )
    parser.add_argument("pr", type=int, help="PR number.")
    parser.add_argument(
        "--repo",
        default="",
        help="Override the repo (defaults to the current git remote).",
    )
    return parser.parse_args()


def resolve_repo(script_dir: Path) -> str:
    result = subprocess.run(
        [sys.executable, str(script_dir / "get_git_remote.py")],
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout.strip()


def fetch_unresolved_threads(script_dir: Path, pr: int, repo: str) -> list[dict]:
    result = subprocess.run(
        [
            sys.executable,
            str(script_dir / "get_pr_comments.py"),
            str(pr),
            "--unresolved-only",
            "--repo", repo,
        ],
        check=True,
        capture_output=True,
        text=True,
    )
    return json.loads(result.stdout)


def normalize_author(login: str) -> str:
    if re.match(r"^coderabbit", login, re.IGNORECASE):
        return "CodeRabbit"
    if re.match(r"^greptile", login, re.IGNORECASE):
        return "Greptile"
    return login


def author_priority(author: str) -> int:
    if author == "CodeRabbit":
        return 0
    if author == "Greptile":
        return 1
    if author.endswith("[bot]"):
        return 2
    return 3


_PREAMBLE_RE = re.compile(r"^_[^_\n]+_(?:\s*\|\s*_[^_\n]+_)*\s*\n+")
_GREPTILE_ANCHOR_RE = re.compile(r"^<a[^>]*>.*?</a>\s*\n+", re.DOTALL)
_BOLD_RE = re.compile(r"\*\*([^*]+)\*\*")


def strip_preamble(body: str) -> str:
    body = _PREAMBLE_RE.sub("", body)
    body = _GREPTILE_ANCHOR_RE.sub("", body)
    return body


def extract_title(body: str) -> str:
    match = _BOLD_RE.search(body)
    if match:
        return match.group(1)
    first_line = body.split("\n", 1)[0]
    return first_line[:160]


def extract_prose(body: str) -> str:
    match = _BOLD_RE.search(body)
    if match:
        # Mirror jq's `\s*\n*` trim after the closing `**`.
        prose = body[match.end():].lstrip()
    else:
        parts = body.split("\n", 1)
        prose = parts[1] if len(parts) == 2 else ""
        prose = prose.lstrip("\n")
    return prose.rstrip()


def render_thread(thread: dict) -> str:
    clean = strip_preamble(thread["body"])
    title = extract_title(clean)
    prose = extract_prose(clean)
    lineref = f" (line {thread['line']})" if thread.get("line") is not None else ""
    out = f"- **{title}**{lineref}\n"
    if prose:
        indented = "\n".join("  " + line for line in prose.split("\n"))
        out += indented + "\n"
    return out


def render(threads: list[dict]) -> tuple[str, int, int]:
    """Return (markdown_body, thread_count, distinct_author_count)."""
    if not threads:
        return "", 0, 0

    normalized = []
    for t in threads:
        if not t.get("comments"):
            continue
        first = t["comments"][0]
        normalized.append({
            "author": normalize_author(first["author"] or ""),
            "path": t["path"],
            "line": t["line"],
            "body": first["body"] or "",
        })

    by_author: dict[str, list[dict]] = defaultdict(list)
    for n in normalized:
        by_author[n["author"]].append(n)

    sections = []
    for author in sorted(by_author.keys(), key=lambda a: (author_priority(a), a.lower())):
        author_threads = sorted(
            by_author[author],
            key=lambda t: (t["path"] or "", t["line"] if t["line"] is not None else 999_999),
        )

        by_path: dict[str, list[dict]] = defaultdict(list)
        for at in author_threads:
            by_path[at["path"]].append(at)

        section = f"### {author} ({len(author_threads)} unresolved)\n"
        for path in by_path:
            section += f"\n**`{path}`**\n"
            section += "".join(render_thread(t) for t in by_path[path])
        sections.append(section)

    body = "\n".join(sections)
    distinct_authors = len(by_author)
    return body, len(normalized), distinct_authors


def status_line(count: int, author_count: int) -> str:
    if count == 0:
        return "🟢 No open PR discussions."
    reviewer_word = "reviewer" if author_count == 1 else "reviewers"
    discussion_word = "discussion" if count == 1 else "discussions"
    if count <= 3:
        return f"🟡 {count} open {discussion_word} across {author_count} {reviewer_word}."
    return f"🔴 {count} open discussions across {author_count} {reviewer_word}."


def main() -> int:
    args = parse_args()
    script_dir = Path(__file__).resolve().parent

    repo = args.repo or resolve_repo(script_dir)
    threads = fetch_unresolved_threads(script_dir, args.pr, repo)

    if not threads:
        print("🟢 No open PR discussions.")
        return 0

    body, count, author_count = render(threads)
    print(body)
    print()
    print(status_line(count, author_count))
    return 0


if __name__ == "__main__":
    sys.exit(main())
