# Vault Module Overview

The `x/vault` module provides a system for tokenized vaults built on Provenance’s marker and account model.  
Vaults allow users to deposit underlying assets in exchange for vault shares, redeem those shares later, and participate in configurable interest accrual.  
Each vault is configured with both an **underlying asset denom** (the backing collateral) and an optional **payment denom**.  
The payment denom provides a secondary unit for payouts and redemptions: users can request to redeem shares into either the underlying asset or the configured payment denom (if supported), with conversions handled via on-chain NAV pricing.  
The module manages vault lifecycle, share issuance, redemptions, dual-asset accounting, interest accrual, AUM fee collection, and time-based job queues for automated processing.  
Total share supply is tracked on the vault as **total_shares**, the authoritative supply-of-record across chains; local marker supply must never exceed this amount.

---
<!-- TOC -->
- [Key Definitions](#key-definitions)
  - [Marker Authority Rules](#marker-authority-rules)
  - [Special Case: YLDS Peg Mode](#special-case-ylds-peg-mode)
- [Keeper Responsibilities](#keeper-responsibilities)
  - [Vault Lifecycle](#vault-lifecycle)
  - [Swap Operations](#swap-operations)
  - [Interest & Fee Management](#interest--fee-management)
  - [Valuation](#valuation)
  - [Queues & Jobs](#queues--jobs)
  - [Genesis](#genesis)
  - [Block Hooks](#block-hooks)
- [Bridge Trust Model & Supply-of-Record](#bridge-trust-model--supply-of-record)
- [Internal NAV & Multi-Asset Settlement](#internal-nav--multi-asset-settlement)
- [Error Handling & Safety](#error-handling--safety)
- [High-Level Flow](#high-level-flow)

---

## Key Definitions

- **Underlying Asset**: the base denom that defines vault value and payouts. TVV is always expressed in this unit.  
- **Payment Denom (Secondary)**: an optional denom configured on a vault. It may be a normal swappable token (e.g., `uusdc`) or a restricted **receipt token** used for accounting. Swapability is determined by marker configuration.  
- **Receipt Token**: a restricted marker that may be set as the payment denom. The only account with transfer authority is the holder of the receipt itself, which represents deployed capital (e.g., receipts into a fund). User swap-out to a receipt token is not possible unless the marker explicitly grants transfer authority. 
- **Principal**: the vault’s total assets held in the **share marker account**, including underlying balances and any payment denom balances (normal or receipt).
- **Reserves**: the vault account balance used to pay positive interest or receive refunds from negative interest.
- **TVV (Total Vault Value)**: the value of all principal assets, computed and reported in the underlying unit.
    - **Gross TVV**: The literal sum of all assets sitting in the principal marker. Used for interest and fee accruals.
    - **Net TVV**: Gross TVV minus **Outstanding AUM Fees**. This is the authoritative value used for share pricing (NAV) and user-facing valuation.
- **AUM Technology Fee**: a 15 bps (0.15% annual) fee collected from the vault principal to support protocol maintenance. It is accrued continuously and collected in the vault's configured `payment_denom`.
- **NAV**: conversion rate between denoms, used for valuation and conversions, subject to special-case rules.
- **Internal NAV Table**: per-vault price entries (`price` for `volume` units of a denom) that are the **sole source of truth** for the valuation engine's conversions. The module never reads external oracles or marker NAVs at valuation time.
- **NAV Authority**: an optional per-vault address authorized to maintain the internal NAV table via `UpdateVaultNAV`. The vault admin acts as NAV authority when unset; the admin rotates it via `UpdateNAVAuthority`.

- **Total Shares**: the canonical supply-of-record across chains. Local marker supply must never exceed `total_shares`.  
- **Asset Manager**: an optional delegated operator address with limited management authority. When set, this account can perform certain administrative actions (e.g., fund management operations) in addition to the vault admin. If unset, only the vault admin holds these permissions.

### Marker Authority Rules

All user I/O must satisfy **both** layers:  
1) **Vault-level gates** — the vault must be unpaused and the relevant toggle (`SwapIn`/`SwapOut`) must be enabled.  
2) **Marker permissions** — the marker for the target denom (including the **underlying**) must allow the transfer (attributes, restrictions, transfer authority).  

Examples:  
- `uylds.fcc` is a restricted marker with required attributes; users must satisfy those attributes to deposit/withdraw even when vault toggles are enabled.  
- Receipt tokens typically deny public transfers; therefore user swap-out to a receipt token is not possible regardless of vault toggles.

### Special Case: YLDS Peg Mode

If a vault’s underlying is set to `uylds.fcc`, all conversions are treated as **1:1** with YLDS regardless of NAV.  
This means any configured payment denom (normal token or receipt token) is valued **1:1** with `uylds.fcc` for TVV and estimates.

---

## Keeper Responsibilities

The keeper ties together state management, account operations, marker integration, interest reconciliation, and queued jobs.

### Vault Lifecycle
- **CreateVault**: validates an existing marker for the underlying asset, establishes a vault account, and creates the share marker with mint/burn/withdraw permissions.
- **GetVault**: retrieves and validates a vault account by address.
- **Pause/Unpause**: admins can pause a vault, freezing operations and fixing balances, or unpause to resume operations.
- **Bridge Controls**: configure a single **bridge address** and **enable/disable** bridging; capacity checks ensure local marker supply never exceeds `total_shares`.
- **SetAssetManager**: assigns or clears the optional delegated **asset manager** address. When set, both the admin and asset manager may perform privileged actions on the vault.

### Swap Operations
- **SwapIn**: deposit underlying assets, mint shares, and transfer them to the depositor.
- **SwapOut**: escrow shares and queue a withdrawal job for later payout after a configured delay.  
  - Withdrawals are processed safely in EndBlocker to avoid state conflicts.
  - Refunds or auto-pauses are triggered on failures.
- **BridgeMintShares**: authorized bridge mints local shares up to the capacity defined by `total_shares - local_supply`, then transfers them to the bridge.
- **BridgeBurnShares**: authorized bridge returns shares to the vault and burns them from the marker, reducing local supply without changing `total_shares`.

### Interest & Fee Management
- **ReconcileVault**: ensures accrued interest is applied and AUM fees are collected before any balance-changing action.
- **Positive Interest**: paid from vault reserves into the principal marker.
- **Negative Interest**: refunded from the principal marker into reserves, capped by available funds.
- **AUM Technology Fee**: 15 bps annual fee collected from the principal marker into the configured ProvLabs collection address.
- **Rate Controls**: vaults have configurable current/desired rates, and optional min/max bounds.
- **Queues**: vaults rotate between verification, interest timeout, and fee timeout queues to forecast payout ability and auto-reconcile state periodically.

### Valuation
- **NAV Calculations**: conversion functions based on Net Asset Value (NAV) between denominations.  
- **TVV (Total Vault Value)**: derived from balances at the principal marker, always expressed in underlying units.  
- **Pro-Rata Conversions**: deterministic share/asset calculations with floor arithmetic to avoid inflation or over-distribution.  
- **Share Supply-of-Record**: `total_shares` tracks all issued shares (local and bridged); local marker supply is a subset bounded by `total_shares`.

### Queues & Jobs
- **Payout Timeout Queue**: tracks when vaults must be revisited for automatic interest reconciliation.
- **Fee Timeout Queue**: tracks when vaults must be revisited for automatic AUM fee collection.
- **Payout Verification Set**: temporary holding set for vaults awaiting validation after rate changes or reconciliations.
- **Pending Swap-Out Queue**: time-ordered queue of withdrawal requests, processed in EndBlocker. Jobs include owner, vault, shares, redeem denom, and request ID.

### Genesis
- **InitGenesis**: loads vault accounts, queue entries, and validates stored state.
- **ExportGenesis**: exports all vaults and active queue entries for chain restart or upgrades.
- **Bridge Fields**: genesis includes `total_shares`, `bridge_address`, and `bridge_enabled`; validation asserts local marker supply does not exceed `total_shares`.
- **Asset Manager Field**: genesis includes the optional `asset_manager` field for each vault, which may be empty if not configured.

### Block Hooks
- **BeginBlocker**: checks vaults with expired timeouts and reconciles or disables interest.
- **EndBlocker**: processes pending swap-out jobs and reconciled vaults safely.

---

## Bridge Trust Model & Supply-of-Record

Bridging lets vault shares move across chains. The on-chain accounting model and its trust boundary are as follows.

- **`total_shares` is the cross-chain supply-of-record.** It counts every issued share, whether currently held locally (local marker/bank supply) or on a remote chain. Local supply is always a subset bounded by `total_shares`. Only `SwapIn` (mint) and the redemption payout path (`processSingleWithdrawal`, burn) change `total_shares`, because only those create or destroy shares outright.

- **Bridge ops move the local/remote split; they do not change `total_shares`.** `BridgeMintShares` re-materializes shares that already exist remotely (local supply rises toward `total_shares`); `BridgeBurnShares` reflects shares leaving for a remote chain (local supply falls). Neither mints new supply nor destroys shares — they only shift where existing shares live. This is why `BridgeBurnShares` deliberately does **not** perform the `SafeSub`+persist of `total_shares` that the local redemption path does: a bridged-out share still exists, just elsewhere.

- **Capacity is the only on-chain guardrail.** A mint is rejected when it would push local supply above `total_shares` (`available = total_shares - local_supply`). A burn lowering local supply re-widens that capacity by exactly the burned amount, so a later mint can bring those same shares back. Consequently, NAV per share (`Net TVV / total_shares`) is invariant across bridge mint/burn — they cannot dilute holders.

- **Trust boundary (accepted assumption).** Both handlers are gated solely on the configured `bridge_address`; there is **no on-chain reconciliation** that a local mint corresponds to a genuine remote burn (or vice versa). Keeping the local/remote split honest is the responsibility of the off-chain bridge operator. A compromised or dishonest bridge key could mint local supply up to `total_shares` without real remote backing; this is bounded by `total_shares` (it can never inflate beyond the supply-of-record or move NAV per share) and is an accepted operator-trust assumption, not an on-chain accounting flaw. Admins can disable bridging (`bridge_enabled`) or rotate `bridge_address` to contain a compromised operator.

## Internal NAV & Multi-Asset Settlement

Vaults can take in **external assets** beyond the underlying and payment denoms, settling them peer-to-peer against the vault's payment denom. Two pieces make this work: the **internal NAV table** (the module's own price book) and the **p2p settlement workflow** built on `x/exchange` payments.

### Internal NAV Table

Each vault carries its own table of price entries, one per asset denom. An entry records `price` (a coin in one of the vault's accepted denoms) for `volume` units of the denom; the per-unit value is `price / volume`, kept as an exact fraction. The valuation engine converts denoms **exclusively** through this table — no oracle or marker-module reads happen at valuation time, so pricing is deterministic and admin-auditable.

Entries are written by four paths:

1. **Creation seed** — an optional `InitialVaultNAV` bootstraps the payment denom's price when the vault is created.
2. **NAV authority updates** — the configured `nav_authority` (the admin when unset) maintains entries via `UpdateVaultNAV`.
3. **Settlements** — each `AcceptAsset` records the realized settlement price as the asset denom's entry (and removes the entry when an outbound settlement drains the denom from the principal).
4. **Migration seeding** — a one-time upgrade migration seeded entries from existing marker-module NAVs.

NAV upserts from paths 2 and 3 are also **published one-way to the marker module**, attributed to the vault address, so downstream marker-NAV consumers can distinguish vault-originated prices. Removals are internal-only: when a settlement drains a denom and its entry is deleted, the marker NAV is left as-is — publishing simply stops. The vault never reads marker NAVs back — the internal table remains authoritative.

### P2P Settlement Workflow

External counterparties propose trades by creating `x/exchange` **payments** that target the vault, escrowing their side. The vault's authority then settles (`AcceptAsset`) or declines (`RejectAsset`) each pending payment; rejection refunds the counterparty's escrow.

Exactly one payment leg must carry the vault's payment denom, which determines the **direction**: *inbound* (vault receives an external asset, pays payment denom) or *outbound* (vault pays out an asset, receives payment denom). Each leg must be a single coin, and the asset denom must be a registered marker.

Settlement is atomic and layers several protections:

- **Reconcile-first** — accrued interest and fees settle against the pre-settlement TVV.
- **Exact-price guardrail** — when an internal NAV entry exists for the asset, the settlement legs must match its price exactly (cross-multiplied, no rounding tolerance). A first acquisition has no entry and skips the check; thereafter the authority must update the NAV (`UpdateVaultNAV`) before settling at a different price. This makes every price change an explicit, evented action rather than a side effect of trade flow.
- **Price recording** — the realized settlement price becomes the asset's internal NAV entry and is published to the marker module. When an outbound settlement empties the principal of the asset, the entry is removed so a stale price cannot linger.

### Valuation Scope

TVV currently sums only the vault's **accepted denoms** (underlying and payment denom) held at the principal marker. External assets acquired through settlement sit in the principal and are priced in the internal NAV table, but are **not yet included** in TVV; extending valuation to NAV-priced multi-asset holdings is future work.

---

## Error Handling & Safety

- **Auto-Pause**: vaults encountering unrecoverable errors during processing are paused automatically, with a stable reason recorded and event emitted.
- **Refund Path**: failed withdrawals attempt to return escrowed shares to the user, with reason codes emitted for transparency.
- **Validation**: strict checks on denoms, admin permissions, share supply, and marker restrictions ensure consistency and prevent misconfiguration.
- **Supply Guardrails**: bridge mints beyond capacity are rejected; burns require the configured bridge address.
- **Delegated Authority**: when an asset manager is set, either the admin or asset manager may perform operations that require vault authority. If cleared, only the admin retains this capability.

---

## High-Level Flow

1. **CreateVault**: admin sets up a new vault.
2. **SwapIn**: users deposit assets → shares minted.
3. **SwapOut**: users escrow shares → queued for payout.
4. **Interest**: accrues over time, reconciled on actions or via queues.
5. **Block Processing**:  
   - BeginBlocker: runs interest checks and timeouts.  
   - EndBlocker: finalizes swap-out jobs and reconciliations.
6. **Admin Tools**: manage interest rates, deposits/withdrawals, pausing/unpausing, queue interventions, and assign asset managers.
7. **Bridge Ops (optional)**: authorized bridge mints/burns local supply under `total_shares` capacity to facilitate cross-chain share movement.
8. **P2P Settlement (optional)**: counterparties propose `x/exchange` payments targeting the vault; the admin or asset manager accepts (settling at the internal-NAV price and recording the result) or rejects (refunding escrow).
