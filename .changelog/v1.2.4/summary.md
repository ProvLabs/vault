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
