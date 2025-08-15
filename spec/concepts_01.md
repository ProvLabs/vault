# Vault Concepts

The `x/vault` module tokenizes a pool of underlying assets and issues vault shares. Users can swap underlying assets for shares (`SwapIn`) and redeem shares for underlying assets (`SwapOut`). The module supports configurable interest (positive or negative) funded and accounted directly on-chain. All transfers honor the Provenance `x/marker` send‑restriction model.

---

<!-- TOC -->

* [Accounts & Denoms](#accounts--denoms)
* [State](#state)
* [Lifecycle](#lifecycle)

  * [CreateVault](#createvault)
  * [SwapIn (assets → shares)](#swapin-assets--shares)
  * [SwapOut (shares → assets)](#swapout-shares--assets)
  * [Principal & Reserves Admin Flows](#principal--reserves-admin-flows)
  * [Swap Toggles](#swap-toggles)
* [Interest](#interest)

  * [Rates & Limits](#rates--limits)
  * [Accrual & Reconciliation](#accrual--reconciliation)
  * [BeginBlocker / EndBlocker behavior](#beginblocker--endblocker-behavior)
  * [Estimation (queries)](#estimation-queries)
* [Queries](#queries)
* [Events](#events)
* [Params](#params)
* [Errors & Validation](#errors--validation)
* [Planned Additions](#planned-additions)

---

## Accounts & Denoms

Each vault uses two on‑chain locations for the single underlying asset denom:

* **Principal (marker account for the vault’s share denom):** holds the underlying asset that backs shares. Address = `marker(share_denom)`.
* **Reserves (vault account address):** funds used to pay **positive** interest (and to receive funds from the marker for **negative** interest).

Denoms:

* `underlying_asset`: the single supported asset denom for the vault.
* `share_denom`: the vault’s share token denom (restricted marker).

## State

`VaultAccount`

* `address` (from embedded `BaseAccount`)
* `share_denom`
* `underlying_assets[0]` (single supported denom)
* `admin`
* `current_interest_rate`, `desired_interest_rate`
* `min_interest_rate`, `max_interest_rate`
* `swap_in_enabled`, `swap_out_enabled`

`VaultInterestDetails`

* `period_start` (Unix seconds)
* `expire_time` (Unix seconds; auto‑reconcile guard window)

Module keeps a keyed set of vault addresses and a map of `VaultInterestDetails` by vault.

## Lifecycle

### CreateVault

* Requires the **underlying asset marker** to already exist.
* Creates a **share marker** for `share_denom` with the vault account as manager and grants:
  * `Mint`, `Burn`, `Withdraw` to the vault account.
* Finalizes and activates the share marker.
* Stores the `VaultAccount`; emits `EventVaultCreated`.

### SwapIn (assets → shares)

High‑level flow:

1. Load vault; require `swap_in_enabled`.
2. `ReconcileVaultInterest` (applies any due interest before changing supply).
3. Validate `assets` denom matches `underlying_asset`.
4. Compute shares from current ratio:
   `shares = f(assets_in, total_assets_at_marker, total_shares_supply)`
5. Mint shares to the **vault account**, then withdraw to the **owner**.
6. Send `assets` from **owner** to the **marker(principal)**.
7. Emit `EventSwapIn`.

### SwapOut (shares → assets)

High‑level flow:

1. Load vault; require `swap_out_enabled`.
2. Require `shares.denom == share_denom`.
3. `ReconcileVaultInterest`.
4. Compute assets from shares:
   `assets = f(shares_in, total_shares_supply, total_assets_at_marker)`
5. Owner sends shares to **marker(principal)**.
6. Burn shares from the **vault account**.
7. Send `assets` from **marker(principal)** to owner using transfer‑agent context (`WithTransferAgents(ctx, vaultAddr)`).
8. Emit `EventSwapOut`.

### Principal & Reserves Admin Flows

* **DepositInterestFunds:** admin → **vault account (reserves)**; validates denom; reconciles before event.
* **WithdrawInterestFunds:** **vault account (reserves)** → admin; validates denom; reconciles before event.
* **DepositPrincipalFunds:** admin → **marker(principal)**; validates denom; reconciles before event.
* **WithdrawPrincipalFunds:** **marker(principal)** → admin; validates denom; reconciles before event.

### Swap Toggles

* **ToggleSwapIn / ToggleSwapOut**: admin‑gated flips that update `swap_in_enabled` / `swap_out_enabled` and emit toggle events.
* Toggling does not affect existing supply; it only gates new swaps.

## Interest

### Rates & Limits

* Admin can set **min**/**max** rate limits (empty string disables that bound).
* Admin updates the **current** and **desired** rates with `UpdateInterestRate`.
* On a rate change:

  * If the previous `current_interest_rate` ≠ 0, reconcile first.
  * Update `current_interest_rate` and `desired_interest_rate`.
  * Manage `VaultInterestDetails` presence based on whether interest is enabled (non‑zero).

### Accrual & Reconciliation

* Accrual is **period‑based** from `VaultInterestDetails.period_start` to current block time.
* Computation: `interest = CalculateInterestEarned(principal_amount, current_rate, duration_seconds)`.
* Transfers:

  * **Positive interest:** move `interest` from **reserves (vault account)** → **marker(principal)**.
  * **Negative interest:** move `|interest|` from **marker(principal)** → **reserves (vault account)** (capped at available principal).
* Emit `EventVaultReconcile(principal_before, principal_after, rate, time, interest_earned)`.
* On successful transfer, reset `period_start` to current block time.

`ReconcileVaultInterest` is called:

* Before `SwapIn`, `SwapOut`.
* Before principal/interest admin moves.
* From BeginBlocker for scheduled checks.

### BeginBlocker / EndBlocker behavior

* **BeginBlocker:** `handleVaultInterestTimeouts`

  * For each vault with `expire_time <= now` (or unset) where a new period has elapsed:

    * Check ability to pay or refund over the elapsed duration (`CanPayoutDuration`):

      * If payable/refundable, perform `PerformVaultInterestTransfer`.
      * If not, mark for depletion handling.
  * Reset `period_start` for reconciled vaults to current time.
* **EndBlocker:** `handleReconciledVaults`

  * Partition reconciled vaults:

    * **Payable:** extend `expire_time = now + AutoReconcileTimeout`.
    * **Depleted:** set `current_interest_rate = 0` (keep `desired_interest_rate`), remove `VaultInterestDetails`.

### Estimation (queries)

Estimates simulate accrual **without mutating state**:

* `CalculateVaultTotalAssets` derives a notional `total_assets = principal + accrued_interest_since(period_start)` using block time.
* `EstimateSwapIn`: computes **shares** for a given asset amount against estimated total assets.
* `EstimateSwapOut`: computes **assets** for a given share amount against estimated total assets.
* Both return the block `height` and UTC `time` used for the estimate.

## Queries

* `Vaults(pagination)` → list of `VaultAccount`.
* `Vault(vault_address)` → `VaultAccount`.
* `EstimateSwapIn(vault_address, assets)` → `shares`, `height`, `time`.
* `EstimateSwapOut(vault_address, assets=shares)` → `assets`, `height`, `time`.

## Events

* `EventVaultCreated(vault_address, admin, share_denom, underlying_assets[])`
* `EventSwapIn(owner, amount_in, shares_received, vault_address)`
* `EventSwapOut(owner, shares_burned, amount_out, vault_address)`
* `EventVaultReconcile(vault_address, principal_before, principal_after, rate, time, interest_earned)`
* `EventVaultInterestChange(vault_address, current_rate, desired_rate)`
* `EventInterestDeposit(vault_address, admin, amount)`
* `EventInterestWithdrawal(vault_address, admin, amount)`
* `EventToggleSwapIn(vault_address, admin, enabled)`
* `EventToggleSwapOut(vault_address, admin, enabled)`
* `EventDepositPrincipalFunds(vault_address, admin, amount)`
* `EventWithdrawPrincipalFunds(vault_address, admin, amount)`
* `EventMinInterestRateUpdated(vault_address, admin, min_rate)`
* `EventMaxInterestRateUpdated(vault_address, admin, max_rate)`

## Params

`Params` currently has no fields; reserved for future module‑wide configuration.

## Errors & Validation

* **Marker existence:** `CreateVault` requires existing marker for the `underlying_asset`.
* **Share denom uniqueness:** share marker must not already exist.
* **Admin auth:** all admin endpoints validate `vault.admin`.
* **Asset/denom checks:** swaps and fund moves must match `underlying_asset`; swap‑out shares must match `share_denom`.
* **Toggles:** `swap_in_enabled` / `swap_out_enabled` must be true for respective flows.
* **Interest funding:** positive interest requires sufficient **reserves**; insufficient reserves produce an error on reconciliation.
* **Bounds:** new rate must satisfy optional `min_interest_rate` / `max_interest_rate`.
* **Depletion handling:** if a vault cannot pay/refund during automated checks, interest is set to zero and period tracking is removed.

## Planned Additions

Not yet implemented but planned:

* **AUM fee**: periodic fee in basis points against total assets.
* **Swap‑out fee**: basis‑point fee applied to redemptions (shares → assets).

---
