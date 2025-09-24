# Vault Msgs

The Vault module defines a set of `Msg` transaction endpoints for creating vaults, managing interest, funding flows, swap operations, and administrative controls.  
All messages are protobuf-defined (`vault.v1`) and handled by the module’s `MsgServer`.

---
<!-- TOC -->
- [Endpoint Gating Matrix](#endpoint-gating-matrix)
- [CreateVault](#createvault)
- [SwapIn](#swapin)
- [SwapOut](#swapout)
- [UpdateMinInterestRate](#updatemininterestrate)
- [UpdateMaxInterestRate](#updatemaxinterestrate)
- [UpdateInterestRate](#updateinterestrate)
- [ToggleSwapIn](#toggleswapin)
- [ToggleSwapOut](#toggleswapout)
- [DepositInterestFunds](#depositinterestfunds)
- [WithdrawInterestFunds](#withdrawinterestfunds)
- [DepositPrincipalFunds](#depositprincipalfunds)
- [WithdrawPrincipalFunds](#withdrawprincipalfunds)
- [ExpeditePendingSwapOut](#expeditependingswapout)
- [PauseVault](#pausevault)
- [UnpauseVault](#unpausevault)

---

## Endpoint Gating Matrix

| Endpoint                 | Admin required | Works when UNPAUSED | Works when PAUSED | Notes / gates that still apply                                                                                |
| ------------------------ | -------------- | ------------------: | ----------------: | ------------------------------------------------------------------------------------------------------------- |
| `CreateVault`            | No             |                   ✅ |                 ✅ | Creation only.                                                                                                |
| `SwapIn`                 | No             |                   ✅ |                 ❌ | Keeper `SwapIn` enforces `!vault.Paused`, `SwapInEnabled`, accepted denom, reconcile.                         |
| `SwapOut`                | No             |                   ✅ |                 ❌ | Keeper `SwapOut` enforces `!vault.Paused`, `SwapOutEnabled`, share denom match, payout restrictions, enqueue. |
| `UpdateMinInterestRate`  | Yes            |                   ✅ |                 ✅ | Calls `SetMinInterestRate`.                                                                                   |
| `UpdateMaxInterestRate`  | Yes            |                   ✅ |                 ✅ | Calls `SetMaxInterestRate`.                                                                                   |
| `UpdateInterestRate`     | Yes            |                   ✅ |                 ✅ | Validates bounds, may reconcile, updates enable/disable flows.                                                |
| `ToggleSwapIn`           | Yes            |                   ✅ |                   ✅ | Allow toggle on pause                                                                                         |
| `ToggleSwapOut`          | Yes            |                   ✅ |                   ✅ | Allow toggle on pause                                                                                         |
| `DepositInterestFunds`   | Yes            |                   ✅ |                 ✅ | Underlying denom only; reconciles after deposit.                                                              |
| `WithdrawInterestFunds`  | Yes            |                   ✅ |                 ✅ | Underlying denom only; reconciles before withdrawal.                                                          |
| `DepositPrincipalFunds`  | Yes            |                   ❌ |                 ✅ | Requires paused; reconciles then deposit to principal marker.                                                 |
| `WithdrawPrincipalFunds` | Yes            |                   ❌ |                 ✅ | Requires paused; reconciles then withdraw from principal marker.                                              |
| `ExpeditePendingSwapOut` | Yes            |                   ✅ |                 ✅ | No pause gating; admin-only expedite.                                                                         |
| `PauseVault`             | Yes            |                   ✅ |                 ❌ | Reconciles, snapshots `PausedBalance`, sets paused.                                                           |
| `UnpauseVault`           | Yes            |                   ❌ |                 ✅ | Clears `PausedBalance`, unpauses, emits with current TVV.                                                     |

**Notes**
- **SwapOut is asynchronous**: the tx enqueues a request and returns `request_id`; completion or refund is emitted later in `EndBlocker`.
- **Principal adjustments require pause** to avoid valuation drift during user flows.
- **Toggles are allowed while paused.** Interest fund moves are also allowed while paused (with reconciliation as noted).

## CreateVault

Creates a new vault account with a configured underlying asset, optional payment denom, and withdrawal delay.  
The creator is recorded as vault admin.

- **Request:** `MsgCreateVaultRequest { admin, share_denom, underlying_asset, payment_denom?, withdrawal_delay_seconds }`  
- **Response:** `MsgCreateVaultResponse {}`  

---

## SwapIn

Deposits accepted assets (underlying or payment denom) into a vault in exchange for newly minted shares.  

- **Request:** `MsgSwapInRequest { owner, vault_address, assets }`  
- **Response:** `MsgSwapInResponse {}`  

---

## SwapOut

Redeems shares from a vault in exchange for assets.  
The `redeem_denom` field selects whether the user wants underlying or the vault’s optional payment denom.  
Swap-outs are queued with respect to `withdrawal_delay_seconds`.

- **Request:** `MsgSwapOutRequest { owner, vault_address, assets (shares), redeem_denom }`  
- **Response:** `MsgSwapOutResponse { request_id }`  

---

## UpdateMinInterestRate

Admin-only. Updates the minimum allowable annual interest rate (or disables with empty string).

- **Request:** `MsgUpdateMinInterestRateRequest { admin, vault_address, min_rate }`  
- **Response:** `MsgUpdateMinInterestRateResponse {}`  

---

## UpdateMaxInterestRate

Admin-only. Updates the maximum allowable annual interest rate (or disables with empty string).

- **Request:** `MsgUpdateMaxInterestRateRequest { admin, vault_address, max_rate }`  
- **Response:** `MsgUpdateMaxInterestRateResponse {}`  

---

## UpdateInterestRate

Admin-only. Updates the current and desired interest rate for a vault.  
If interest was previously enabled, triggers a reconciliation before updating.  
Transitions may enqueue or clear payout verification / timeout entries.

- **Request:** `MsgUpdateInterestRateRequest { admin, vault_address, new_rate }`  
- **Response:** `MsgUpdateInterestRateResponse {}`  

---

## ToggleSwapIn

Admin-only. Enables or disables user swap-in operations.

- **Request:** `MsgToggleSwapInRequest { admin, vault_address, enabled }`  
- **Response:** `MsgToggleSwapInResponse {}`  

---

## ToggleSwapOut

Admin-only. Enables or disables user swap-out operations.

- **Request:** `MsgToggleSwapOutRequest { admin, vault_address, enabled }`  
- **Response:** `MsgToggleSwapOutResponse {}`  

---

## DepositInterestFunds

Admin-only. Moves interest reserve funds from admin → vault account.  
Only underlying denom is accepted.

- **Request:** `MsgDepositInterestFundsRequest { admin, vault_address, amount }`  
- **Response:** `MsgDepositInterestFundsResponse {}`  

---

## WithdrawInterestFunds

Admin-only. Withdraws unused interest reserve funds vault → admin.  
Only underlying denom is accepted.

- **Request:** `MsgWithdrawInterestFundsRequest { admin, vault_address, amount }`  
- **Response:** `MsgWithdrawInterestFundsResponse {}`  

---

## DepositPrincipalFunds

Admin-only. Deposits principal into a vault’s backing marker account.  
Vault must be paused to allow this adjustment.

- **Request:** `MsgDepositPrincipalFundsRequest { admin, vault_address, amount }`  
- **Response:** `MsgDepositPrincipalFundsResponse {}`  

---

## WithdrawPrincipalFunds

Admin-only. Withdraws principal from a vault’s backing marker account.  
Vault must be paused to allow this adjustment.

- **Request:** `MsgWithdrawPrincipalFundsRequest { admin, vault_address, amount }`  
- **Response:** `MsgWithdrawPrincipalFundsResponse {}`  

---

## ExpeditePendingSwapOut

Admin-only. Immediately processes a specific queued swap-out by ID.  

- **Request:** `MsgExpeditePendingSwapOutRequest { admin, request_id }`  
- **Response:** `MsgExpeditePendingSwapOutResponse {}`  

---

## PauseVault

Admin-only. Pauses a vault, disabling swap-ins and swap-outs, and recording reason + balance snapshot.

- **Request:** `MsgPauseVaultRequest { admin, vault_address, reason }`  
- **Response:** `MsgPauseVaultResponse {}`  

---

## UnpauseVault

Admin-only. Resumes a paused vault, clears paused balance, and recalculates NAV.

- **Request:** `MsgUnpauseVaultRequest { admin, vault_address }`  
- **Response:** `MsgUnpauseVaultResponse {}`  

---