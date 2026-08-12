## [v1.2.4](https://github.com/provlabs/vault/releases/tag/v1.2.4) 2026-08-12

This release materializes each vault's total value into module state, so
reading total vault value costs one store read instead of one walk over the
NAV table. Every path that moves a priced balance or changes a price folds its
change into the entry, and the walk remains the definition of the number. The
module `ConsensusVersion` is bumped to 3, and the v2->v3 migration seeds the
entry for every existing vault. That seeding is required rather than optional,
because the new `total-value` invariant reads state without repairing it, so an
unseeded vault would report as broken on an invariant-enabled chain. A vault
the migration cannot value is logged and skipped rather than failing the
upgrade. The same migration also registers `x/vault` invariants with the crisis
module: `total-value`, `share-supply`, and `escrowed-shares`.

Vault creation is no longer unconditionally governance-gated. The new
`gov_only_vault_creation` module param restricts `CreateVault` to the
governance module account while enabled, and defaults to disabled so
development, docker, and testnet chains can create vaults without a proposal.
The v2->v3 migration enables the param on `pio-mainnet-1`, so the upgrade does
not leave mainnet vault creation open to any signer, and either setting can be
changed later with an `UpdateParams` proposal.

Repricing gained a pause requirement and a batched form. `UpdateVaultNAV` now
requires the vault to be paused to reprice an asset the vault holds, since that
price step moves the share price and a live vault let a user swap in ahead of
it and out after it, taking the difference from the existing shareholders.
Pricing a denom the vault does not hold stays available while live, as does
restating a held asset at the unit price it already carries. The new
`RepriceVault` transaction applies a batch of NAV updates under the identical
per-entry rules, capped at 1000 entries with duplicate denoms rejected, and an
optional `resume` unpauses the vault in the same state transition so an
oversized restatement can span several messages and reopen exactly once.
Setting `resume` requires a strict pause the current NAV authority took itself.
`PauseVault` now accepts the NAV authority as a signer so a pricing oracle can
open the pause window itself; `UnpauseVault` remains admin or asset manager
only. `VaultAccount` gained `paused_by` and `paused_forced`, and a vault
already paused when this ships carries neither, so its pause needs a management
unpause.

`MsgAcceptAssetRequest` now carries the full `payment` being approved instead
of the `source` and `external_id` fields that referenced it, and settlement
proceeds only when the pending payment matches those terms exactly.

The rest of the release is a hardening pass. Several paths that could halt a
chain or panic a handler were closed: genesis now validates pending swap-outs
and bounds `withdrawal_delay_seconds`, the v1->v2 migration pauses and
persists a legacy vault that fails validation instead of aborting the upgrade
handler on every node, and the pro-rata, bridge mint, and `ExpDec` paths use
checked arithmetic. Swap-in now rejects depositors on the underlying marker's
deny list, which the marker bypass previously skipped, and fails closed when
assets are priced against zero total value with shares outstanding. `ibc/`
voucher denoms are blocked from underlying assets and NAV entries, each vault's
NAV table is capped at 2000 priced denoms, bridge mint and burn are rejected on
a paused vault, and swap-out reconciles before pricing so the limits gate the
current value.

### Features

* Added a `RepriceVault` transaction: the batched form of `UpdateVaultNAV`, enforcing the identical per-entry rules, plus an optional `resume` that unpauses the vault in the same state transition. It exists so a pricing oracle can run a repricing cadence without a second signature from the vault admin or asset manager. Batching is required rather than convenient, because a vault holding many priced positions restates them on one cadence and the resume must land once after the last of them, which repeated single-denom messages cannot express; the batch is capped at `MaxRepriceBatchSize` (1000) with duplicate denoms rejected. Leaving `resume` unset applies the batch and leaves the pause untouched, which covers two flows: continuing a restatement too large for one transaction across several messages so the vault reopens exactly once, and letting the NAV authority write a held asset down during a pause somebody else took without being able to lift it. Setting `resume` requires a strict pause the current NAV authority took itself, checked before any price is written, so operator, forced, and automatic pauses, and a pause predating a NAV authority rotation, all still require a management unpause. `PauseVault` now also accepts the NAV authority as a signer, since a pricing oracle observes an event warranting a freeze (a depeg, say) before anyone else and needs to be able to open the pause window itself; `UnpauseVault` is unchanged and remains admin or asset manager only. `VaultAccount` gained `paused_by` and `paused_forced`, recording who took the current pause and whether it waived the strict reconcile and valuation gate; both are cleared on unpause, and a vault already paused when this ships carries neither, so its pause needs a management unpause [PR 270](https://github.com/provlabs/vault/pull/270).
* Registered `x/vault` invariants with the crisis module: `total-value` asserts each vault's materialized total value matches the value derived from its NAV table and principal balances, `share-supply` asserts no vault's local share supply exceeds its `total_shares` cross-chain supply-of-record, and `escrowed-shares` asserts a vault holds at least the shares its pending swap-outs account for. Registration is wired on the crisis keeper directly, because `module.Manager.RegisterInvariants` is a no-op in the current SDK. Every route resolves the vault lookup through the same path the migration and genesis import use, so a lookup entry with no vault account behind it is logged and counted rather than halting an invariant-enabled chain over inert state seeding could never have satisfied [PR 270](https://github.com/provlabs/vault/pull/270).

### Improvements

* Document the NAV freshness model and the NAV authority's repricing responsibility [PR 270](https://github.com/provlabs/vault/pull/270).
* Jitter swap-out retry times deterministically so identical failures stop re-clustering [PR 270](https://github.com/provlabs/vault/pull/270).
* Document the drain-before-rotate procedure for bridge address rotation [PR 270](https://github.com/provlabs/vault/pull/270).
* Cap each vault's NAV table at 2000 priced denoms [PR 270](https://github.com/provlabs/vault/pull/270).
* Unpause reads the materialized total vault value instead of walking the NAV table [PR 270](https://github.com/provlabs/vault/pull/270).
* Block `ibc/` voucher denoms from vault underlying assets and NAV entries [PR 270](https://github.com/provlabs/vault/pull/270).

### Bug Fixes

* Reject deposits from an address on the underlying marker's deny list, which the marker bypass used by swap-in and the interest/principal deposits previously skipped [PR 270](https://github.com/provlabs/vault/pull/270).
* Validate every genesis pending swap-out entry, rejecting a nil, negative, zero, or otherwise invalid escrowed share coin and a negative queue time, and skip a malformed request in the `EndBlocker` instead of building `sdk.Coins` from it, so an imported entry can no longer panic and halt the chain when it matures [PR 270](https://github.com/provlabs/vault/pull/270).
* Reject a nil, zero, or negative `assets` amount on the `EstimateSwapIn` query with `InvalidArgument`, matching the `MsgSwapIn` validation, so a malformed query no longer panics the handler on a nil `big.Int`. The pro-rata share and redeem helpers also reject nil inputs instead of dereferencing them [PR 270](https://github.com/provlabs/vault/pull/270).
* Use `SafeAdd` when applying the virtual asset and share offsets in `CalculateSharesProRata` and `CalculateRedeemProRata`, so totals within the virtual offset of the 256-bit integer maximum return a wrapped error instead of panicking on the redeem path, which has no recover [PR 270](https://github.com/provlabs/vault/pull/270).
* Republish the marker mirrored share NAV on any reconcile that moves the net total vault value, including a fee-only reconcile and the `BeginBlocker` interest and fee timeout paths, so external consumers of the mirror no longer read a stale price-per-share [PR 270](https://github.com/provlabs/vault/pull/270).
* Treat the drained-denom NAV cleanup at the end of an outbound `AcceptAsset` as best-effort, logging and continuing when the entry is already absent instead of reverting a settlement whose funds have already moved [PR 270](https://github.com/provlabs/vault/pull/270).
* Enforce the two-year `MaxWithdrawalDelay` cap in `VaultAccount.Validate()` so genesis import and migration bound `withdrawal_delay_seconds` like the message handlers do, and compute the swap-out payout time with a checked conversion so an out-of-range delay can no longer wrap into the past and pay out in the requesting block [PR 270](https://github.com/provlabs/vault/pull/270).
* Compute bridge mint capacity with `SafeSub` in `BridgeMintShares` [PR 270](https://github.com/provlabs/vault/pull/270).
* Bound the `shares` string on the `EstimateSwapOut` query and validate it before the vault lookup, without echoing it back in the error. The same bound is applied to the swap-limit and interest-rate strings [PR 270](https://github.com/provlabs/vault/pull/270).
* Remove a live vault from the `PayoutVerificationSet` only in the same atomic write that transitions it, so a failed transition retries instead of stalling the vault's accrual [PR 270](https://github.com/provlabs/vault/pull/270).
* Stop the v1->v2 migration from halting the chain when a legacy vault fails the current `VaultAccount.Validate()`. Such a vault is now paused (zeroing its current interest rate and clearing its accrual queue entries) and persisted with the tolerated error on `EventVaultPaused.forced_error`, instead of aborting the upgrade handler on every node [PR 270](https://github.com/provlabs/vault/pull/270).
* Reject deposits priced against zero total assets with shares outstanding, so `SwapIn` fails closed and `EstimateSwapIn` agrees instead of quoting a mint the transaction would refuse [PR 270](https://github.com/provlabs/vault/pull/270).
* Error on a missing or unreadable key in `PendingSwapOutQueue.Dequeue` instead of reporting success [PR 270](https://github.com/provlabs/vault/pull/270).
* Cap `OutstandingAumFee` at the vault's gross total vault value so a persistently uncollectable AUM fee cannot accumulate into a claim larger than the vault holds [PR 270](https://github.com/provlabs/vault/pull/270).
* Reschedule payout and fee timeouts atomically, so a failed re-file leaves the vault queued [PR 270](https://github.com/provlabs/vault/pull/270).
* Export the payout verification set in genesis and re-derive it on import so vaults keep automatic interest scheduling [PR 270](https://github.com/provlabs/vault/pull/270).
* Default a nil paused balance to zero when auto-pausing a vault, avoiding an invalid coin [PR 270](https://github.com/provlabs/vault/pull/270).
* `AcceptAsset` settles only when the pending payment matches the `payment` terms carried in the approval exactly [PR 270](https://github.com/provlabs/vault/pull/270).

### Api Breaking

* `MsgAcceptAssetRequest` now carries the full `payment` being approved instead of the `source` and `external_id` fields that referenced it [PR 270](https://github.com/provlabs/vault/pull/270).

### State Machine Breaking

* Added the `gov_only_vault_creation` module param, which restricts `CreateVault` to the governance module account while enabled. It defaults to disabled so development, docker, and testnet chains can create vaults without a proposal. The vault module `ConsensusVersion` is bumped to 3, and a v2->v3 migration enables the param on `pio-mainnet-1` so the upgrade does not leave mainnet vault creation open to any signer. Either setting can be changed later with an `UpdateParams` proposal [PR 270](https://github.com/provlabs/vault/pull/270).
* Required the vault to be paused for `UpdateVaultNAV` to reprice an asset the vault holds, since that price step moves the share price and a live vault let a user swap in ahead of it and out after it, taking the difference from the existing shareholders. Pricing a denom the vault does not hold stays available while live, as does restating a held asset at the unit price it already carries, so the acquisition path and price attestation refreshes are unaffected. Correcting the price of a held asset is now a pause, reprice, unpause sequence, with swaps closed for the whole span [PR 270](https://github.com/provlabs/vault/pull/270).
* Reconcile the vault before pricing a swap-out so the limits gate the current value [PR 270](https://github.com/provlabs/vault/pull/270).
* Range-reduce the `e^(rt)` exponent in `ExpDec` to prevent sign inversion [PR 270](https://github.com/provlabs/vault/pull/270).
* Rejected `BridgeMintShares` and `BridgeBurnShares` on a paused vault, bringing the bridge inside the pause circuit breaker [PR 270](https://github.com/provlabs/vault/pull/270).
* Materialized each vault's total value into module state (prefix 12), so reading total vault value costs one store read instead of one per priced denom. Every path that moves a priced balance or changes a price folds its change into the entry, and the walk over the NAV table remains the definition of the number. Bumped the module consensus version to 3 with a v2->v3 migration that seeds the entry for every existing vault, which is required rather than optional: the `total-value` invariant reads state without repairing it, so an unseeded vault would report as broken on an invariant-enabled chain. A vault the migration cannot value is logged and skipped rather than failing the upgrade [PR 270](https://github.com/provlabs/vault/pull/270).

### Full Commit History

* https://github.com/provlabs/vault/compare/v1.2.3...v1.2.4

