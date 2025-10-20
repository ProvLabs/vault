# Vault Msgs

The Vault module defines a set of `Msg` transaction endpoints for creating vaults, managing interest, funding flows, swap operations, and administrative controls.
All messages are protobuf-defined (`vault.v1`) and handled by the module’s `MsgServer`.

---

<!-- TOC -->

* [Endpoint Gating Matrix](#endpoint-gating-matrix)
* [CreateVault](#createvault)
* [SwapIn](#swapin)
* [SwapOut](#swapout)
* [BridgeMintShares](#bridgemintshares)
* [BridgeBurnShares](#bridgeburnshares)
* [SetBridgeAddress](#setbridgeaddress)
* [ToggleBridgeEnabled](#togglebridgeenabled)
* [UpdateMinInterestRate](#updatemininterestrate)
* [UpdateMaxInterestRate](#updatemaxinterestrate)
* [UpdateInterestRate](#updateinterestrate)
* [ToggleSwapIn](#toggleswapin)
* [ToggleSwapOut](#toggleswapout)
* [DepositInterestFunds](#depositinterestfunds)
* [WithdrawInterestFunds](#withdrawinterestfunds)
* [DepositPrincipalFunds](#depositprincipalfunds)
* [WithdrawPrincipalFunds](#withdrawprincipalfunds)
* [ExpeditePendingSwapOut](#expeditependingswapout)
* [PauseVault](#pausevault)
* [UnpauseVault](#unpausevault)

---

## Endpoint Gating Matrix

| Endpoint                 | Admin required (or Asset Manager) | Works when UNPAUSED | Works when PAUSED | Notes / gates that still apply                                                                                |
| ------------------------ | --------------------------------- | ------------------: | ----------------: | ------------------------------------------------------------------------------------------------------------- |
| `CreateVault`            | No                                |                   ✅ |                 ✅ | Creation only.                                                                                                |
| `SwapIn`                 | No                                |                   ✅ |                 ❌ | Keeper `SwapIn` enforces `!vault.Paused`, `SwapInEnabled`, accepted denom, reconcile.                         |
| `SwapOut`                | No                                |                   ✅ |                 ❌ | Keeper `SwapOut` enforces `!vault.Paused`, `SwapOutEnabled`, share denom match, payout restrictions, enqueue. |
| `BridgeMintShares`       | Bridge only                       |                   ✅ |                 ✅ | Requires `bridge_enabled`, signer == `bridge_address`, shares denom match, positive amount, capacity ≤ `total_shares`. |
| `BridgeBurnShares`       | Bridge only                       |                   ✅ |                 ✅ | Requires `bridge_enabled`, signer == `bridge_address`, shares denom match, positive amount; burns from marker. |
| `SetBridgeAddress`       | Admin only                        |                   ✅ |                 ✅ | Sets or updates the single authorized `bridge_address`.                                                       |
| `ToggleBridgeEnabled`    | Admin only                        |                   ✅ |                 ✅ | Enables/disables bridge operations; no mint/burn allowed when disabled.                                       |
| `UpdateMinInterestRate`  | Admin only                        |                   ✅ |                 ✅ | Validates and updates the minimum allowable interest rate.                                                    |
| `UpdateMaxInterestRate`  | Admin only                        |                   ✅ |                 ✅ | Validates and updates the maximum allowable interest rate.                                                    |
| `UpdateInterestRate`     | Admin only                        |                   ✅ |                 ✅ | Validates bounds, may reconcile, updates enable/disable flows.                                                |
| `ToggleSwapIn`           | Admin only                        |                   ✅ |                 ✅ | Allows enabling or disabling swap-in operations.                                                              |
| `ToggleSwapOut`          | Admin only                        |                   ✅ |                 ✅ | Allows enabling or disabling swap-out operations.                                                             |
| `DepositInterestFunds`   | Admin or Asset Manager            |                   ✅ |                 ✅ | Underlying denom only; reconciles after deposit.                                                              |
| `WithdrawInterestFunds`  | Admin or Asset Manager            |                   ✅ |                 ✅ | Underlying denom only; reconciles before withdrawal.                                                          |
| `DepositPrincipalFunds`  | Admin or Asset Manager            |                   ❌ |                 ✅ | Requires paused; reconciles then deposit to principal marker.                                                 |
| `WithdrawPrincipalFunds` | Admin or Asset Manager            |                   ❌ |                 ✅ | Requires paused; reconciles then withdraw from principal marker.                                              |
| `ExpeditePendingSwapOut` | Admin or Asset Manage             |                   ✅ |                 ✅ | No pause gating;                                                                                              |
| `PauseVault`             | Admin or Asset Manager            |                   ✅ |                 ❌ | Reconciles, snapshots `PausedBalance`, sets paused.                                                           |
| `UnpauseVault`           | Admin or Asset Manager            |                   ❌ |                 ✅ | Clears `PausedBalance`, unpauses, emits with current TVV.                                                     |
| `SetAssetManager`        | Admin only                        |                   ✅ |                 ✅ | Sets or clears the delegated asset manager.                                                                   |

**Notes**
* *Admin or Asset Manager* indicates that either the vault admin or the delegated asset manager may sign and execute the transaction.
* **Bridge** operations are restricted to the configured bridge address, not the admin or asset manager.
* **SwapOut** remains asynchronous (enqueues `request_id` for later processing).
* **Principal adjustments** and **pause/unpause** operations are allowed for the asset manager as delegated administrative control.

## CreateVault

Creates a new vault account with a configured underlying asset, optional payment denom, and withdrawal delay.
The creator is recorded as vault admin.

* **Request:** `MsgCreateVaultRequest { admin, share_denom, underlying_asset, payment_denom?, withdrawal_delay_seconds }`
* **Response:** `MsgCreateVaultResponse {}`

---

## SwapIn

Deposits accepted assets (underlying or payment denom) into a vault in exchange for newly minted shares.

* **Request:** `MsgSwapInRequest { owner, vault_address, assets }`
* **Response:** `MsgSwapInResponse {}`

---

## SwapOut

Redeems shares from a vault in exchange for assets.
The `redeem_denom` field selects whether the user wants underlying or the vault’s optional payment denom.
Swap-outs are queued with respect to `withdrawal_delay_seconds`.

* **Request:** `MsgSwapOutRequest { owner, vault_address, assets (shares), redeem_denom }`
* **Response:** `MsgSwapOutResponse { request_id }`

---

## UpdateMinInterestRate

Admin-only. Updates the minimum allowable annual interest rate (or disables with empty string).

* **Request:** `MsgUpdateMinInterestRateRequest { admin, vault_address, min_rate }`
* **Response:** `MsgUpdateMinInterestRateResponse {}`

---

## UpdateMaxInterestRate

Admin-only. Updates the maximum allowable annual interest rate (or disables with empty string).

* **Request:** `MsgUpdateMaxInterestRateRequest { admin, vault_address, max_rate }`
* **Response:** `MsgUpdateMaxInterestRateResponse {}`

---

## UpdateInterestRate

Admin-only. Updates the current and desired interest rate for a vault.
If interest was previously enabled, triggers a reconciliation before updating.
Transitions may enqueue or clear payout verification / timeout entries.

* **Request:** `MsgUpdateInterestRateRequest { admin, vault_address, new_rate }`
* **Response:** `MsgUpdateInterestRateResponse {}`

---

## ToggleSwapIn

Admin-only. Enables or disables user swap-in operations.

* **Request:** `MsgToggleSwapInRequest { admin, vault_address, enabled }`
* **Response:** `MsgToggleSwapInResponse {}`

---

## ToggleSwapOut

Admin-only. Enables or disables user swap-out operations.

* **Request:** `MsgToggleSwapOutRequest { admin, vault_address, enabled }`
* **Response:** `MsgToggleSwapOutResponse {}`

---

## DepositInterestFunds

Admin or Asset Manager. Moves interest reserve funds from authority → vault account.
Only underlying denom is accepted.

* **Request:** `MsgDepositInterestFundsRequest { authority, vault_address, amount }`
* **Response:** `MsgDepositInterestFundsResponse {}`

---

## WithdrawInterestFunds

Admin or Asset Manager. Withdraws unused interest reserve funds vault → authority.
Only underlying denom is accepted.

* **Request:** `MsgWithdrawInterestFundsRequest { authority, vault_address, amount }`
* **Response:** `MsgWithdrawInterestFundsResponse {}`

---

## DepositPrincipalFunds

Admin-only. Deposits principal into a vault’s backing marker account.
Vault must be paused to allow this adjustment.

* **Request:** `MsgDepositPrincipalFundsRequest { admin, vault_address, amount }`
* **Response:** `MsgDepositPrincipalFundsResponse {}`

---

## WithdrawPrincipalFunds

Admin-only. Withdraws principal from a vault’s backing marker account.
Vault must be paused to allow this adjustment.

* **Request:** `MsgWithdrawPrincipalFundsRequest { admin, vault_address, amount }`
* **Response:** `MsgWithdrawPrincipalFundsResponse {}`

---

## ExpeditePendingSwapOut

Admin-only. Immediately processes a specific queued swap-out by ID.

* **Request:** `MsgExpeditePendingSwapOutRequest { admin, request_id }`
* **Response:** `MsgExpeditePendingSwapOutResponse {}`

---

## PauseVault

Admin-only. Pauses a vault, disabling swap-ins and swap-outs, and recording reason + balance snapshot.

* **Request:** `MsgPauseVaultRequest { admin, vault_address, reason }`
* **Response:** `MsgPauseVaultResponse {}`

---

## UnpauseVault

Admin-only. Resumes a paused vault, clears paused balance, and recalculates NAV.

* **Request:** `MsgUnpauseVaultRequest { admin, vault_address }`
* **Response:** `MsgUnpauseVaultResponse {}`

---

## SetBridgeAddress

Admin-only. Sets or updates the single authorized external bridge address for a vault.

* **Request:** `MsgSetBridgeAddressRequest { admin, vault_address, bridge_address }`
* **Response:** `MsgSetBridgeAddressResponse {}`

---

## ToggleBridgeEnabled

Admin-only. Enables or disables bridge operations for a vault.

* **Request:** `MsgToggleBridgeEnabledRequest { admin, vault_address, enabled }`
* **Response:** `MsgToggleBridgeEnabledResponse {}`

---

## BridgeMintShares

Mints local share marker supply to the bridge within capacity (`total_shares - local_supply`) and transfers the minted shares to the bridge address.

* **Request:** `MsgBridgeMintSharesRequest { bridge, vault_address, shares }`
* **Response:** `MsgBridgeMintSharesResponse {}`

---

## BridgeBurnShares

Transfers shares from the bridge back to the vault and burns them from the marker, reducing local supply (does not change `total_shares`).

* **Request:** `MsgBridgeBurnSharesRequest { bridge, vault_address, shares }`
* **Response:** `MsgBridgeBurnSharesResponse {}`

---

## SetAssetManager

Admin-only. Sets or clears the optional asset manager address for a vault.
Passing an empty `asset_manager` clears the configured value.

* **Request:** `MsgSetAssetManagerRequest { admin, vault_address, asset_manager }`
* **Response:** `MsgSetAssetManagerResponse {}`
