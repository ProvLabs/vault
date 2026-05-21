#!/usr/bin/env bash
set -euo pipefail

remote="${1:-origin}"

url="$(git config --get "remote.${remote}.url" || true)"
if [[ -z "${url}" ]]; then
    echo "No URL found for remote '${remote}'." >&2
    exit 1
fi

# Normalize to owner/repo for use with `gh`.
# Handles: git@github.com:owner/repo(.git), https://github.com/owner/repo(.git), ssh://git@github.com/owner/repo(.git)
slug="${url#*github.com[:/]}"
slug="${slug%.git}"

if [[ -z "${slug}" || "${slug}" == "${url}" ]]; then
    echo "Could not parse owner/repo from URL: ${url}" >&2
    exit 1
fi

echo "${slug}"
