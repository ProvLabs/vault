#!/usr/bin/env python3
"""Resolve a GitHub `owner/repo` slug from a local git remote URL.

Defaults to `origin`. Accepts SSH (`git@github.com:owner/repo.git`),
HTTPS (`https://github.com/owner/repo.git`), and `ssh://` forms.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "remote",
        nargs="?",
        default="origin",
        help="Git remote name (default: origin).",
    )
    return parser.parse_args()


def get_remote_url(remote: str) -> str | None:
    try:
        result = subprocess.run(
            ["git", "config", "--get", f"remote.{remote}.url"],
            check=True,
            capture_output=True,
            text=True,
        )
    except (subprocess.CalledProcessError, FileNotFoundError):
        return None
    return result.stdout.strip() or None


def parse_slug(url: str) -> str | None:
    match = re.search(r"github\.com[:/](.+?)(?:\.git)?/?$", url)
    if not match:
        return None
    return match.group(1)


def main() -> int:
    args = parse_args()

    url = get_remote_url(args.remote)
    if not url:
        print(f"No URL found for remote '{args.remote}'.", file=sys.stderr)
        return 1

    slug = parse_slug(url)
    if not slug:
        print(f"Could not parse owner/repo from URL: {url}", file=sys.stderr)
        return 1

    print(slug)
    return 0


if __name__ == "__main__":
    sys.exit(main())
