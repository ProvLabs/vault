# Vault Events

This document describes all events emitted by the `x/vault` module and how to use them operationally—especially for **swap-out** flows that are queued and completed later in `EndBlocker`.

---

<!-- TOC -->
- [Lifecycle](#lifecycle)
  - [EventVaultCreated](#eventvaultcreated)
  - [EventVaultPaused](#eventvaultpaused)
  - [EventVaultUnpaused](#eventvaultunpaused)
  - [EventAssetManagerSet](#eventassetmanagerset)
- [Swaps](#swaps)
  - [EventSwapIn](#eventswapin)
  - [EventSwapOutRequested](#eventswapoutrequested)
  - [EventPendingSwapOutExpedited](#eventpendingswapoutexpedited)
  - [EventSwapOutCompleted](#eventswapoutcompleted)
  - [EventSwapOutRefunded](#eventswapoutrefunded)
  - [How to tell if your SwapOut succeeded](#how-to-tell-if-your-swapout-succeeded)
- [Interest & Fees](#interest--fees)
  - [EventVaultReconcile](#eventvaultreconcile)
  - [EventVaultFeeCollected](#eventvaultfeecollected)
  - [EventVaultInterestChange](#eventvaultinterestchange)
  - [EventMinInterestRateUpdated](#eventmininterestrateupdated)
  - [EventMaxInterestRateUpdated](#eventmaxinterestrateupdated)
  - [EventInterestDeposit](#eventinterestdeposit)
  - [EventInterestWithdrawal](#eventinterestwithdrawal)
- [Principal Management](#principal-management)
  - [EventDepositPrincipalFunds](#eventdepositprincipalfunds)
  - [EventWithdrawPrincipalFunds](#eventwithdrawprincipalfunds)
- [Admin Toggles](#admin-toggles)
  - [EventToggleSwapIn](#eventtoggleswapin)
  - [EventToggleSwapOut](#eventtoggleswapout)
  - [EventWithdrawalDelayUpdated](#eventwithdrawaldelayupdated)
- [Bridge](#bridge)
  - [EventBridgeAddressSet](#eventbridgeaddressset)
  - [EventBridgeToggled](#eventbridgetoggled)
  - [EventBridgeMintShares](#eventbridgemintshares)
  - [EventBridgeBurnShares](#eventbridgeburnshares)
- [Settlement & NAV](#settlement--nav)
  - [EventAssetAccepted](#eventassetaccepted)
  - [EventAssetRejected](#eventassetrejected)
  - [EventNAVUpdated](#eventnavupdated)
  - [EventNAVRemoved](#eventnavremoved)
  - [EventNAVAuthorityUpdated](#eventnavauthorityupdated)
- [Metadata](#metadata)
  - [EventSetShareDenomMetadata](#eventsetsharedenommetadata)
  - [EventDenomUnit](#eventdenomunit)

---

## Lifecycle

### EventVaultCreated

Emitted when a vault is created.

**Fields**

* `vault_address` — bech32 vault address
* `admin` — vault admin
* `share_denom` — vault share token denom
* `underlying_asset` — base collateral denom

---

### EventVaultPaused

Emitted when a vault is paused (user ops disabled).

**Fields**

* `vault_address` — vault
* `authority` — actor that triggered the pause. For a manual pause this is the admin or asset manager; for an automated auto-pause it is the vault's own address.
* `reason` — pause reason. For a manual pause this is the user-supplied reason; for an automated auto-pause it carries the hard-coded reason describing the critical error that forced the pause.
* `total_vault_value` — snapshot of TVV (coin in underlying denom)
* `forced` — true when the pause waived the strict reconcile/valuation gate; set by a `force = true` manual pause and by every automated auto-pause
* `forced_error` — the reconcile and/or valuation error tolerated by an explicit `force = true` manual pause; empty when nothing failed. Only the manual `force` path sets this. Auto-pause leaves it empty: the critical error that triggered the pause is carried in `reason`, and any secondary valuation failure while snapshotting the balance is logged only. When non-empty, `total_vault_value` may be stale or zero.

---

### EventVaultUnpaused

Emitted when a vault is unpaused (user ops re-enabled).

**Fields**

* `vault_address` — vault
* `authority` — actor (admin or asset manager)
* `total_vault_value` — TVV at unpause (coin in underlying denom)

---

### EventAssetManagerSet

Emitted when an asset manager is configured or cleared.

**Fields**

* `vault_address` — vault
* `admin` — actor
* `asset_manager` — bech32 address (empty if cleared)

---

## Swaps

### EventSwapIn

Emitted when assets are swapped into shares.

**Fields**

* `owner` — depositor
* `amount_in` — assets deposited (underlying denom or payment denom if supported for in-flow)
* `shares_received` — minted shares
* `vault_address` — vault

---

### EventSwapOutRequested

Emitted when a **SwapOut** request is accepted into the **pending** queue. This happens immediately in the tx that calls `MsgSwapOut` (not at payout time).

**Fields**

* `vault_address` — vault
* `owner` — requester (recipient will be the same owner)
* `redeem_denom` — chosen payout denom (`underlying_asset` or optional `payment_denom`)
* `shares` — escrowed shares amount
* `request_id` — **stable handle** for this request

**Notes**

* The swap-out is **not** paid yet. Use `request_id` to track completion/refund later.

---

### EventPendingSwapOutExpedited

Emitted when an authority expedites a pending swap-out (moves it to the front of the processing queue).

**Fields**

* `request_id` — target request
* `vault` — vault address
* `authority` — actor (admin or asset manager)

---

### EventSwapOutCompleted

Emitted when a pending swap-out is **successfully paid** in `EndBlocker`.

**Fields**

* `vault_address` — vault
* `owner` — recipient of funds
* `assets` — payout amount (in `redeem_denom` that was requested)
* `request_id` — the completed request

---

### EventSwapOutRefunded

Emitted when a pending swap-out **fails** and escrowed shares are returned to the owner.

**Fields**

* `vault_address` — vault
* `owner` — shares returned to this address
* `shares` — refunded share amount
* `request_id` — the failed request
* `reason` — short reason (insufficient liquidity, paused, denom unsupported, etc.)

---

### How to tell if your SwapOut succeeded

Swap-outs are **asynchronous** and complete in `EndBlocker` after the vault’s `withdrawal_delay_seconds` elapses.

**Client pattern**

1. Submit `MsgSwapOut` and capture `request_id` from the tx’s `MsgSwapOutResponse`.
2. Watch subsequent blocks for one of these events with that `request_id`:

   * **Success:** `EventSwapOutCompleted{ request_id, assets, owner, vault_address }`
   * **Failure/Refund:** `EventSwapOutRefunded{ request_id, shares, reason, owner, vault_address }`
3. (Optional) If you have authority and need to accelerate processing, call `MsgExpeditePendingSwapOut` and look for `EventPendingSwapOutExpedited{ request_id }`. Completion will still be signaled by the `Completed` **or** `Refunded` event later.

**Operational tips**

* If the vault is **paused** after your request, payout will not occur until unpaused; you may see `EventVaultPaused` followed by a future `EventVaultUnpaused`. Your request will ultimately end in `Completed` or `Refunded`.
* For monitoring systems, index events by `request_id` and `vault_address`, and set a timeout expectation based on `withdrawal_delay_seconds` plus normal block timings.

---

## Interest & Fees

### EventVaultReconcile

Emitted whenever the module applies accrued interest (positive or negative).

**Fields**

* `vault_address` — vault
* `principal_before` — marker balance before
* `principal_after` — marker balance after
* `rate` — annualized rate used for the period (decimal string)
* `time` — payout duration in seconds covered by this reconciliation
* `interest_earned` — interest applied (coin; may be negative)

---

### EventVaultFeeCollected

Emitted when the 15 bps AUM technology fee is collected.

**Fields**

* `vault_address` — vault
* `collected_amount` — amount actually transferred to ProvLabs (payment denom)
* `requested_amount` — total accrued fee for this period + any previous unpaid amount (payment denom)
* `aum_snapshot` — TVV snapshot used for calculation (underlying denom)
* `outstanding_amount` — remaining unpaid fee after this collection (payment denom)
* `duration_seconds` — time period covered by this collection

---

### EventVaultInterestChange

Emitted when the vault’s interest rate configuration changes.

**Fields**

* `vault_address` — vault
* `current_rate` — active rate after change (may be `"0"` to disable)
* `desired_rate` — desired/admin rate (mirrors current in this flow)

---

### EventMinInterestRateUpdated

Emitted when the vault’s **minimum** interest limit is updated.

**Fields**

* `vault_address` — vault
* `admin` — actor
* `min_rate` — decimal string (`""` to clear)

---

### EventMaxInterestRateUpdated

Emitted when the vault’s **maximum** interest limit is updated.

**Fields**

* `vault_address` — vault
* `admin` — actor
* `max_rate` — decimal string (`""` to clear)

---

### EventInterestDeposit

Emitted when interest reserve funds are deposited (authority → vault).

**Fields**

* `vault_address` — vault
* `authority` — actor (admin or asset manager)
* `amount` — coin (must be underlying denom)

---

### EventInterestWithdrawal

Emitted when unused interest reserve funds are withdrawn (vault → authority).

**Fields**

* `vault_address` — vault
* `authority` — actor (admin or asset manager)
* `amount` — coin (underlying denom)

---

## Principal Management

### EventDepositPrincipalFunds

Emitted when principal funds are deposited (authority → vault principal marker).

**Fields**

* `vault_address` — vault
* `authority` — actor (admin or asset manager)
* `amount` — coin (must be underlying denom)

---

### EventWithdrawPrincipalFunds

Emitted when principal funds are withdrawn (vault principal marker → authority).

**Fields**

* `vault_address` — vault
* `authority` — actor (admin or asset manager)
* `amount` — coin (underlying denom)

---

## Admin Toggles

### EventToggleSwapIn

Emitted when **swap-in** is enabled/disabled.

**Fields**

* `vault_address` — vault
* `admin` — actor
* `enabled` — boolean

---

### EventToggleSwapOut

Emitted when **swap-out** is enabled/disabled.

**Fields**

* `vault_address` — vault
* `admin` — actor
* `enabled` — boolean

---

### EventWithdrawalDelayUpdated

Emitted when the vault's withdrawal delay is updated.

**Fields**

* `vault_address` — vault
* `authority` — actor (admin or asset manager)
* `withdrawal_delay_seconds` — new delay value in seconds

---

## Bridge

### EventBridgeAddressSet

Emitted when the **bridge address** for a vault is configured or updated.

**Fields**

* `vault_address` — vault
* `admin` — actor
* `bridge_address` — external address authorized to mint/burn shares

---

### EventBridgeToggled

Emitted when **bridge functionality** is enabled or disabled.

**Fields**

* `vault_address` — vault
* `admin` — actor
* `enabled` — boolean

**Notes**

* When disabled or the vault is **paused**, bridge mint/burn requests are rejected.

---

### EventBridgeMintShares

Emitted when shares are **minted to the bridge** and transferred out.

**Fields**

* `vault_address` — vault
* `bridge` — bridge signer
* `shares` — minted share amount

---

### EventBridgeBurnShares

Emitted when shares are **burned from the bridge** balance.

**Fields**

* `vault_address` — vault
* `bridge` — bridge signer
* `shares` — burned share amount

---

## Settlement & NAV

### EventAssetAccepted

Emitted when the vault's asset manager settles a pending `x/exchange` payment targeting the vault (via `MsgAcceptAsset`).

**Fields**

* `vault_address` — vault that settled the payment
* `source` — account that created the settled payment
* `external_id` — payment identifier (unique per source)
* `source_amount` — funds the source paid the vault (coins string)
* `target_amount` — funds the vault paid the source (coins string)
* `direction` — `"inbound"` (asset moved into the vault) or `"outbound"` (asset moved out), relative to the vault's payment denom

**Notes**

* An `EventNAVUpdated` for the settled asset denom follows in the same tx, carrying the settlement price. When an outbound settlement drains the denom, an `EventNAVRemoved` follows that.

---

### EventAssetRejected

Emitted when the vault's asset manager declines a pending `x/exchange` payment targeting the vault (via `MsgRejectAsset`). The exchange module refunds the source's escrow.

**Fields**

* `vault_address` — vault that rejected the payment
* `source` — account that created the rejected payment
* `external_id` — payment identifier (unique per source)

---

### EventNAVUpdated

Emitted when a vault's internal NAV entry for a denom is created or updated — by `MsgUpdateVaultNAV` or by a settlement (`MsgAcceptAsset`).

**Fields**

* `vault_address` — vault
* `denom` — asset denom the entry prices
* `price` — total value of `volume` units of the denom (coin string)
* `volume` — number of units `price` covers
* `source` — origin of the update (e.g., oracle name; the vault address for settlement-driven updates)
* `signer` — address that performed the update
* `updated_block_height` — block height of the update

**Notes**

* The upserted NAV is also published to the **marker module**, attributed to the vault address, so a `provenance.marker.v1.EventSetNetAssetValue` is emitted alongside.

---

### EventNAVRemoved

Emitted when a vault's internal NAV entry for a denom is removed — currently when an outbound settlement leaves the vault's principal holding zero of the denom.

**Fields**

* `vault_address` — vault
* `denom` — asset denom whose entry was removed
* `last_price` — total value of `last_volume` units recorded before removal (coin string)
* `last_volume` — number of units `last_price` covered

**Notes**

* When a settlement both prices and drains a denom, this event follows an `EventNAVUpdated` for the same denom in the same tx; `last_price`/`last_volume` carry that final settlement price. Consumers maintaining a local price cache must process both event types.
* The marker module's NAV is left as-is — publishing simply stops.

---

### EventNAVAuthorityUpdated

Emitted when a vault's NAV authority is rotated (via `MsgUpdateNAVAuthority`).

**Fields**

* `vault_address` — vault
* `admin` — vault administrator that performed the rotation
* `new_authority` — address now authorized to mutate the internal NAV table

---

## Metadata

### EventSetShareDenomMetadata

Emitted when denom metadata is set for a vault’s share denom (via `MsgSetShareDenomMetadata`).

**Fields**

- `vault_address` — vault
- `metadata_base` — base denom (e.g., `nushare`)
- `metadata_description` — description of the share denom
- `metadata_display` — display denom (e.g., `ushare` or `SHARE`)
- `metadata_denom_units` — list of denom units with exponents and aliases
- `administrator` — admin who set the metadata
- `metadata_name` — human-readable name
- `metadata_symbol` — ticker-style symbol

---

### EventDenomUnit

Included inside `EventSetShareDenomMetadata` to describe each denom unit.

**Fields**

- `denom` — unit name (e.g., `nushare`, `ushare`)
- `exponent` — power of 10 exponent relative to base unit
- `aliases` — optional alternative names (may be empty)
