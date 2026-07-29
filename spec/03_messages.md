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
- [RemoveVaultNAV](#removevaultnav)
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
| `UpdateVaultNAV`         | NAV authority only                |                   ✅ |                 ✅ | Upserts the internal NAV entry; the price is never mirrored to the marker module. Reconciles first when unpaused; leaves `PausedBalance` frozen when paused, so a pause-reprice-unpause sequence cannot be front-run and the new price takes effect at unpause. |
| `RemoveVaultNAV`         | NAV authority only                |                   ✅ |                 ✅ | Deletes the internal NAV entry for a denom the vault does not hold. Value-neutral in both states, since an unheld denom contributes nothing to total vault value. |
| `UpdateNAVAuthority`     | Admin only                        |                   ✅ |                 ✅ | Rotates the address authorized to mutate the internal NAV table.                                              |
| `AcceptAsset`            | Asset Manager only                |                   ✅ |                 ❌ | Rejected while paused (settlement would move value); otherwise reconciles first, requires an internal NAV entry and enforces its price exactly, then settles the `x/exchange` payment. Never writes the NAV table. |
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

* **Single Denom:** Vaults are strictly single-denom on `underlying_asset`; it is the only denom for deposits, redemptions, interest, and fees.
* **Units:** All swap limit values (`min_swap_in_value`, `min_swap_out_value`, `max_swap_in_value`, `max_swap_out_value`) are denominated in the vault's **underlying_asset**.
* **Clearing Limits:** 
    * Minimums: An empty string "" or the string "0" clears/disables the minimum limit.
    * Maximums: An empty string "" clears/disables the maximum limit.
* **Constraints:** Any provided maximum swap value must be **positive (> 0)**. A value of "0" is invalid and will be rejected.

* **Request:** `MsgCreateVaultRequest { admin, share_denom, underlying_asset, withdrawal_delay_seconds, min_swap_in_value?, min_swap_out_value?, max_swap_in_value?, max_swap_out_value? }`
* **Response:** `MsgCreateVaultResponse {}`

> **Deprecated:** `payment_denom` is deprecated but retained on the wire for compatibility with
> released clients; if set, it must equal `underlying_asset` or the message is rejected.

---

## SetShareDenomMetadata

Admin-only. Sets Bank module metadata for a vault’s share denom, defining how it is displayed (name, symbol, units).

- **Request:** `MsgSetShareDenomMetadataRequest { admin, vault_address, metadata }`
- **Response:** `MsgSetShareDenomMetadataResponse {}`

---

## SwapIn

Deposits the vault's underlying asset into a vault in exchange for newly minted shares. The underlying asset is the only accepted deposit denom.

* **Request:** `MsgSwapInRequest { owner, vault_address, assets }`
* **Response:** `MsgSwapInResponse {}`

---

## SwapOut

Redeems shares from a vault in exchange for the vault's underlying asset.
Payouts are always made in the underlying asset.
Swap-outs are queued with respect to `withdrawal_delay_seconds`.

* **Request:** `MsgSwapOutRequest { owner, vault_address, assets (shares) }`
* **Response:** `MsgSwapOutResponse { request_id }`

> **Deprecated:** `redeem_denom` no longer selects the payout coin; payouts are always the
> underlying asset. The field is retained on the wire for compatibility with released clients —
> if set, it must equal the vault's `underlying_asset` or the message is rejected.

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

If the vault is paused when the expedited request comes due, the request is dequeued and refunded with `EventSwapOutRefunded{ reason = "vault_paused" }` instead of being paid out.

Expediting also clears the request's `failure_count`, so it is the lever for forcing an immediate retry on a request whose attempts kept failing and which is waiting out a retry backoff. See [Retry & Backoff](06_blocker.md#retry--backoff).

---

## PauseVault

Admin or Asset Manager. Pauses a vault, disabling swap-ins and swap-outs, and recording reason + balance snapshot.

By default the pause is **strict**: it reconciles outstanding interest and fees and values the vault first, and any failure (insufficient reserves to settle positive interest, or a broken TVV/NAV conversion) aborts the request and leaves the vault unpaused. The failed transaction is the operator's signal that the vault is in an unexpected state.

Setting `force = true` makes the pause an **emergency control**: a reconcile or valuation failure is logged and tolerated rather than blocking the freeze. The frozen `PausedBalance` is the net TVV when the vault can be valued, or zero when the valuation itself is what failed, so it may be approximate. Persistence is also best-effort: the handler first writes the paused account with validation, and if that validation fails it falls back to writing without validation so an already-inconsistent vault can still be frozen. Every tolerated failure (reconcile, valuation, and persistence) is recorded on `EventVaultPaused.forced_error`.

Both paths remove the vault from the `PayoutVerificationSet` and from its `PayoutTimeoutQueue` and `VaultFeeTimeoutQueue` entries, and clear its interest and fee period starts, so a paused vault accrues neither interest nor AUM fees while frozen. Removal is keyed on the vault's recorded timeouts to keep it constant-time; a forced or automatic pause tolerates a failed removal, and any surviving entry is dequeued by the blocker that next visits it.

* **Request:** `MsgPauseVaultRequest { authority, vault_address, reason, force }`
* **Response:** `MsgPauseVaultResponse {}`

---

## UnpauseVault

Admin or Asset Manager. Resumes a paused vault, clears paused balance, and recalculates NAV.

It also re-arms what pausing cleared: the vault is added back to the `PayoutVerificationSet` and a fresh fee timeout is enqueued, with both period starts set to the unpause block time so the paused span is never charged interest or AUM fees.

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

NAV authority only (the vault admin when no `nav_authority` is configured). Creates or updates the vault's **internal NAV entry** for a denom: the price of `volume` units of `denom`, denominated in the vault's underlying asset.

The handler is accepted **whether or not the vault is paused**, so an operator can pause, reprice, and unpause as one deliberate sequence. Swap-ins and swap-outs are closed for the whole paused span, which keeps a repricing from being front-run by a user transaction ordered ahead of it.

When the vault is **not paused**, the handler reconciles first, so accrued interest settles against the TVV that held before the price change.

When the vault **is paused**, the reconcile is a no-op (accrual is already halted) and `PausedBalance` is left untouched, still holding the value as of the moment of pausing. This matches how the pause already treats `DepositPrincipalFunds` and `WithdrawPrincipalFunds`, which are themselves only allowed while paused and likewise do not move the frozen value. Everything done during the pause, repricings and principal movements alike, takes effect together at `UnpauseVault`, when the snapshot is cleared and total vault value is recomputed from live balances and the NAV table.

The price stays **internal to the vault**. A vault does not own the assets it prices, so an asset price is never mirrored into that asset's marker-module NAV records, where it would compete with prices set by the marker's own administrators. Only the vault's share denom, which the vault does own, gets a published marker NAV.

* `denom` must not be the vault's share denom, and must name an asset that exists on-chain: a registered marker, or, for a metadata value-owner denom (`nft/<scope-id>`), an existing metadata scope.
* `volume` must be positive. The per-unit value is `price / volume`.
* `source` is an optional origin label (e.g., an oracle name).

The vault does **not** have to hold the denom. The internal NAV table is a price list rather than a held-asset inventory, and an entry for a denom the vault does not hold contributes nothing to total vault value until the asset arrives at the principal marker. Pricing a denom ahead of time is how the NAV authority authorizes the asset manager to acquire it: `AcceptAsset` requires an entry and settles only at exactly that price.

* **Request:** `MsgUpdateVaultNAVRequest { signer, vault_address, denom, price, volume, source? }`
* **Response:** `MsgUpdateVaultNAVResponse {}`

---

## RemoveVaultNAV

NAV authority only (the vault admin when no `nav_authority` is configured). Deletes the vault's internal NAV entry for a denom, revoking the authorization to acquire that denom at that price.

Only entries for denoms the vault does **not** hold may be removed. Because total vault value is computed by valuing held balances against the entries in the NAV table, dropping the entry for a held asset would erase that balance from the vault's value rather than restate it. A held asset that has lost its value is written down to a zero price through `UpdateVaultNAV` instead, and the settlement path removes the entry on its own once an outbound trade drains the denom.

The handler is accepted **whether or not the vault is paused**, matching `UpdateVaultNAV`, so the NAV table stays editable across a pause-reprice-unpause sequence. No reconcile is needed in either state: the held-balance check above already restricts removal to denoms that contribute nothing to total vault value, so a removal cannot move the valuation basis.

* `denom` must have an existing internal NAV entry on the vault.

* **Request:** `MsgRemoveVaultNAVRequest { signer, vault_address, denom }`
* **Response:** `MsgRemoveVaultNAVResponse {}`

---

## UpdateNAVAuthority

Admin-only. Rotates the address authorized to mutate the vault's internal NAV table via `UpdateVaultNAV` and `RemoveVaultNAV`.

* **Request:** `MsgUpdateNAVAuthorityRequest { signer, vault_address, new_authority }`
* **Response:** `MsgUpdateNAVAuthorityResponse {}`

---

## AcceptAsset

Asset Manager only — the admin cannot settle, and a vault without an asset manager cannot settle at all. Settles a pending `x/exchange` payment whose target is the vault, exchanging an external asset for the vault's underlying asset. The payment is identified by its `source` account and `external_id`.

Settlement is **rejected while the vault is paused**: a paused vault freezes its value at `PausedBalance`, and settling would move principal funds and the vault's value. Reject the payment or unpause first.

Exactly one payment leg must carry the vault's underlying asset; the **settlement direction** is derived from which leg that is:

* **Inbound** — underlying asset on the target leg: the vault receives the asset (`source_amount`) and pays the underlying asset (`target_amount`).
* **Outbound** — underlying asset on the source leg: the vault pays the asset (`target_amount`) and receives the underlying asset (`source_amount`).

Each leg must carry exactly one coin, and the asset denom must carry an internal NAV entry set by the vault's NAV authority.

Settlement layers several responsibilities into one atomic transaction:

1. **Reconcile** — the vault reconciles before any value change, so interest settles against the pre-settlement TVV.
2. **NAV guardrail** — the asset denom must already have an internal NAV entry, and the settlement legs must match its price exactly (cross-multiplied, no rounding). A denom the NAV authority has never priced cannot be acquired.
3. **Settle** — funds stage through the vault account as an atomic hop (`Principal -> Vault`, exchange `AcceptPayment`, `Vault -> Principal`); the principal marker remains the long-term store.
4. **Drained-denom cleanup** — when an outbound settlement drains the principal of the asset denom, its internal NAV entry is removed (see `EventNAVRemoved`), so reacquiring the denom requires a fresh price. Nothing else about the NAV table changes: the guardrail has already proven the trade executed at the authority's recorded price, so settling never writes a price.

Any failure reverts the whole transaction.

Because pricing and settling are separate messages, a first acquisition takes two: `UpdateVaultNAV` from the NAV authority, then `AcceptAsset` from the asset manager. Both can ride in a single transaction — with two signatures when the roles are held by different entities — so the price and the settlement commit atomically.

* **Request:** `MsgAcceptAssetRequest { authority, vault_address, source, external_id }`
* **Response:** `MsgAcceptAssetResponse {}`

---

## RejectAsset

Asset Manager only — the admin cannot reject, and a vault without an asset manager cannot reject at all. Declines a pending `x/exchange` payment whose target is the vault. The exchange module cancels the payment and refunds the source's escrow. No vault state changes, so this remains available even while the vault is paused.

* **Request:** `MsgRejectAssetRequest { authority, vault_address, source, external_id }`
* **Response:** `MsgRejectAssetResponse {}`
