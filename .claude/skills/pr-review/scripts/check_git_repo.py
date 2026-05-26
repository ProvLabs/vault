#!/usr/bin/env python3
"""Check whether the current working directory is inside a git repository.

Prints the repository toplevel path on success, an error to stderr on failure.
Exit 0 inside a repo, 1 otherwise.
"""

from __future__ import annotations

import subprocess
import sys


def main() -> int:
    try:
        toplevel = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            check=True,
            capture_output=True,
            text=True,
        ).stdout.strip()
    except (subprocess.CalledProcessError, FileNotFoundError):
        print("Not in a git repository.", file=sys.stderr)
        return 1

    print(f"In a git repository: {toplevel}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
