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
* `authority` — actor (admin or asset manager)
* `reason` — pause reason (opaque string)
* `total_vault_value` — snapshot of TVV (coin in underlying denom)

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
