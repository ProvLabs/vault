---
name: cosmos-sdk-provenance-knowledge
description: Reference for Cosmos SDK and Provenance Blockchain architecture used by this repo — module layout, keepers, message/query servers, ante handlers, ABCI lifecycle, IBC, upgrade handlers, x/marker specifics, errorsmod patterns. Invoke at the start of any audit, review, or implementation task that touches cosmos-sdk or Provenance modules.
---

# Cosmos SDK & Provenance Reference

This is the domain knowledge baseline for anyone touching the vault repo's cosmos-sdk module code or interacting with Provenance Blockchain's custom modules. The reference repo for Provenance is https://github.com/provenance-io/provenance.

## Module lifecycle (apply in this order)

When building or modifying a cosmos-sdk module:

1. **Proto schema** — `.proto` files under `proto/`. Define state, messages, queries, events with full Godoc-grade documentation. After changes, run `make proto-all` (or `proto-regen` for an iterating loop).
2. **Validation** — message validation, basic type validation, and helper types under `types/`. `ValidateBasic` lives here; signer-aware validation lives in the handler.
3. **Logic** — core keeper functionality under `keeper/`. Keepers hold dependency keepers and store keys; methods take `sdk.Context` first.
4. **Exposure** — `MsgServer` and `QueryServer` implementations that wire RPC into keeper methods.
5. **Verification** — exhaustive table-driven tests, including suite-level integration tests in `*_test.go` and helpers in `suite_test.go`.

## Keepers

- A keeper holds: store key(s), codec, dependency keepers (bank, account, marker, etc.), and an authority.
- Methods take `ctx sdk.Context` as the first argument (or `context.Context` if invoked via a gRPC server boundary that hasn't unwrapped to sdk.Context yet).
- Never store `ctx` in the keeper struct. Threading `ctx` per-call preserves determinism and lets the SDK manage gas, events, and stores.
- Use prefix iteration via `sdk.KVStorePrefixIterator` and always `defer iterator.Close()`.

## Message and query servers

- `MsgServer` methods are the public mutation entrypoints. They:
  1. Unwrap `sdk.UnwrapSDKContext(goCtx)` to get the sdk.Context.
  2. Validate the signer / authority.
  3. Call into keeper methods.
  4. Emit typed events via `ctx.EventManager().EmitTypedEvent(...)`.
  5. Return the response proto.
- `QueryServer` methods are read-only — no state mutations, no event emission.

## Ante handlers and decorators

- Ante handlers run *before* DeliverTx in a defined chain. Ordering matters — earlier decorators can short-circuit later ones.
- Common decorators: signature verification, sequence checking, fee deduction (`msgfees` in Provenance), nonce / replay protection.
- Adding a custom ante decorator means inserting it into the chain in the app constructor at the right position.

## ABCI lifecycle (consensus-critical)

The following run in consensus and must be deterministic and panic-free under all reachable inputs:

- `InitChain` / `InitGenesis` — once at genesis. Validate genesis state thoroughly before committing.
- `BeginBlock` — per block, before transactions. Use for reward distribution, slashing, scheduled triggers.
- `DeliverTx` / `CheckTx` — per transaction. Mutate state (DeliverTx) or simulate (CheckTx).
- `EndBlock` — per block, after transactions. Use for validator updates, governance tallying.
- `Commit` — finalize the block.
- `ExportGenesis` — produce genesis JSON for export. Must round-trip with `InitGenesis`.

Any panic in these paths halts the chain. See the `chain-halt-safety` skill for the full risk model.

## IBC

- ICS-20 fungible token transfer is the most common touchpoint.
- ICS-27 interchain accounts, ICS-29 fees.
- Provenance adds **ibchooks** (CosmWasm-callable on receive) and **ibcratelimit** (per-denom flow caps) — both affect restricted markers travelling cross-chain.
- Send restrictions on restricted markers must apply to outbound IBC transfers; relying only on x/bank gates is a known bypass class.

## Upgrade handlers and migrations

- Each upgrade has a name registered in the app constructor and a handler that runs once at the upgrade height.
- Handlers may run store migrations (key reformatting, schema changes). Use `Migrator` patterns per module.
- An upgrade handler that fails after partial state mutation is a chain-halt scenario — make state changes idempotent or transactional.
- Update `CHANGELOG.md` and note any state breaks.

## Error wrapping

Cosmos-sdk uses typed errors that carry a codespace and code so RPC clients can react.

- Prefer `errorsmod.Wrap(err, "specific context")` or `errorsmod.Wrapf(err, "...", args)` over `fmt.Errorf` when the underlying error is a cosmos-sdk error.
- For brand-new errors, register a `ModuleErr` in `types/errors.go` with a stable codespace and code.
- Do NOT lose typed error information by wrapping with `fmt.Errorf("...: %w", typedErr)` — that strips downstream code-based handling.

## Provenance x/marker specifics

- **Marker types**: `COIN` (unrestricted) vs `RESTRICTED`. Restricted markers enforce send restrictions and required attributes; unrestricted markers do not. Never conflate them.
- **Send restrictions**: enforced via `SendRestrictionFn` hooks registered with the bank keeper. They must apply to all transfer paths — direct `MsgSend`, `MsgMultiSend`, `SendCoinsFromModuleToAccount`, IBC, authz grants, and CosmWasm-driven transfers.
- **Required attributes**: each restricted marker can require recipient attributes. Enforcement runs before any transfer. Empty attribute lists effectively unrestrict the marker — flag that.
- **Access roles**: `MINT`, `BURN`, `DEPOSIT`, `WITHDRAW`, `DELETE`, `ADMIN`, `TRANSFER`. Audit who can grant these and which msgs require which role.
- **Marker states**: `PROPOSED` → `ACTIVE` → `FINALIZED` (with `CANCELLED`/`DESTROYED` terminal states). Proposed markers cannot be used until finalized.
- **Forced transfer**: `MsgTransferRequest` lets an admin move restricted marker coins. Audit that only authorized parties can call it.

## Other Provenance modules to know

- `x/metadata` — record / scope / session ownership.
- `x/name`, `x/attribute` — name resolution and account attributes (used by required-attribute enforcement).
- `x/msgfees` — per-message additional fee gating.
- `x/exchange` — orderbook and matching.
- `x/hold` — non-custodial fund locking.
- `x/quarantine`, `x/sanction` — recipient-side controls.
- `x/oracle`, `x/trigger` — external data and conditional execution.
- `x/reward` — reward programs.

## Determinism rules

State machine code must be deterministic across all validators. Sources of non-determinism to watch for:

- `time.Now()` — use `ctx.BlockTime()`.
- `rand` without a deterministic seed — use seeded sources.
- Iteration over Go maps — sort keys first.
- Floating point — use `sdk.Dec` for fixed-point math.
- External I/O (network, disk outside the KVStore) — never in consensus paths.

## Run `make proto-all` after proto changes

Generated code (`*.pb.go`, `*.pulsar.go`, gRPC, OpenAPI) does not refresh automatically. After editing `.proto`, run `make proto-all` and commit the regenerated files.
