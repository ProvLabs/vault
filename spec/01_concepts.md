# Vault Module Overview

The `x/vault` module provides a system for tokenized vaults built on Provenance’s marker and account model.  
Vaults allow users to deposit underlying assets in exchange for vault shares, redeem those shares later, and participate in configurable interest accrual.  
Each vault is configured with both an **underlying asset denom** (the backing collateral) and an optional **payment denom**.  
The payment denom provides a secondary unit for payouts and redemptions: users can request to redeem shares into either the underlying asset or the configured payment denom (if supported), with conversions handled via on-chain NAV pricing.  
The module manages vault lifecycle, share issuance, redemptions, dual-asset accounting, interest accrual, and time-based job queues for automated processing.  
Total share supply is tracked on the vault as **total_shares**, the authoritative supply-of-record across chains; local marker supply must never exceed this amount.

## Keeper Responsibilities

The keeper ties together state management, account operations, marker integration, interest reconciliation, and queued jobs.

### Vault Lifecycle
- **CreateVault**: validates an existing marker for the underlying asset, establishes a vault account, and creates the share marker with mint/burn/withdraw permissions.
- **GetVault**: retrieves and validates a vault account by address.
- **Pause/Unpause**: admins can pause a vault, freezing operations and fixing balances, or unpause to resume operations.
- **Bridge Controls**: configure a single **bridge address** and **enable/disable** bridging; capacity checks ensure local marker supply never exceeds `total_shares`.

### Swap Operations
- **SwapIn**: deposit underlying assets, mint shares, and transfer them to the depositor.
- **SwapOut**: escrow shares and queue a withdrawal job for later payout after a configured delay.  
  - Withdrawals are processed safely in EndBlocker to avoid state conflicts.
  - Refunds or auto-pauses are triggered on failures.
- **BridgeMintShares**: authorized bridge mints local shares up to the capacity defined by `total_shares - local_supply`, then transfers them to the bridge.
- **BridgeBurnShares**: authorized bridge returns shares to the vault and burns them from the marker, reducing local supply without changing `total_shares`.

### Interest Management
- **ReconcileVaultInterest**: ensures accrued interest is applied before any balance-changing action.
- **Positive Interest**: paid from vault reserves into the principal marker.
- **Negative Interest**: refunded from the principal marker into reserves, capped by available funds.
- **Rate Controls**: vaults have configurable current/desired rates, and optional min/max bounds.
- **Queues**: vaults rotate between verification and timeout queues to forecast payout ability and auto-reconcile interest periodically.

### Valuation
- **NAV Calculations**: conversion functions based on Net Asset Value (NAV) between denominations.
- **TVV (Total Vault Value)**: derived from balances at the principal marker, always expressed in underlying units.
- **Pro-Rata Conversions**: deterministic share/asset calculations with floor arithmetic to avoid inflation or over-distribution.
- **Share Supply-of-Record**: `total_shares` tracks all issued shares (local and bridged); local marker supply is a subset bounded by `total_shares`.

### Queues & Jobs
- **Payout Timeout Queue**: tracks when vaults must be revisited for automatic interest reconciliation.
- **Payout Verification Set**: temporary holding set for vaults awaiting validation after rate changes or reconciliations.
- **Pending Swap-Out Queue**: time-ordered queue of withdrawal requests, processed in EndBlocker. Jobs include owner, vault, shares, redeem denom, and request ID.

### Genesis
- **InitGenesis**: loads vault accounts, queue entries, and validates stored state.
- **ExportGenesis**: exports all vaults and active queue entries for chain restart or upgrades.
- **Bridge Fields**: genesis includes `total_shares`, `bridge_address`, and `bridge_enabled`; validation asserts local marker supply does not exceed `total_shares`.

### Block Hooks
- **BeginBlocker**: checks vaults with expired timeouts and reconciles or disables interest.
- **EndBlocker**: processes pending swap-out jobs and reconciled vaults safely.

---

## Error Handling & Safety

- **Auto-Pause**: vaults encountering unrecoverable errors during processing are paused automatically, with a stable reason recorded and event emitted.
- **Refund Path**: failed withdrawals attempt to return escrowed shares to the user, with reason codes emitted for transparency.
- **Validation**: strict checks on denoms, admin permissions, share supply, and marker restrictions ensure consistency and prevent misconfiguration.
- **Supply Guardrails**: bridge mints beyond capacity are rejected; burns require the configured bridge address.

---

## High-Level Flow

1. **CreateVault**: admin sets up a new vault.
2. **SwapIn**: users deposit assets → shares minted.
3. **SwapOut**: users escrow shares → queued for payout.
4. **Interest**: accrues over time, reconciled on actions or via queues.
5. **Block Processing**:
   - BeginBlocker: runs interest checks and timeouts.
   - EndBlocker: finalizes swap-out jobs and reconciliations.
6. **Admin Tools**: manage interest rates, deposits/withdrawals, pausing/unpausing, and queue interventions.
7. **Bridge Ops (optional)**: authorized bridge mints/burns local supply under `total_shares` capacity to facilitate cross-chain share movement.
