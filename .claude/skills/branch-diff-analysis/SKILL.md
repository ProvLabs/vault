---
name: branch-diff-analysis
description: How to scope work to the current branch's diff against main — enumerate changed files, identify changed functions/branches, focus reviews and tests on what actually changed. Invoke whenever the task description mentions "the branch", "this PR", "what changed", or any review/test work that should not redo the whole codebase.
---

# Branch Diff Analysis

When a task is scoped to "the current branch" or "this PR", anchor your work to the actual diff against main. Do not reason about the entire codebase; do not invent context that isn't in the diff.

## Step 1: Confirm the base branch

In this repo the base is `main`. Confirm with `git rev-parse --abbrev-ref HEAD` to know what you're diffing *from*, and verify `main` exists locally with `git rev-parse --verify main`.

If the user mentions a different base (e.g., `release/x.y`), use that instead.

## Step 2: Enumerate changed files

```bash
git diff main...HEAD --name-only
```

The `...` (three dots) form gives the diff vs the merge base, which is what reviewers see. Two dots (`..`) gives a diff vs current main HEAD, which can include unrelated changes from main and is usually wrong for PR review.

Categorize files into:

- **Go source** (`.go`, excluding `*_test.go`) — code under review/test.
- **Tests** (`*_test.go`) — verify they cover the source changes.
- **Proto** (`.proto`) — schema changes; require `make proto-all`.
- **Generated** (`*.pb.go`, `*.pulsar.go`, OpenAPI/swagger output) — derived; review only for unexpected churn.
- **Config / build** (`Makefile`, `go.mod`, `go.sum`, CI YAML) — flag separately.
- **Docs** (`*.md`, `spec/`) — review for consistency with code changes.

## Step 3: Get the diff content

```bash
git diff main...HEAD
```

For large diffs, prefer per-file:

```bash
git diff main...HEAD -- <path>
```

When you need just the function-level summary:

```bash
git diff main...HEAD --stat
```

## Step 4: Map changes to functions and branches

For every changed `.go` file, list:

- Which functions/methods were added, modified, or removed.
- For each modified function, which branches (if/else, switch cases, error paths, return paths) changed.
- Any new error wrapping introduced — confirm it follows the repo's `fmt.Errorf("failed to X: %w", err)` pattern (or `errorsmod.Wrap...` for typed cosmos-sdk errors).
- Any new exported identifier — confirm it has a Godoc.

This map is the input to a focused review or test pass. Do not write tests for code that didn't change; do not review code that didn't change unless the diff has a direct dependency on it.

## Step 5: Cross-check tests vs source

For each changed source file, find the corresponding `*_test.go` (or `suite_test.go` for the package). Confirm:

- Each changed function has a test that exercises the changed branches.
- Existing tests still pass conceptually (no signature drift, no behavior assumption broken).
- If a new helper or type was added, it has at least one test, or a justification for skipping.

## Step 6: Frame scope in your output

When reporting findings, anchor each to a specific file and line from the diff. Don't make claims about code that wasn't changed unless the change directly depends on it.

## Quick reference

| Goal | Command |
|------|---------|
| Files changed | `git diff main...HEAD --name-only` |
| Line counts | `git diff main...HEAD --stat` |
| Full diff | `git diff main...HEAD` |
| Per-file diff | `git diff main...HEAD -- <path>` |
| Commits on branch | `git log main..HEAD --oneline` |
| Merge base | `git merge-base main HEAD` |
