---
name: vault-lint-test
description: Run the vault repo's full local verification — format, lint, unit tests, and the simplest simulations (test-sim-simple, test-sim-nondeterminism). Use before opening a PR, after merging main, or when the user asks to "lint and test" or "run sims". Reports any failure with the first failing target's output and stops.
---

# vault-lint-test

Run the vault repo's standard local-verification chain and report pass/fail. This is the same chain that runs in CI for unit work; running it locally catches most issues before pushing.

## Order of operations

Run these targets sequentially. **Stop at the first failure** and surface the failing output. Do not continue to later steps if an earlier one failed — the later results will be misleading.

| Step | Target | What it does | Approx duration |
|------|--------|--------------|-----------------|
| 1 | `make format` | Runs `gofumpt -l -w` and `goimports-reviser` over `keeper/*`. Modifies files in place. | seconds |
| 2 | `make lint` | Runs `golangci-lint run --timeout=10m`. | 1–5 min |
| 3 | `make test-unit` | Runs `go test -cover -race -v ./...` over keeper/interest/queue/types. Produces `coverage.out` and `coverage.html`. | 1–3 min |
| 4 | `make test-sim-simple` | Simplest module simulation: `TestSimple` against `./simapp` with 50 blocks, block size 100, seed 99, 1h timeout. | 5–15 min |
| 5 | `make test-sim-nondeterminism` | Non-determinism check: `TestAppStateDeterminism` with memdb, 50 blocks, block size 100, 24h timeout. | 5–15 min |

## How to run

From the repo root:

```bash
make format && make lint && make test-unit && make test-sim-simple && make test-sim-nondeterminism
```

If you only have a few minutes (e.g., the user wants a quick sanity check), default to **steps 1–3** and ask whether to continue into the sims. Sims take long enough that they shouldn't be implicit on every invocation.

## Handling failures

- **format produces a diff**: `gofumpt` / `goimports-reviser` already wrote the fixes. Re-stage and continue.
- **lint failures**: Report the exact `golangci-lint` finding (file:line + linter name). Do not auto-fix lint findings — the user reviews them; many are stylistic and may need a `//nolint` with rationale instead of a code change.
- **test-unit failures**: Show the failing test name and the assertion message. Re-run the single failing test in verbose mode (`go test -v -run TestName ./path/...`) to get cleaner output if the full-suite log is noisy.
- **sim failures**: Sims are seeded and deterministic. If `test-sim-simple` fails, capture the seed and block height from the output — those are the keys to reproducing. If `test-sim-nondeterminism` fails, that means two seeds produced divergent state hashes, which is a consensus-breaking bug — flag it as critical.

## When to skip the sims

The sims add 10–30 minutes. Skip them when:
- The change is documentation-only (no `.go`, `.proto`, no genesis-touching code).
- The user explicitly asks for a fast loop (e.g., during iterative fixing of a single test).
- A previous run in this session already passed the sims and the only changes since are confined to test files or godocs.

Otherwise, run them. The whole point of the sims is to catch state-machine bugs that unit tests miss.

## Reporting back

After the chain completes (pass or first-fail), report:

```
make format    : PASS (no changes needed) | MODIFIED (N files reformatted)
make lint      : PASS | FAIL — first failing rule and location
make test-unit : PASS (X tests, coverage Y%) | FAIL — failing test name
make test-sim-simple        : PASS | SKIPPED | FAIL — seed, block, reason
make test-sim-nondeterminism: PASS | SKIPPED | FAIL — state hash divergence details
```

Keep it terse. The user can drill into specifics if they care.
