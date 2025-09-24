# Vault Events

This document describes all events emitted by the `x/vault` module and how to use them operationally—especially for **swap-out** flows that are queued and completed later in `EndBlocker`.

---
<!-- TOC -->
- [Lifecycle](#lifecycle)
  - [EventVaultCreated](#eventvaultcreated)
  - [EventVaultPaused](#eventvaultpaused)
  - [EventVaultUnpaused](#eventvaultunpaused)
- [Swaps](#swaps)
  - [EventSwapIn](#eventswapin)
  - [EventSwapOutRequested](#eventswapoutrequested)
  - [EventPendingSwapOutExpedited](#eventpendingswapoutexpedited)
  - [EventSwapOutCompleted](#eventswapoutcompleted)
  - [EventSwapOutRefunded](#eventswapoutrefunded)
  - [How to tell if your SwapOut succeeded](#how-to-tell-if-your-swapout-succeeded)
- [Interest](#interest)
  - [EventVaultReconcile](#eventvaultreconcile)
  - [EventVaultInterestChange](#eventvaultinterestchange)
  - [EventMinInterestRateUpdated](#eventmininterestrateupdated)
  - [EventMaxInterestRateUpdated](#eventmaxinterestrateupdated)
  - [EventInterestDeposit](#eventinterestdeposit)
  - [EventInterestWithdrawal](#eventinterestwithdrawal)
- [Admin Toggles](#admin-toggles)
  - [EventToggleSwapIn](#eventtoggleswapin)
  - [EventToggleSwapOut](#eventtoggleswapout)

---

## Lifecycle

### EventVaultCreated
Emitted when a vault is created.

**Fields**
- `vault_address` — bech32 vault address  
- `admin` — vault admin  
- `share_denom` — vault share token denom  
- `underlying_asset` — base collateral denom

---

### EventVaultPaused
Emitted when a vault is paused (user ops disabled).

**Fields**
- `vault_address` — vault  
- `admin` — actor  
- `reason` — pause reason (opaque string)  
- `total_vault_value` — snapshot of TVV (coin in underlying denom)

---

### EventVaultUnpaused
Emitted when a vault is unpaused (user ops re-enabled).

**Fields**
- `vault_address` — vault  
- `admin` — actor  
- `total_vault_value` — TVV at unpause (coin in underlying denom)

---

## Swaps

### EventSwapIn
Emitted when assets are swapped into shares.

**Fields**
- `owner` — depositor  
- `amount_in` — assets deposited (underlying denom or payment denom if supported for in-flow)  
- `shares_received` — minted shares  
- `vault_address` — vault

---

### EventSwapOutRequested
Emitted when a **SwapOut** request is accepted into the **pending** queue. This happens immediately in the tx that calls `MsgSwapOut` (not at payout time).

**Fields**
- `vault_address` — vault  
- `owner` — requester (recipient will be the same owner)  
- `redeem_denom` — chosen payout denom (`underlying_asset` or optional `payment_denom`)  
- `shares` — escrowed shares amount  
- `request_id` — **stable handle** for this request

**Notes**
- The swap-out is **not** paid yet. Use `request_id` to track completion/refund later.

---

### EventPendingSwapOutExpedited
Emitted when an admin expedites a pending swap-out (moves it to the front of the processing queue).

**Fields**
- `request_id` — target request  
- `vault` — vault address  
- `admin` — actor

---

### EventSwapOutCompleted
Emitted when a pending swap-out is **successfully paid** in `EndBlocker`.

**Fields**
- `vault_address` — vault  
- `owner` — recipient of funds  
- `assets` — payout amount (in `redeem_denom` that was requested)  
- `request_id` — the completed request

---

### EventSwapOutRefunded
Emitted when a pending swap-out **fails** and escrowed shares are returned to the owner.

**Fields**
- `vault_address` — vault  
- `owner` — shares returned to this address  
- `shares` — refunded share amount  
- `request_id` — the failed request  
- `reason` — short reason (insufficient liquidity, paused, denom unsupported, etc.)

---

### How to tell if your SwapOut succeeded

Swap-outs are **asynchronous** and complete in `EndBlocker` after the vault’s `withdrawal_delay_seconds` elapses.

**Client pattern**
1. Submit `MsgSwapOut` and capture `request_id` from the tx’s `MsgSwapOutResponse`.  
2. Watch subsequent blocks for one of these events with that `request_id`:
   - **Success:** `EventSwapOutCompleted{ request_id, assets, owner, vault_address }`
   - **Failure/Refund:** `EventSwapOutRefunded{ request_id, shares, reason, owner, vault_address }`
3. (Optional) If you have admin access and need to accelerate processing, call `MsgExpeditePendingSwapOut` and look for `EventPendingSwapOutExpedited{ request_id }`. Completion will still be signaled by the `Completed` **or** `Refunded` event later.

**Operational tips**
- If the vault is **paused** after your request, payout will not occur until unpaused; you may see `EventVaultPaused` followed by a future `EventVaultUnpaused`. Your request will ultimately end in `Completed` or `Refunded`.
- For monitoring systems, index events by `request_id` and `vault_address`, and set a timeout expectation based on `withdrawal_delay_seconds` plus normal block timings.

---

## Interest

### EventVaultReconcile
Emitted whenever the module applies accrued interest (positive or negative).

**Fields**
- `vault_address` — vault  
- `principal_before` — marker balance before  
- `principal_after` — marker balance after  
- `rate` — annualized rate used for the period (decimal string)  
- `time` — payout duration in seconds covered by this reconciliation  
- `interest_earned` — interest applied (coin; may be negative)

---

### EventVaultInterestChange
Emitted when the vault’s interest rate configuration changes.

**Fields**
- `vault_address` — vault  
- `current_rate` — active rate after change (may be `"0"` to disable)  
- `desired_rate` — desired/admin rate (mirrors current in this flow)

---

### EventMinInterestRateUpdated
Emitted when the vault’s **minimum** interest limit is updated.

**Fields**
- `vault_address` — vault  
- `admin` — actor  
- `min_rate` — decimal string (`""` to clear)

---

### EventMaxInterestRateUpdated
Emitted when the vault’s **maximum** interest limit is updated.

**Fields**
- `vault_address` — vault  
- `admin` — actor  
- `max_rate` — decimal string (`""` to clear)

---

### EventInterestDeposit
Emitted when interest reserve funds are deposited (admin → vault).

**Fields**
- `vault_address` — vault  
- `admin` — actor  
- `amount` — coin (must be underlying denom)

---

### EventInterestWithdrawal
Emitted when unused interest reserve funds are withdrawn (vault → admin).

**Fields**
- `vault_address` — vault  
- `admin` — actor  
- `amount` — coin (underlying denom)

---

## Admin Toggles

### EventToggleSwapIn
Emitted when **swap-in** is enabled/disabled.

**Fields**
- `vault_address` — vault  
- `admin` — actor  
- `enabled` — boolean

---

### EventToggleSwapOut
Emitted when **swap-out** is enabled/disabled.

**Fields**
- `vault_address` — vault  
- `admin` — actor  
- `enabled` — boolean

---
