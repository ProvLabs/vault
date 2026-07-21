## [v1.2.0](https://github.com/provlabs/vault/releases/tag/v1.2.0) 2026-07-21

This release introduces the internal NAV system and the peer-to-peer multi-asset
vault workflow, allowing a vault to accept external assets in exchange for its underlying asset through `x/exchange` settlement, priced against a per-vault NAV table managed by a designated NAV authority.

It also removes the mixed-denom (payment denom) vault functionality in favor of the p2p workflow: vaults are now strictly single-denom on their underlying asset, with a v1 to v2 migration that flattens existing mixed-denom vaults in place. Vault share markers now require deposit access, restricting principal deposits to the vault itself.

### Features

* Added an internal NAV system: a per-vault, per-denom NAV table with a designated NAV authority, `UpdateVaultNAV` and `UpdateNAVAuthority` transactions, `VaultNavs` and `NavValue` queries, genesis import/export, and `EventNAVUpdated`/`EventNAVAuthorityUpdated` events [#189](https://github.com/provlabs/vault/issues/189).
* Added a peer-to-peer multi-asset vault workflow that lets a vault take in external assets in exchange for its payment denom: new `AcceptAsset` and `RejectAsset` transactions settle or decline pending `x/exchange` payments targeting the vault, with supporting queries and simulations. Either the vault admin or asset manager may authorize a settlement. Built on the newly integrated Provenance `x/exchange` module (with its `x/hold` and `x/metadata` dependencies) [#192](https://github.com/provlabs/vault/issues/192).
* Added settlement-time responsibilities to the p2p multi-asset workflow: `AcceptAsset` now enforces an exact-price guardrail against the vault's internal NAV, records the settlement price in the internal NAV table (removing the entry with a new `EventNAVRemoved` when an outbound settlement drains the denom), publishes the updated NAV to the marker module attributed to the vault, and triggers a vault reconcile before settling. `UpdateVaultNAV` likewise reconciles first and publishes the updated NAV to the marker module [#193](https://github.com/provlabs/vault/issues/193).
* Add a `force` flag to `PauseVault` that pauses a vault best-effort when the pre-pause reconcile, valuation, or account persistence fails, falling back to an unvalidated write and recording every tolerated error on `EventVaultPaused.forced_error`; without it the pause stays strict and aborts on failure [#228](https://github.com/provlabs/vault/issues/228).

### Improvements

* Classify swap-out refund reasons with typed errors instead of fragile error-string matching [#90](https://github.com/provlabs/vault/issues/90).
* Add simulation tests to validate swap estimation accuracy [#112](https://github.com/provlabs/vault/issues/112).
* Switched the valuation engine to read prices exclusively from the per-vault Internal NAV table; `UnitPriceFraction` now performs a single `GetVaultNAV` lookup, no longer consults `MarkerKeeper.GetNetAssetValue`, no longer applies the temporary `uylds.fcc` 1:1 peg, and surfaces a distinct `no internal NAV entry for denom` error when an entry is missing [#190](https://github.com/provlabs/vault/issues/190).
* Add a net TVV calculation that subtracts the outstanding AUM fee liability from gross vault value, and use it as the valuation basis for share NAV publication, NAV per share, deposit/redeem conversions, and the captured paused balance [PR 213](https://github.com/provlabs/vault/pull/213).
* Fix the lint workflow and run it against pull requests, upgrade golangci-lint to v2, and resolve the outstanding lint findings [PR 214](https://github.com/provlabs/vault/pull/214).
* Reworked `GetTVV` to value the accepted denoms directly and iterate the per-vault Internal NAV table for held assets instead of walking every principal-marker balance, so Total Vault Value cost scales with the number of valued denoms rather than with whatever is parked at the principal marker [#223](https://github.com/provlabs/vault/issues/223).
* Share NAV publication is now uint64-safe: when a vault's total shares exceed the `uint64` marker volume ceiling, the volume is capped at the uint64 maximum and the price is scaled down proportionally (computed via `math/big` so the intermediate product cannot overflow), so a correct price-per-share is always published instead of being skipped. Supplies that already fit in uint64 publish their exact share count unchanged. When a vault's supply drops to zero, a previously published share NAV is overwritten with a zero price so a stale price is not served for a share denom that no longer has supply [#233](https://github.com/provlabs/vault/issues/233).
* Disallow vault creation with a payment denom that differs from the underlying asset. The payment denom must now be empty (defaulting to the underlying) or equal to it. This creation-time check does not itself alter existing vaults; a separate state-machine-breaking migration flattens pre-existing mixed-denom vaults [#239](https://github.com/provlabs/vault/issues/239).
* Split the expected `ExchangeKeeper` interface into a keeper part and a new `ExchangeQueryServer` dependency so consuming apps can wire the exchange keeper and `exchangekeeper.NewQueryServer` directly instead of writing an adapter struct [#247](https://github.com/provlabs/vault/issues/247).

### Bug Fixes

* Enforce per-block visit budgets on all ABCI queue processors (interest timeouts, fee timeouts, payout verification set, and pending swap-outs) and dequeue/refund pending swap-outs for paused vaults so they cannot camp at the front of the queue [#225](https://github.com/provlabs/vault/issues/225).
* Preserve a pending swap-out and its escrowed shares when a payout or refund fails critically, removing the queue entry only when its payout or refund commits [#226](https://github.com/provlabs/vault/issues/226).
* Reject swap-in deposits that are too small to mint at least one share, so no funds move when the computed share amount is zero [#237](https://github.com/provlabs/vault/issues/237).

### Deprecated

* Remove the mixed-denom request fields (`payment_denom` and `initial_payment_nav` on `MsgCreateVaultRequest`, `redeem_denom` on `MsgSwapOutRequest` and `QueryEstimateSwapOutRequest`) with their field numbers reserved, while `VaultAccount.payment_denom` and `PendingSwapOut.redeem_denom` remain deprecated on the wire for migration decoding [#240](https://github.com/provlabs/vault/issues/240).

### State Machine Breaking

* Remove the mixed-denom (payment denom) vault functionality in favor of the p2p workflow, making vaults strictly single-denom on their underlying asset, with a v1->v2 migration that flattens existing mixed-denom vaults in place [#240](https://github.com/provlabs/vault/issues/240).
* Enable require_deposit_access on every vault share marker, at creation and via the 1->2 migration for existing vaults, and grant the vault address deposit access so only the vault can move coins into its principal marker [#248](https://github.com/provlabs/vault/issues/248).

### Full Commit History

* https://github.com/provlabs/vault/compare/v1.1.0...v1.2.0

