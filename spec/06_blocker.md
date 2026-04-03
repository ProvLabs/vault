````markdown
# Vault BeginBlocker / EndBlocker

This document explains how the vault module uses ABCI block hooks to keep vaults healthy over time and to safely fulfill delayed redemptions.

- **BeginBlocker**: periodic interest accrual & AUM fee collection.
- **EndBlocker**: processes pending swap-outs (payouts) and advances interest/fee scheduling state.

---
<!-- TOC 2 2 -->
  - [High-Level Goals](#high-level-goals)
  - [Key Data Structures](#key-data-structures)
  - [BeginBlocker](#beginblocker)
    - [handleVaultInterestTimeouts](#handlevaultinteresttimeouts)
    - [handleVaultFeeTimeouts](#handlevaultfeetimeouts)
  - [EndBlocker](#endblocker)
    - [processPendingSwapOuts](#processpendingswapouts)
    - [handleReconciledVaults](#handlereconciledvaults)
  - [Interest & Fee Accrual](#interest--fee-accrual)
  - [Payout Processing Details](#payout-processing-details)
  - [Forecast Window](#forecast-window)
  - [Paused Vault Behavior](#paused-vault-behavior)
  - [Events & Operational Signals](#events--operational-signals)
  - [Safety & Invariants](#safety--invariants)

---

## High-Level Goals

1. **Apply interest on time** without user transactions:
   - Move positive interest from reserves → principal.
   - Move negative interest from principal → reserves (bounded by principal).
2. **Schedule next accrual** or disable interest if funding is insufficient.
3. **Fulfill swap-out requests** after the per-vault withdrawal delay, with strong safety guarantees:
   - Pay out assets and burn shares, or
   - Refund escrowed shares with a reason.

---

## Key Data Structures

- **PayoutTimeoutQueue**  
  Time-ordered `(timeout, vault)` entries indicating the next time a vault should be reconciled or re-checked.

- **PayoutVerificationSet**  
  Set of vaults that must be (re)validated before rescheduling (e.g., after rate changes or on first accrual).

- **PendingSwapOutQueue**  
  Time-ordered `(dueTime, requestID, vaultAddr)` entries holding user swap-out requests to be paid later.

---

## BeginBlocker

At the start of each block, the module reconciles interest for vaults whose **timeout has elapsed**.

### handleVaultInterestTimeouts

Processing model (safe “collect-then-mutate”):

1. **Collect due entries** from `PayoutTimeoutQueue` with `timeout <= now`.

   * Skip paused vaults.
2. **Dequeue** each collected `(timeout, vault)` before processing (prevents iterator invalidation).
3. **For each vault**:

   * Compute `periodDuration` as `timeout - PeriodStart` (fallback to `now - PeriodStart` if needed).
   * **Check ability to pay/refund** over `periodDuration` via `CanPayInterestDuration`.

     * If **insufficient** → mark **depleted**.
     * If **sufficient** → execute `PerformVaultInterestTransfer` (emits `EventVaultReconcile`) and mark **reconciled**.
4. **Advance state**:

   * For **reconciled** vaults → `resetVaultInterestPeriods` (starts new period and enqueues next timeout).
   * For **depleted** vaults → `handleDepletedVaults` (sets `current_rate = "0"`; interest disabled, desired preserved).

**Skips paused vaults.** They remain in place until unpaused.

### handleVaultFeeTimeouts

Reconciles the configurable AUM fee for vaults whose fee timeout has elapsed.

1. **Collect due entries** from `VaultFeeTimeoutQueue` with `timeout <= now`.
2. **Dequeue** each collected entry from the main context before processing to ensure it is not retried if a transient error occurs.
3. **Attempt Atomic Reconciliation** (via `atomicallyReconcileFee` using `CacheContext`):
   - **PerformVaultFeeTransfer**:
     - Computes fee based on **Gross TVV** and the vault's `aum_fee_bips`.
     - Collects from principal marker into the configured `tech_fee_address`.
     - **Success (Partial/Full Collection)**: If the marker lacks liquidity, the uncollected remainder is recorded in `outstanding_aum_fee`. This is considered a successful transfer.
     - **Schedules next fee timeout** and commits state changes.
   - **Failure (Transient Error)**:
     - If reconciliation fails (e.g., missing NAV for denom conversion), the `CacheContext` is discarded.
     - **Rescheduling**: The vault's fee timeout is rescheduled to the next block window (`rescheduleFeeTimeout`) on the main context to preserve accrued fees while preventing block-to-block retry loops.

---

## EndBlocker

Ordering is intentional:

1. **ProcessPendingSwapOuts** – fulfill user withdrawals first.
2. **handleReconciledVaults** – then rotate vaults that recently reconciled or changed interest into their next schedule.

### ProcessPendingSwapOuts

At block end, the module fulfills **due swap-out requests**:

To prevent a large queue from consuming excessive block time and memory, a maximum of `MaxSwapOutBatchSize` (currently 100) requests are processed per block.

1. **Collect due requests** from `PendingSwapOutQueue` with `dueTime <= now`.

   * Skip paused vaults; they remain queued.
2. **Process each job** (see “Payout Processing Details”).
   - Each job is processed within its own **CacheContext**.
   - Failed payouts (recoverable) are rolled back atomically and the user is refunded.
   - Successful payouts commit their state changes.
   - This ensures failures do not leave the vault in an inconsistent state and do not interfere with other jobs in the same block.

   * Missing vault → dequeue & skip (logged).
   * Paused vault → leave queued (not dequeued).
3. Errors:

   * **Recoverable** (e.g., insufficient funds, attribute check failure) → attempt **refund** and emit `EventSwapOutRefunded`.
   * **Critical** after payout (e.g., failed share burn) → **auto-pause** vault with a stable reason.

### handleReconciledVaults

This advances vaults from the **verification set**:

1. **Collect keys** from `PayoutVerificationSet`; skip paused vaults.
2. **Remove** each from the set (before processing).
3. **Partition** into:

   * **Payable**: can cover the **forecast window** (see below) → re-enqueue next timeout (`SafeEnqueueTimeout`).
   * **Depleted**: cannot cover forecast → disable interest (`current_rate = "0"`; desired preserved).

---

## Interest & Fee Accrual

* **ReconcileVault**
  No-op if paused. Ensures both interest and AUM fees are reconciled before any balance-changing action. Uses a single `CacheContext` to ensure both transfers are atomic.

* **PerformVaultInterestTransfer**
  Computes `interestEarned = f(principal, currentRate, duration)`.

  * **Positive** → transfer from **reserves (vault account)** → **principal (marker account)**.
  * **Negative** → refund from **principal** → **reserves** (bounded by available principal).
    Emits `EventVaultReconcile`.

* **PerformVaultFeeTransfer**
  Computes the annual AUM fee based on the vault's `aum_fee_bips` and **Gross TVV**.
  - Collects fee in the configured `payment_denom`.
  - Transfers from **principal (marker account)** → module `tech_fee_address`.
  - Caps collection at available `payment_denom` balance.
  - Emits `EventVaultFeeCollected`.

* **UpdateInterestRates**
  Sets `current_rate` and `desired_rate`, emits `EventVaultInterestChange`, and persists the account.

---

## Payout Processing Details

* **processSingleWithdrawal** (called from `processPendingSwapOuts`)

  1. **ReconcileVault** (reconcile both interest and AUM fees using a single `CacheContext`/atomic transfer).
  2. Convert **shares → payout coin** (`underlying_asset` or optional **payment denom**), using current NAV and pro-rata TVV.
  3. **Payout assets** from **principal (marker)** → **owner** with transfer-agent context.
  4. **Burn shares**: move escrowed shares **vault → principal**, then `BurnCoin`.
  5. Emit `EventSwapOutCompleted`.

* **refundWithdrawal**
  On recoverable failure before payout, return escrowed shares **vault → owner** and emit `EventSwapOutRefunded(reason=…)`.

* **Critical errors & auto-pause**
  If a critical error occurs after payout (e.g., burn failed) or the refund itself fails, the vault is **auto-paused** with a stable reason; further user ops are blocked until admin intervention.

---

## Forecast Window

* **AutoReconcilePayoutDuration = 24 hours**
  Used when deciding if a vault remains **payable**.
  `handleReconciledVaults` calls `partitionVaults` which uses `CanPayInterestDuration` over this window:

  * **Positive interest** → must have reserves ≥ forecasted interest.
  * **Negative interest** → principal must be > 0.
  * Zero interest → always payable.

---

## Paused Vault Behavior

* **BeginBlocker / EndBlocker** both **skip** paused vaults:

  * Interest is **not** reconciled while paused.
  * Pending swap-outs remain **queued** (not dequeued) until unpaused.
* Admins can still:

  * **Deposit/Withdraw principal** (only while paused).
  * **ExpeditePendingSwapOut** (has no effect until unpaused; job stays queued).

---

## Events & Operational Signals

* **Interest**: `EventVaultReconcile`, `EventVaultInterestChange`
* **Swap-outs**:

  * Enqueue: `EventSwapOutRequested{request_id,…}`
  * Success: `EventSwapOutCompleted{request_id, assets,…}`
  * Refund: `EventSwapOutRefunded{request_id, reason,…}`
  * Admin expedite: `EventPendingSwapOutExpedited{request_id}`
* **Pause lifecycle**: `EventVaultPaused`, `EventVaultUnpaused`

**Client pattern for swap-out**
Submit `MsgSwapOut` → capture `request_id` → watch for `Completed` or `Refunded` with that `request_id` after the vault’s withdrawal delay.

---

## Safety & Invariants

* **Collect-then-mutate** iteration for all queues/sets (prevents iterator invalidation).
* **Dequeue before mutate** when processing due items; skipped/paused items remain enqueued.
* **Reconcile before supply-affecting ops** (e.g., swap-out payout) to keep NAV and TVV consistent.
* **Flooring** in conversions prevents over-distribution or share inflation.
* **Auto-pause on critical errors** creates a safe dead-stop until an admin resolves the issue.

---

```
::contentReference[oaicite:0]{index=0}
```
