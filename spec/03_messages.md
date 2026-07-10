# Vault Msgs

The Vault module defines a set of `Msg` transaction endpoints for creating vaults, managing interest, funding flows, swap operations, and administrative controls.
All messages are protobuf-defined (`vault.v1`) and handled by the module’s `MsgServer`.

---

<!-- TOC -->
- [Endpoint Gating Matrix](#endpoint-gating-matrix)
- [CreateVault](#createvault)
- [SetShareDenomMetadata](#setShareDenomMetadata)
- [SwapIn](#swapin)
- [SwapOut](#swapout)
- [BridgeMintShares](#bridgemintshares)
- [BridgeBurnShares](#bridgeburnshares)
- [SetBridgeAddress](#setbridgeaddress)
- [ToggleBridgeEnabled](#togglebridgeenabled)
- [UpdateMinInterestRate](#updatemininterestrate)
- [UpdateMaxInterestRate](#updatemaxinterestrate)
- [UpdateInterestRate](#updateinterestrate)
- [UpdateWithdrawalDelay](#updatewithdrawaldelay)
- [UpdateMinSwapInValue](#updateminswapinvalue)
- [UpdateMinSwapOutValue](#updateminswapoutvalue)
- [UpdateMaxSwapInValue](#updatemaxswapinvalue)
- [UpdateMaxSwapOutValue](#updatemaxswapoutvalue)
- [ToggleSwapIn](#toggleswapin)
- [ToggleSwapOut](#toggleswapout)
- [DepositInterestFunds](#depositinterestfunds)
- [WithdrawInterestFunds](#withdrawinterestfunds)
- [DepositPrincipalFunds](#depositprincipalfunds)
- [WithdrawPrincipalFunds](#withdrawprincipalfunds)
- [ExpeditePendingSwapOut](#expeditependingswapout)
- [PauseVault](#pausevault)
- [UnpauseVault](#unpausevault)
- [SetAssetManager](#setassetmanager)
- [UpdateVaultNAV](#updatevaultnav)
- [UpdateNAVAuthority](#updatenavauthority)
- [AcceptAsset](#acceptasset)
- [RejectAsset](#rejectasset)

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
| `UpdateInterestRate`     | Admin or Asset Manager            |                   ✅ |                 ✅ | Validates bounds, may reconcile, updates enable/disable flows.                                                |
| `UpdateWithdrawalDelay`  | Admin or Asset Manager            |                   ✅ |                 ✅ | Updates the withdrawal delay for future swap-out requests.                                                    |
| `UpdateMinSwapInValue`   | Admin or Asset Manager            |                   ✅ |                 ✅ | Updates the minimum allowed value for a swap-in operation.                                                    |
| `UpdateMinSwapOutValue`  | Admin or Asset Manager            |                   ✅ |                 ✅ | Updates the minimum allowed value for a swap-out operation.                                                   |
| `UpdateMaxSwapInValue`   | Admin or Asset Manager            |                   ✅ |                 ✅ | Updates the maximum allowed value for a swap-in operation.                                                    |
| `UpdateMaxSwapOutValue`  | Admin or Asset Manager            |                   ✅ |                 ✅ | Updates the maximum allowed value for a swap-out operation.                                                   |
| `ToggleSwapIn`           | Admin only                        |                   ✅ |                 ✅ | Allows enabling or disabling swap-in operations.                                                              |
| `ToggleSwapOut`          | Admin only                        |                   ✅ |                 ✅ | Allows enabling or disabling swap-out operations.                                                             |
| `DepositInterestFunds`   | Admin or Asset Manager            |                   ✅ |                 ✅ | Underlying denom only; reconciles after deposit.                                                              |
| `WithdrawInterestFunds`  | Admin or Asset Manager            |                   ✅ |                 ✅ | Underlying denom only; reconciles before withdrawal.                                                          |
| `DepositPrincipalFunds`  | Admin or Asset Manager            |                   ❌ |                 ✅ | Requires vault to be paused; reconciles then deposit to principal marker.                                     |
| `WithdrawPrincipalFunds` | Admin or Asset Manager            |                   ❌ |                 ✅ | Requires vault to be paused; reconciles then withdraw from principal marker.                                  |
| `ExpeditePendingSwapOut` | Admin or Asset Manager            |                   ✅ |                 ✅ | No pause gating;                                                                                              |
| `PauseVault`             | Admin or Asset Manager            |                   ✅ |                 ❌ | Strict by default: reconciles, snapshots `PausedBalance`, sets paused; aborts if reconcile/valuation fails. `force=true` pauses best-effort, tolerating failures and recording them on `EventVaultPaused`. |
| `UnpauseVault`           | Admin or Asset Manager            |                   ❌ |                 ✅ | Clears `PausedBalance`, unpauses, emits with current TVV.                                                     |
| `SetAssetManager`        | Admin only                        |                   ✅ |                 ✅ | Sets or clears the delegated asset manager.                                                                   |
| `UpdateVaultNAV`         | NAV authority only                |                   ✅ |                 ❌ | Rejected while paused (value is frozen at `PausedBalance`); otherwise reconciles first, upserts the internal NAV entry, publishes it to the marker module. |
| `UpdateNAVAuthority`     | Admin only                        |                   ✅ |                 ✅ | Rotates the address authorized to mutate the internal NAV table.                                              |
| `AcceptAsset`            | Asset Manager only                |                   ✅ |                 ❌ | Rejected while paused (settlement would move value); otherwise reconciles first, enforces the internal-NAV price guardrail, settles the `x/exchange` payment, records and publishes the settlement NAV. |
| `RejectAsset`            | Asset Manager only                |                   ✅ |                 ✅ | Declines a pending `x/exchange` payment; the exchange module refunds the source's escrow.                     |

**Notes**
* *Admin or Asset Manager* indicates that either the vault admin or the delegated asset manager may sign and execute the transaction.
* **Bridge** operations are restricted to the configured bridge address, not the admin or asset manager.
* **SwapOut** remains asynchronous (enqueues `request_id` for later processing).
* **Principal adjustments** and **pause/unpause** operations are allowed for the asset manager as delegated administrative control.
* *NAV authority only* means the vault's configured `nav_authority`; when none is set, the vault admin acts as the NAV authority.
* *Asset Manager only* means exactly the vault's configured `asset_manager` — the admin cannot sign, and a vault with no asset manager cannot execute the message. The field is a role, not a person: composite approval workflows (e.g. admin and manager both sign) are configured by pointing `asset_manager` at a group address.

## CreateVault

Creates a new vault account with a configured underlying asset, withdrawal delay, and minimum/maximum swap values.
The creator is recorded as vault admin.

* **Single Denom:** Vaults are created with a single denom. `payment_denom` must be empty (it defaults to `underlying_asset`) or equal to `underlying_asset`; creation with a differing payment denom is rejected. `initial_payment_nav` must be omitted since it only applied to mixed-denom vaults. Existing vaults with mixed denoms are unaffected; only creation is gated.
* **Units:** All swap limit values (`min_swap_in_value`, `min_swap_out_value`, `max_swap_in_value`, `max_swap_out_value`) are denominated in the vault's **underlying_asset**.
* **Clearing Limits:** 
    * Minimums: An empty string "" or the string "0" clears/disables the minimum limit.
    * Maximums: An empty string "" clears/disables the maximum limit.
* **Constraints:** Any provided maximum swap value must be **positive (> 0)**. A value of "0" is invalid and will be rejected.

* **Request:** `MsgCreateVaultRequest { admin, share_denom, underlying_asset, payment_denom?, withdrawal_delay_seconds, min_swap_in_value?, min_swap_out_value?, max_swap_in_value?, max_swap_out_value? }`
* **Response:** `MsgCreateVaultResponse {}`

---

## SetShareDenomMetadata

Admin-only. Sets Bank module metadata for a vault’s share denom, defining how it is displayed (name, symbol, units).

- **Request:** `MsgSetShareDenomMetadataRequest { admin, vault_address, metadata }`
- **Response:** `MsgSetShareDenomMetadataResponse {}`

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

Admin or Asset Manager. Updates the current and desired interest rate for a vault.
If interest was previously enabled, triggers a reconciliation before updating.
Transitions may enqueue or clear payout verification / timeout entries.

* **Request:** `MsgUpdateInterestRateRequest { authority, vault_address, new_rate }`
* **Response:** `MsgUpdateInterestRateResponse {}`

---

## UpdateWithdrawalDelay

Admin or Asset Manager. Updates the withdrawal delay for future swap-out requests.

* **Request:** `MsgUpdateWithdrawalDelayRequest { authority, vault_address, withdrawal_delay_seconds }`
* **Response:** `MsgUpdateWithdrawalDelayResponse {}`

---

## UpdateMinSwapInValue

Admin or Asset Manager. Updates the minimum allowed value for a swap-in operation.
An empty string "" or "0" clears the limit. Values are in **underlying_asset** units.

* **Request:** `MsgUpdateMinSwapInValueRequest { authority, vault_address, min_swap_in_value }`
* **Response:** `MsgUpdateMinSwapInValueResponse {}`

---

## UpdateMinSwapOutValue

Admin or Asset Manager. Updates the minimum allowed value for a swap-out operation.
An empty string "" or "0" clears the limit. Values are in **underlying_asset** units.

* **Request:** `MsgUpdateMinSwapOutValueRequest { authority, vault_address, min_swap_out_value }`
* **Response:** `MsgUpdateMinSwapOutValueResponse {}`

---

## UpdateMaxSwapInValue

Admin or Asset Manager. Updates the maximum allowed value for a swap-in operation.
An empty string "" clears the limit. Values must be **positive (> 0)** and are in **underlying_asset** units.

* **Request:** `MsgUpdateMaxSwapInValueRequest { authority, vault_address, max_swap_in_value }`
* **Response:** `MsgUpdateMaxSwapInValueResponse {}`

---

## UpdateMaxSwapOutValue

Admin or Asset Manager. Updates the maximum allowed value for a swap-out operation.
An empty string "" clears the limit. Values must be **positive (> 0)** and are in **underlying_asset** units.

* **Request:** `MsgUpdateMaxSwapOutValueRequest { authority, vault_address, max_swap_out_value }`
* **Response:** `MsgUpdateMaxSwapOutValueResponse {}`

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

Admin or Asset Manager. Deposits principal into a vault’s backing marker account.
Vault must be paused to allow this adjustment.

* **Request:** `MsgDepositPrincipalFundsRequest { authority, vault_address, amount }`
* **Response:** `MsgDepositPrincipalFundsResponse {}`

---

## WithdrawPrincipalFunds

Admin or Asset Manager. Withdraws principal from a vault’s backing marker account.
Vault must be paused to allow this adjustment.

* **Request:** `MsgWithdrawPrincipalFundsRequest { authority, vault_address, amount }`
* **Response:** `MsgWithdrawPrincipalFundsResponse {}`

---

## ExpeditePendingSwapOut

Admin or Asset Manager. Immediately processes a specific queued swap-out by ID.

* **Request:** `MsgExpeditePendingSwapOutRequest { authority, request_id }`
* **Response:** `MsgExpeditePendingSwapOutResponse {}`

---

## PauseVault

Admin or Asset Manager. Pauses a vault, disabling swap-ins and swap-outs, and recording reason + balance snapshot.

By default the pause is **strict**: it reconciles outstanding interest and fees and values the vault first, and any failure (insufficient reserves to settle positive interest, or a broken TVV/NAV conversion) aborts the request and leaves the vault unpaused. The failed transaction is the operator's signal that the vault is in an unexpected state.

Setting `force = true` makes the pause an **emergency control**: a reconcile or valuation failure is logged and tolerated rather than blocking the freeze. The frozen `PausedBalance` is the net TVV when the vault can be valued, or zero when the valuation itself is what failed, so it may be approximate. Persistence is also best-effort: the handler first writes the paused account with validation, and if that validation fails it falls back to writing without validation so an already-inconsistent vault can still be frozen. Every tolerated failure (reconcile, valuation, and persistence) is recorded on `EventVaultPaused.forced_error`.

* **Request:** `MsgPauseVaultRequest { authority, vault_address, reason, force }`
* **Response:** `MsgPauseVaultResponse {}`

---

## UnpauseVault

Admin or Asset Manager. Resumes a paused vault, clears paused balance, and recalculates NAV.

* **Request:** `MsgUnpauseVaultRequest { authority, vault_address }`
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

Mints local share marker supply to the bridge within capacity (`total_shares - local_supply`) and transfers the minted shares to the bridge address. The mint re-materializes shares that already exist on a remote chain, so it raises local supply toward `total_shares` but does **not** change `total_shares`.

* **Request:** `MsgBridgeMintSharesRequest { bridge, vault_address, shares }`
* **Response:** `MsgBridgeMintSharesResponse {}`

---

## BridgeBurnShares

Transfers shares from the bridge back to the vault and burns them from the marker, reducing local supply. It does **not** change `total_shares`: a bridged-out share still exists on the remote chain, so — unlike the local redemption path — no `total_shares` decrement is performed. The burn re-widens mint capacity (`total_shares - local_supply`) by the burned amount, allowing those shares to be re-minted when they return.

See [Bridge Trust Model & Supply-of-Record](01_concepts.md#bridge-trust-model--supply-of-record) for the full model and the off-chain operator trust assumption.

* **Request:** `MsgBridgeBurnSharesRequest { bridge, vault_address, shares }`
* **Response:** `MsgBridgeBurnSharesResponse {}`

---

## SetAssetManager

Admin-only. Sets or clears the optional asset manager address for a vault.
Passing an empty `asset_manager` clears the configured value.

* **Request:** `MsgSetAssetManagerRequest { admin, vault_address, asset_manager }`
* **Response:** `MsgSetAssetManagerResponse {}`

---

## UpdateVaultNAV

NAV authority only (the vault admin when no `nav_authority` is configured). Creates or updates the vault's **internal NAV entry** for a denom: the price of `volume` units of `denom`, denominated in one of the vault's accepted denoms.

The handler is **rejected while the vault is paused**: a paused vault freezes its value at `PausedBalance`, so a NAV update would assert a price that the vault deliberately ignores until unpause. When not paused, the handler reconciles the vault first, so accrued interest settles against the TVV that held before the price change. After the upsert, the NAV is published downstream to the **marker module**, attributed to the vault address.

* `denom` must not be the vault's share denom and must be a registered marker.
* `volume` must be positive. The per-unit value is `price / volume`.
* `source` is an optional origin label (e.g., an oracle name).

* **Request:** `MsgUpdateVaultNAVRequest { signer, vault_address, denom, price, volume, source? }`
* **Response:** `MsgUpdateVaultNAVResponse {}`

---

## UpdateNAVAuthority

Admin-only. Rotates the address authorized to mutate the vault's internal NAV table via `UpdateVaultNAV`.

* **Request:** `MsgUpdateNAVAuthorityRequest { signer, vault_address, new_authority }`
* **Response:** `MsgUpdateNAVAuthorityResponse {}`

---

## AcceptAsset

Asset Manager only — the admin cannot settle, and a vault without an asset manager cannot settle at all. Settles a pending `x/exchange` payment whose target is the vault, exchanging an external asset for the vault's payment denom. The payment is identified by its `source` account and `external_id`.

Settlement is **rejected while the vault is paused**: a paused vault freezes its value at `PausedBalance`, and settling would move principal funds and the vault's value. Reject the payment or unpause first.

Exactly one payment leg must carry the vault's `payment_denom`; the **settlement direction** is derived from which leg that is:

* **Inbound** — payment denom on the target leg: the vault receives the asset (`source_amount`) and pays the payment denom (`target_amount`).
* **Outbound** — payment denom on the source leg: the vault pays the asset (`target_amount`) and receives the payment denom (`source_amount`).

Each leg must carry exactly one coin, and the asset denom must be a registered marker.

Settlement layers several responsibilities into one atomic transaction:

1. **Reconcile** — the vault reconciles before any value change, so interest settles against the pre-settlement TVV.
2. **NAV guardrail** — when an internal NAV entry exists for the asset denom, the settlement legs must match its price exactly (cross-multiplied, no rounding). A first acquisition (no entry) skips the check.
3. **Settle** — funds stage through the vault account as an atomic hop (`Principal -> Vault`, exchange `AcceptPayment`, `Vault -> Principal`); the principal marker remains the long-term store.
4. **Internal NAV upsert** — the settlement price is recorded as the asset denom's internal NAV entry, sourced to the vault. When an outbound settlement drains the principal of the asset denom, the entry is removed (see `EventNAVRemoved`).
5. **Marker publish** — the upserted NAV is published to the marker module, attributed to the vault address.

Any failure reverts the whole transaction.

* **Request:** `MsgAcceptAssetRequest { authority, vault_address, source, external_id }`
* **Response:** `MsgAcceptAssetResponse {}`

---

## RejectAsset

Asset Manager only — the admin cannot reject, and a vault without an asset manager cannot reject at all. Declines a pending `x/exchange` payment whose target is the vault. The exchange module cancels the payment and refunds the source's escrow. No vault state changes, so this remains available even while the vault is paused.

* **Request:** `MsgRejectAssetRequest { authority, vault_address, source, external_id }`
* **Response:** `MsgRejectAssetResponse {}`
