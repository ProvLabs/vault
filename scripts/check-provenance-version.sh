#!/usr/bin/env bash
# check-provenance-version.sh
#
# Compares the github.com/provenance-io/provenance version pinned in go.mod
# against the upstream provenance-io/provenance main branch (and latest release)
# and reports how far behind the pin is ("drift").
#
# Usage: check-provenance-version.sh [--go-mod <path>] [--report-file <path>]
#                                    [--exit-code] [-h|--help]
#
#   --go-mod <path>       go.mod to inspect (default: repo-root go.mod).
#   --report-file <path>  Also write the human-readable report to this file.
#   --exit-code           Exit 1 when drift is detected (default: always exit 0).
#
# Environment:
#   GITHUB_TOKEN   Optional. Used to authenticate GitHub API calls (higher rate
#                  limits). In GitHub Actions, pass ${{ github.token }}.
#   GITHUB_OUTPUT  When set (GitHub Actions), key=value results are appended:
#                  drift, behind, pinned_version, pinned_commit,
#                  upstream_commit, latest_release.
#
# Exit codes:
#   0  Ran successfully.
#   1  Drift detected and --exit-code was given.
#   2  Usage or runtime error.

set -euo pipefail

UPSTREAM_REPO="provenance-io/provenance"
MODULE_PATH="github.com/provenance-io/provenance"
GOMOD=""
REPORT_FILE=""
EXIT_ON_DRIFT=false

err() { printf '%s\n' "$*" >&2; }

while [[ $# -gt 0 ]]; do
	case "$1" in
		-h|--help)
			sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'
			exit 0
			;;
		--go-mod)
			[[ -n "${2:-}" ]] || { err "Missing argument after $1"; exit 2; }
			GOMOD="$2"; shift 2;;
		--report-file)
			[[ -n "${2:-}" ]] || { err "Missing argument after $1"; exit 2; }
			REPORT_FILE="$2"; shift 2;;
		--exit-code)
			EXIT_ON_DRIFT=true; shift;;
		*)
			err "Unknown argument: $1"; exit 2;;
	esac
done

if ! command -v curl >/dev/null 2>&1; then
	err "ERROR: curl is required."; exit 2
fi
if ! command -v python3 >/dev/null 2>&1; then
	err "ERROR: python3 is required."; exit 2
fi

if [[ -z "$GOMOD" ]]; then
	repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
	GOMOD="${repo_root}/go.mod"
fi
if [[ ! -f "$GOMOD" ]]; then
	err "ERROR: go.mod not found: $GOMOD"; exit 2
fi

# The actual pin lives in a `replace ... => ... <version>` line when present
# (the require line is only the nominal MVS version). Fall back to require.
pinned_version="$(awk -v m="$MODULE_PATH" '$0 ~ m && /=>/ {print $NF}' "$GOMOD" | tail -1)"
if [[ -z "$pinned_version" ]]; then
	pinned_version="$(awk -v m="$MODULE_PATH" '$1 == m {print $2}' "$GOMOD" | tail -1)"
fi
if [[ -z "$pinned_version" ]]; then
	err "ERROR: could not find $MODULE_PATH in $GOMOD"; exit 2
fi

# A Go pseudo-version ends with -<12 hex chars> (the commit). A plain tag does not.
pinned_commit=""
if [[ "$pinned_version" =~ -([0-9a-f]{12})$ ]]; then
	pinned_commit="${BASH_REMATCH[1]}"
	pinned_ref="$pinned_commit"
else
	pinned_ref="$pinned_version"
fi

gh_api() {
	local hdr=(-fsSL -H "Accept: application/vnd.github+json")
	if [[ -n "${GITHUB_TOKEN:-}" ]]; then
		hdr+=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
	fi
	curl "${hdr[@]}" "https://api.github.com/repos/${UPSTREAM_REPO}/$1"
}

jq_py() { python3 -c "import sys,json; d=json.load(sys.stdin); $1"; }

upstream_commit="$(gh_api "commits/main" | jq_py 'print(d["sha"])')" || {
	err "ERROR: failed to fetch upstream main commit."; exit 2
}
latest_release="$(gh_api "releases/latest" | jq_py 'print(d.get("tag_name",""))' || true)"

compare_json="$(gh_api "compare/${pinned_ref}...main" || true)"
if [[ -z "$compare_json" ]]; then
	err "ERROR: failed to compare ${pinned_ref}...main (commit missing upstream?)."
	exit 2
fi
behind="$(printf '%s' "$compare_json" | jq_py 'print(d.get("ahead_by",0))')"
status="$(printf '%s' "$compare_json" | jq_py 'print(d.get("status",""))')"
commit_list="$(printf '%s' "$compare_json" | jq_py '
[print("- " + ((c["commit"]["message"] or "").splitlines() or ["(no message)"])[0]) for c in d.get("commits",[])[-25:]]' || true)"

drift=false
if [[ "$status" != "identical" && "${behind:-0}" -gt 0 ]]; then
	drift=true
fi

build_report() {
	echo "## Provenance dependency drift check"
	echo
	echo "- Module: \`${MODULE_PATH}\`"
	echo "- Pinned version: \`${pinned_version}\`"
	if [[ -n "$pinned_commit" ]]; then
		echo "- Pinned commit: \`${pinned_commit}\`"
	fi
	echo "- Upstream \`main\` HEAD: \`${upstream_commit:0:12}\`"
	echo "- Latest upstream release: \`${latest_release:-unknown}\`"
	echo
	if [[ "$drift" == true ]]; then
		echo "**Drift detected: the pin is ${behind} commit(s) behind \`${UPSTREAM_REPO}\` main.**"
		echo
		echo "Commits on \`main\` not in the current pin (up to 25 shown of ${behind}):"
		echo
		echo "${commit_list}"
		echo
		echo "Run the \`provenance-drift\` Claude Code agent for a breaking-change summary,"
		echo "or see https://github.com/${UPSTREAM_REPO}/compare/${pinned_ref}...main"
	else
		echo "**Up to date: the pin matches \`${UPSTREAM_REPO}\` main.**"
	fi
}

report="$(build_report)"
printf '%s\n' "$report"

if [[ -n "$REPORT_FILE" ]]; then
	printf '%s\n' "$report" > "$REPORT_FILE"
fi

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
	{
		echo "drift=${drift}"
		echo "behind=${behind:-0}"
		echo "pinned_version=${pinned_version}"
		echo "pinned_commit=${pinned_commit}"
		echo "upstream_commit=${upstream_commit}"
		echo "latest_release=${latest_release}"
	} >> "$GITHUB_OUTPUT"
fi

if [[ "$drift" == true && "$EXIT_ON_DRIFT" == true ]]; then
	exit 1
fi
exit 0
