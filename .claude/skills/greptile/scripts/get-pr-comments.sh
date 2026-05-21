#!/usr/bin/env bash
set -euo pipefail

usage() {
    cat >&2 <<EOF
Usage: $(basename "$0") <pr-number> [--user <login>] [--unresolved-only] [--repo <owner/repo>]

Fetches PR review-thread conversations (with resolved/outdated status).
Outputs JSON to stdout.

Options:
  --user <login>        Only include threads with at least one comment
                        authored by <login> (e.g. greptile-apps[bot]).
  --unresolved-only     Only include threads where isResolved=false.
  --repo <owner/repo>   Override the repo (defaults to the current git remote).
EOF
    exit 1
}

pr=""
user=""
unresolved_only=0
repo=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --user) user="$2"; shift 2 ;;
        --unresolved-only) unresolved_only=1; shift ;;
        --repo) repo="$2"; shift 2 ;;
        -h|--help) usage ;;
        *)
            if [[ -z "${pr}" ]]; then
                pr="$1"; shift
            else
                echo "Unexpected argument: $1" >&2; usage
            fi
            ;;
    esac
done

[[ -z "${pr}" ]] && usage

if [[ -z "${repo}" ]]; then
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    repo="$("${script_dir}/get-git-remote.sh")"
fi

owner="${repo%%/*}"
name="${repo##*/}"

threads_json="$(gh api graphql \
    -f query='
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
                    nodes{ author{login} body createdAt url }
                  }
                }
              }
            }
          }
        }' \
    -F owner="${owner}" -F repo="${name}" -F pr="${pr}")"

jq \
    --arg user "${user}" \
    --argjson unresolved_only "${unresolved_only}" \
    '
    .data.repository.pullRequest.reviewThreads.nodes
    | map({
        isResolved, isOutdated, path, line,
        comments: (.comments.nodes | map({author: .author.login, body, createdAt, url}))
      })
    | if $user != ""
        then map(select(.comments | any(.author == $user)))
        else . end
    | if $unresolved_only == 1
        then map(select(.isResolved == false))
        else . end
    ' <<<"${threads_json}"
