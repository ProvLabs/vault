#!/usr/bin/env bash
set -euo pipefail

if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "In a git repository: $(git rev-parse --show-toplevel)"
    exit 0
else
    echo "Not in a git repository." >&2
    exit 1
fi
