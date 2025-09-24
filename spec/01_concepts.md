# Vault Module Overview

The `x/vault` module provides a system for tokenized vaults built on Provenance’s marker and account model.
Vaults allow users to deposit underlying assets in exchange for vault shares, redeem those shares later, and participate in configurable interest accrual.
Each vault is configured with both an **underlying asset denom** (the backing collateral) and an optional **payment denom**.
The payment denom provides a secondary unit for payouts and redemptions: users can request to redeem shares into either the underlying asset or the configured payment denom (if supported), with conversions handled via on-chain NAV pricing.
The module manages vault lifecycle, share issuance, redemptions, dual-asset accounting, interest accrual, and time-based job queues for automated processing.
**Bridging:** each vault may designate a single **bridge address** that, when enabled, can mint or burn local share supply as shares move off/on chain. The vault’s `total_shares` is the canonical supply-of-record across chains; local marker supply must never exceed `total_shares`.

## Keeper Responsibilities

The keeper ties together state management, account operations, marker integration, interest reconciliation, and queued jobs.

### Vault Lifecycle

* **CreateVault**: validates an existing marker for the underlying asset, establishes a vault account, and creates the share marker with mint/burn/withdraw permissions.
* **GetVault**: retrieves and validates a vault account by address.
* **Pause/Unpause**: admins can pause a vault, freezing operations and fixing balances, or unpause to resume operations.

### Swap Operations

* **SwapIn**: deposit underlying assets, mint shares, and transfer them to the depositor.
* **SwapOut**: escrow shares and queue a withdrawal job for later payout after a configured delay.

  * Withdrawals are processed safely in EndBlocker to avoid state conflicts.
  * Refunds or auto-pauses are triggered on failures.

### Bridging

* **SetBridgeAddress**: sets the single external address authorized to move share supply across chains.
* **ToggleBridgeEnabled**: enables/disables bridge operations without changing the configured address.
* **BridgeMintShares**: authorized bridge mints local share marker supply and withdraws it to the bridge account, subject to:

  * vault exists and bridge is enabled,
  * signer matches the configured bridge address,
  * denom/amount are valid and positive,
  * capacity check: `current_local_supply + mint_amount ≤ total_shares`.
  * Emits `EventBridgeMintShares`.
* **BridgeBurnShares**: authorized bridge returns shares to the vault’s share marker and burns them, reducing local supply when shares are retired off chain.

  * Requires bridge enabled and authorized signer, valid denom/amount.
  * Emits `EventBridgeBurnShares`.

### Interest Management

* **ReconcileVaultInterest**: ensures accrued interest is applied before any balance-changing action.
* **Positive Interest**: paid from vault reserves into the principal marker.
* **Negative Interest**: refunded from the principal marker into reserves, capped by available funds.
* **Rate Controls**: vaults have configurable current/desired rates, and optional min/max bounds.
* **Queues**: vaults rotate between verification and timeout queues to forecast payout ability and auto-reconcile interest periodically.

### Valuation

* **NAV Calculations**: conversion functions based on Net Asset Value (NAV) between denominations.
* **TVV (Total Vault Value)**: derived from balances at the principal marker, always expressed in underlying units.
* **Pro-Rata Conversions**: deterministic share/asset calculations with floor arithmetic to avoid inflation or over-distribution.
* **Supply Semantics**: `total_shares` is the authoritative supply across chains; local Provenance marker supply may be ≤ `total_shares` when some shares are bridged out. Bridging does not change TVV—only the distribution of shares between local and external holders.

### Queues & Jobs

* **Payout Timeout Queue**: tracks when vaults must be revisited for automatic interest reconciliation.
* **Payout Verification Set**: temporary holding set for vaults awaiting validation after rate changes or reconciliations.
* **Pending Swap-Out Queue**: time-ordered queue of withdrawal requests, processed in EndBlocker. Jobs include owner, vault, shares, redeem denom, and request ID.

### Genesis

* **InitGenesis**: loads vault accounts, queue entries, and validates stored state (including `total_shares`, bridge address, and bridge enabled state).
* **ExportGenesis**: exports all vaults and active queue entries for chain restart or upgrades; preserves `total_shares` as supply-of-record.

### Block Hooks

* **BeginBlocker**: checks vaults with expired timeouts and reconciles or disables interest.
* **EndBlocker**: processes pending swap-out jobs and reconciled vaults safely.

---

## Error Handling & Safety

* **Auto-Pause**: vaults encountering unrecoverable errors during processing are paused automatically, with a stable reason recorded and event emitted.
* **Refund Path**: failed withdrawals attempt to return escrowed shares to the user, with reason codes emitted for transparency.
* **Validation**: strict checks on denoms, admin permissions, share supply, and marker restrictions ensure consistency and prevent misconfiguration.
* **Bridge Safety**: bridge operations require an enabled bridge and the exact configured address; mint capacity is enforced so local supply never exceeds `total_shares`.

---

## High-Level Flow

1. **CreateVault**: admin sets up a new vault.
2. **SwapIn**: users deposit assets → shares minted.
3. **SwapOut**: users escrow shares → queued for payout.
4. **Bridge (optional)**: authorized bridge mints local shares to export supply or burns returned shares to reconcile supply.
5. **Interest**: accrues over time, reconciled on actions or via queues.
6. **Block Processing**:
   * BeginBlocker: runs interest checks and timeouts.
   * EndBlocker: finalizes swap-out jobs and reconciliations.
7. **Admin Tools**: manage interest rates, deposits/withdrawals, pausing/unpausing, bridging controls, and queue interventions.
