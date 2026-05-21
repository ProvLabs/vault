---
name: vault-lint-test
description: Run the vault repo's full local verification — format, lint, unit tests, and the simplest simulations (test-sim-simple, test-sim-nondeterminism). Use before opening a PR, after merging main, or when the user asks to "lint and test" or "run sims". Reports any failure with the first failing target's output and stops.
---

# vault-lint-test

Run `scripts/run.sh` from this skill directory. The script orchestrates the chain, stops at the first failure, captures per-step logs under `$TMPDIR/vault-lint-test/`, and prints a status table.

```bash
${CLAUDE_PLUGIN_ROOT:-.claude/skills/vault-lint-test}/scripts/run.sh [--fast] [--from=N]
```

## Mode selection (you decide)

The script accepts flags; choose them based on the change context:

- **Default (no flags)** — all 5 steps. Use when verifying a PR, after merging main, or any non-trivial code change.
- **`--fast`** — skip the two sims. Pick this when:
  - The change is documentation-only (no `.go`, `.proto`, no genesis-touching code).
  - The user explicitly asks for a fast loop (e.g., iterating on a single failing test).
  - A previous run in this session already passed the sims and only test files or godocs have changed since.
- **`--from=N`** — resume at step N (1=format, 2=lint, 3=test-unit, 4=sim-simple, 5=sim-nondeterminism). Use after fixing a failure to skip re-running passed steps.

Sims add 10–30 minutes — don't run them implicitly when `--fast` is justified.

## Interpreting failures

The script's status table tells you which step failed and gives a one-line summary. To act on it:

- **lint FAIL** — do NOT auto-fix. Report the `file:line + linter` from the table to the user. Many findings are stylistic and may warrant a `//nolint` with rationale instead of a code change. Let the user decide.
- **test-unit FAIL** — re-run the named test in verbose mode for a cleaner trace:
  ```bash
  go test -v -run <TestName> ./<path>/...
  ```
- **test-sim-simple FAIL** — the table shows `seed=X, block=Y`. Those reproduce the failure. Investigate the state machine at that block height with that seed.
- **test-sim-nondeterminism FAIL** — two seeds produced divergent state hashes. This is a **CRITICAL** consensus-breaking bug. Flag it as critical regardless of how the trigger looks.
- **format MODIFIED** — `gofumpt`/`goimports-reviser` rewrote files. Re-stage them and continue; this is not a failure.

Full per-step output lives in the log directory printed by the script.
