---
name: branch-diff-analysis
description: How to scope work to the current branch's diff against main — enumerate changed files, identify changed functions/branches, focus reviews and tests on what actually changed. Invoke whenever the task description mentions "the branch", "this PR", "what changed", or any review/test work that should not redo the whole codebase.
---

# Branch Diff Analysis

When a task is scoped to "the current branch" or "this PR", anchor your work to the actual diff. Do not reason about the entire codebase; do not invent context that isn't in the diff.

## Get the diff report

Run `scripts/run.py` from the skill directory. It produces a structured report:

```bash
python3 ${CLAUDE_PLUGIN_ROOT:-.claude/skills/branch-diff-analysis}/scripts/run.py [--base=<ref>]
```

The report contains:

- Current branch, base ref, merge-base SHA, total file count.
- Changed files **grouped by category** (Go source, Go tests, Protobuf, Generated, Config / build, Docs, Other) with `+added / -removed` line counts per file.
- A **test coverage cross-check** listing changed Go source files that have no `*_test.go` modification in the same package.
- A **function-level summary** listing every function name that appears in a diff hunk header.

Default base is `main`. Override with `--base=release/x.y` if the user specifies a different base.

## What to do with the report

The script handles the mechanical parts. Your job is the judgment:

1. **Map functions to changed branches.** For each function the report names, open the per-file diff (`git diff <base>...HEAD -- <path>`) and identify which `if/else`, `switch`, error paths, and return paths actually changed. Don't reason about untouched branches.

2. **Confirm error-wrapping conventions.** Any new error return must wrap with `fmt.Errorf("failed to X: %w", err)` (or `errorsmod.Wrap...` for typed cosmos-sdk errors). Flag bare returns.

3. **Check Godoc coverage.** Every new exported identifier needs a Godoc.

4. **Use the cross-check list.** Files in the "without matching test change" list need either a test added or a written justification for skipping (e.g., the change is a pure rename, the function is already covered by an integration test elsewhere).

5. **Frame findings in your output** with file paths and line numbers from the diff. Do not make claims about code that wasn't changed unless the change directly depends on it.

## Generated files

Anything in the "Generated" bucket (`*.pb.go`, `*.pulsar.go`, swagger output) is derived. Review only for unexpected churn — never write tests against generated code. If a `.proto` file changed without a regenerated `*.pb.go`, flag that `make proto-all` was likely skipped.
