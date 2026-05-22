#!/usr/bin/env python3
"""Fetch PR review-thread conversations via `gh api graphql`.

Outputs a JSON array of threads (with optional filtering) to stdout.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path

GRAPHQL_QUERY = """
query($owner:String!,$repo:String!,$pr:Int!){
  repository(owner:$owner,name:$repo){
    pullRequest(number:$pr){
      reviewThreads(first:100){
        nodes{
          isResolved
          isOutdated
          path
          line
          comments(first:50){
            nodes{ databaseId author{login} body createdAt url }
          }
        }
      }
    }
  }
}
"""


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Fetch PR review-thread conversations and output JSON.",
    )
    parser.add_argument("pr", type=int, help="PR number.")
    parser.add_argument(
        "--user",
        default="",
        help="Only include threads with at least one comment authored by <login>.",
    )
    parser.add_argument(
        "--unresolved-only",
        action="store_true",
        help="Only include threads where isResolved=false.",
    )
    parser.add_argument(
        "--repo",
        default="",
        help="Override the repo (defaults to the current git remote).",
    )
    return parser.parse_args()


def resolve_repo() -> str:
    script_dir = Path(__file__).resolve().parent
    result = subprocess.run(
        [sys.executable, str(script_dir / "get_git_remote.py")],
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout.strip()


def fetch_threads(owner: str, name: str, pr: int) -> dict:
    result = subprocess.run(
        [
            "gh", "api", "graphql",
            "-f", f"query={GRAPHQL_QUERY}",
            "-F", f"owner={owner}",
            "-F", f"repo={name}",
            "-F", f"pr={pr}",
        ],
        check=True,
        capture_output=True,
        text=True,
    )
    return json.loads(result.stdout)


def flatten_thread(node: dict) -> dict:
    return {
        "isResolved": node.get("isResolved"),
        "isOutdated": node.get("isOutdated"),
        "path": node.get("path"),
        "line": node.get("line"),
        "comments": [
            {
                "author": (c.get("author") or {}).get("login"),
                "body": c.get("body"),
                "createdAt": c.get("createdAt"),
                "url": c.get("url"),
            }
            for c in (node.get("comments") or {}).get("nodes", [])
        ],
    }


def main() -> int:
    args = parse_args()

    repo = args.repo or resolve_repo()
    if "/" not in repo:
        print(f"Invalid repo slug: {repo}", file=sys.stderr)
        return 1
    owner, name = repo.split("/", 1)

    payload = fetch_threads(owner, name, args.pr)
    nodes = (
        payload.get("data", {})
        .get("repository", {})
        .get("pullRequest", {})
        .get("reviewThreads", {})
        .get("nodes", [])
    )

    threads = [flatten_thread(n) for n in nodes]

    if args.user:
        threads = [
            t for t in threads
            if any(c["author"] == args.user for c in t["comments"])
        ]
    if args.unresolved_only:
        threads = [t for t in threads if t["isResolved"] is False]

    json.dump(threads, sys.stdout, indent=2)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
