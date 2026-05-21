---
name: chain-halt-safety
description: Chain-halt risk model for cosmos-sdk consensus code — panics in ABCI paths, non-determinism, unbounded iteration, upgrade-handler partial mutation, missing genesis validation. Invoke when auditing or implementing any code that runs inside InitChain, InitGenesis, BeginBlock, DeliverTx, CheckTx, EndBlock, Commit, ExportGenesis, or an upgrade handler.
---

# Chain Halt Safety

The chain MUST NOT halt. Any code path that can cause a consensus-layer panic — across all validators simultaneously — is a **CRITICAL** finding, regardless of how unlikely the trigger looks.

## Consensus-critical entry points

Any function reachable from these paths is consensus-critical:

- `InitChain` / `InitGenesis`
- `BeginBlock`
- `CheckTx`
- `DeliverTx` (a.k.a. `FinalizeBlock` in newer SDK)
- `EndBlock`
- `Commit`
- `ExportGenesis`
- Upgrade handler bodies registered with `UpgradeKeeper`

"Reachable from" means by call graph — a helper invoked five hops deep from `EndBlock` is just as dangerous as code directly in `EndBlock`.

## Halt vectors (CRITICAL)

### Panics in the call graph

- `panic()` calls in any function reachable from consensus paths.
- Implicit panics: nil-pointer dereference, out-of-bounds slice/map access (on un-initialized maps, write panics), integer overflow with `panic`-on-overflow types, division by zero, type assertion without the comma-ok form.
- Missing error handling where the alternative is a panic.

**Remediation**: return errors instead of panicking, except for true invariant violations that the SDK is expected to catch and recover (and even then, prefer logging + error returns).

### Non-determinism

The same input must produce the same output on every validator. Sources of divergence:

- Wall-clock time (`time.Now()`) — use `ctx.BlockTime()`.
- Unseeded randomness — use a seeded source derived from block state.
- Map iteration order — sort keys before iterating if state is affected.
- Floating-point math — use `sdk.Dec` / `math.LegacyDec`.
- External I/O — disk outside the KVStore, network calls, environment variables.
- Goroutines that mutate observable state — order is undefined.

A non-deterministic write that diverges across validators won't halt instantly but will halt at the next block when state hashes mismatch.

### Unbounded loops / iteration

- Unbounded loops in consensus code can prevent a block from ever completing.
- Unbounded prefix iteration over a store that grows without limit.

**Remediation**: cap iteration with a max per-block budget; paginate work across blocks if needed.

### Upgrade handler failures

- An upgrade handler that fails after partial state mutation leaves the chain in a half-migrated state. All nodes hit the same failure → all halt.
- Migration logic must be idempotent or transactional so a partial run can be retried.

### Missing genesis validation

- `InitGenesis` must reject invalid genesis state before committing. Validating after commit is too late.
- `ExportGenesis` → `InitGenesis` must round-trip without loss.

### Stack overflow

- Deep recursion in consensus paths can overflow the goroutine stack and panic.

### External dependencies in consensus paths

- Any consensus-path code that depends on a service or oracle that could be unavailable is a halt risk. If the dependency fails, every validator fails the same way at the same height.

## Severity language for halt findings

All halt vectors are **CRITICAL**, even if the trigger looks unlikely. The blast radius is the whole network — every user is affected. Do not soften severity to avoid alarm.

## Auditing checklist for consensus-path code

For any function reachable from a consensus entry point, confirm:

- [ ] No `panic()` calls (except in code paths that are themselves protected by SDK recovery and explicitly intended as invariant assertions).
- [ ] No nil-pointer dereferences in unguarded paths.
- [ ] No out-of-bounds slice or unbounded map writes.
- [ ] No division by zero or unchecked overflow in numeric computation.
- [ ] All errors are returned, not panicked.
- [ ] No `time.Now()` — uses `ctx.BlockTime()`.
- [ ] No unsorted map iteration if iteration order affects state.
- [ ] No floats; uses `sdk.Dec`.
- [ ] No external I/O or environment reads.
- [ ] All loops bounded by a per-block cap.
- [ ] Genesis validation rejects malformed input.
- [ ] Genesis import/export round-trips.
- [ ] Upgrade handler is idempotent or transactional under partial-failure.

## When to flag a halt finding

Whenever you find one. State explicitly:

- What triggers the halt (the input or state condition).
- Which consensus phase it affects.
- The remediation (usually: return an error instead of panicking, or move I/O out of the consensus path).
