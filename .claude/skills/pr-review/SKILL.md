---
name: pr-review
description: Summarize a PR's open review discussions, grouped by the person who started each thread. Known bots (CodeRabbit, Greptile) appear first, then humans alphabetically. Use when the user asks to see, list, summarize, or triage PR review feedback, or runs `/pr-review <PR#>`.
version: 0.1.0
---

# PR Review Summary

Fetches all **unresolved** review-thread conversations on a pull request and groups them by the thread originator. Output is markdown, one section per reviewer.

Section order:
1. **CodeRabbit** (if present)
2. **Greptile** (if present)
3. **Other bots** — alphabetical
4. **Humans** — alphabetical

Within each section, threads are grouped by file (sorted alphabetically) then by line number ascending (nulls last).

## Invocation

```bash
python3 ${CLAUDE_PLUGIN_ROOT:-.claude/skills/pr-review}/scripts/run.py <PR#> [--repo <owner/repo>]
```

If the user didn't give a PR number, ask before running. The `--repo` flag is only needed when the current directory isn't a clone of the PR's repo.

## Prerequisites

- **`gh`** — authenticated via `gh auth login`. The skill uses `gh api graphql`.
- **`python3`** — standard library only; used for filtering, grouping, and rendering.
- **`git`** — must be run from inside a git repo whose `origin` (or `--repo`) points at a GitHub repository.

## Output format

Per reviewer section:

```markdown
### <Reviewer> (<N> unresolved)

**`path/to/file.go`**
- **<title>** (line 17)
  <prose>
- **<another title>** (line 42)
  <prose>

**`other/file.ts`**
- **<title>** (line 88)
  <prose>
```

Title extraction:
- For bot threads (CodeRabbit, Greptile), the first `**bold**` span in the body. Severity preambles (`_⚠️ Potential issue_ | _🟡 Minor_`) and Greptile priority-badge anchors are stripped.
- For human threads with no bold title, the first line of the body (truncated to 160 chars) becomes the title; the rest becomes the prose.

Trailing `<details>` blocks, fenced ` ```suggestion ` blocks, and inline code are preserved in the prose.

## Status indicator

The script ends with one line:

- `0` unresolved → `🟢 No open PR discussions.`
- `1`–`3` unresolved → `🟡 N open discussion(s) across M reviewer(s).`
- `4`+ unresolved → `🔴 N open discussions across M reviewers.`

## Notes

- This skill only covers **review-thread conversations**. Top-level issue comments (walkthrough summaries, general PR comments) are excluded because they can't be resolved.
- Resolved threads are intentionally omitted from output — the focus is open discussion.
- The thread *originator* is the author of the first comment in the thread; subsequent replies don't change the grouping.
