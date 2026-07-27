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
    - [Retry & Backoff](#retry--backoff)
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

1. **Collect due entries** from `PayoutTimeoutQueue` with `timeout <= now`, visiting at most
   `MaxInterestTimeoutsPerBlock` (currently 100) entries per block; the remainder stays due
   for later blocks.

   * Dequeue paused vaults without reconciling them (they count against the visit budget).
2. **Dequeue** each collected `(timeout, vault)` before processing (prevents iterator invalidation).
3. **For each vault**:

   * Compute `periodDuration` as `timeout - PeriodStart` (fallback to `now - PeriodStart` if needed).
   * **Check ability to pay/refund** over `periodDuration` via `CanPayInterestDuration`.

     * If **insufficient** → mark **depleted**.
     * If **sufficient** → execute `PerformVaultInterestTransfer` (emits `EventVaultReconcile`) and mark **reconciled**.
4. **Advance state**:

   * For **reconciled** vaults → `SafeEnqueuePayoutTimeout` (starts new period and enqueues next timeout).
   * For **depleted** vaults → `handleDepletedVaults` (sets `current_rate = "0"`; interest disabled, desired preserved).

**Never reconciles paused vaults.** Pausing already dequeues the vault; any entry still present
(filed under another key, or a vault paused after the walk began) is dequeued on sight so it does not
camp at the front of the queue. A failed dequeue is logged and the entry is retried on a later block.

### handleVaultFeeTimeouts

Reconciles the 15 bps AUM technology fee for vaults whose fee timeout has elapsed.

1. **Collect due entries** from `VaultFeeTimeoutQueue` with `timeout <= now`, visiting at most
   `MaxFeeTimeoutsPerBlock` (currently 100) entries per block; the remainder stays due for
   later blocks. Paused vaults count against the budget and are dequeued without collecting a fee;
   a failed dequeue is logged and retried on a later block.
2. **Dequeue** each collected entry from the main context before processing to ensure it is not retried if a transient error occurs.
3. **Attempt Atomic Reconciliation** (via `atomicallyReconcileFee` using `CacheContext`):
   - **PerformVaultFeeTransfer**:
     - Computes fee based on **Gross TVV**.
     - Collects from principal marker into the configured ProvLabs collection address.
     - **Success (Partial/Full Collection)**: If the marker lacks liquidity, the uncollected remainder is recorded in `outstanding_aum_fee`. This is considered a successful transfer.
     - **Schedules next fee timeout** and commits state changes.
   - **Failure (Transient Error)**:
     - If reconciliation fails (e.g., missing NAV for denom conversion), the `CacheContext` is discarded.
     - **Rescheduling**: The vault's fee timeout is rescheduled to the next block window (`RescheduleFeeTimeout`) on the main context to preserve accrued fees while preventing block-to-block retry loops.
---

## EndBlocker

Ordering is intentional:

1. **ProcessPendingSwapOuts** – fulfill user withdrawals first.
2. **handleReconciledVaults** – then rotate vaults that recently reconciled or changed interest into their next schedule.

### ProcessPendingSwapOuts

At block end, the module fulfills **due swap-out requests**:

To prevent a large queue from consuming excessive block time and memory, a maximum of `MaxSwapOutBatchSize` (currently 100) queue entries are visited per block. Every visited entry counts against the budget, including entries for paused vaults.

1. **Collect due requests** from `PendingSwapOutQueue` with `dueTime <= now`, up to the batch budget.
2. **Process each job** (see “Payout Processing Details”).
   - Each job is processed within its own **CacheContext**.
   - Failed payouts (recoverable) are rolled back atomically and the user is refunded.
   - Successful payouts commit their state changes.
   - This ensures failures do not leave the vault in an inconsistent state and do not interfere with other jobs in the same block.

   * Missing vault → dequeue & skip (logged).
   * Paused vault → atomically dequeue & refund escrowed shares (`EventSwapOutRefunded{ reason = "vault_paused" }`), so paused entries cannot camp at the front of the queue and starve processable requests. If the refund fails, nothing is committed and the request is re-keyed to a later retry time (see [Retry & Backoff](#retry--backoff)).
3. Errors:

   * **Recoverable** (e.g., insufficient funds, attribute check failure) → attempt **refund** and emit `EventSwapOutRefunded`.
   * **Critical** after payout (e.g., failed share burn) → **auto-pause** vault with a stable reason.

### handleReconciledVaults

This advances vaults from the **verification set**:

1. **Collect keys** from `PayoutVerificationSet`, visiting at most `MaxPayoutVerificationsPerBlock`
   (currently 100) entries per block; paused vaults are removed from the set without being processed
   (they count against the visit budget), and a failed removal is logged and retried on a later block.
2. **Remove** each from the set (before processing).
3. **Partition** into:

   * **Payable**: can cover the **forecast window** (see below) → re-enqueue next timeout (`SafeEnqueuePayoutTimeout`).
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
  Computes the 15 bps (0.15% annual) technology fee based on **Gross TVV**.
  - Collects fee in the vault's `underlying_asset`.
  - Transfers from **principal (marker account)** → ProvLabs collection address.
  - Caps collection at available `underlying_asset` balance.
  - Emits `EventVaultFeeCollected`.

* **UpdateInterestRates**
  Sets `current_rate` and `desired_rate`, emits `EventVaultInterestChange`, and persists the account.

---

## Payout Processing Details

* **processSingleWithdrawal** (called from `processPendingSwapOuts`)

  1. **ReconcileVault** (reconcile both interest and AUM fees using a single `CacheContext`/atomic transfer).
  2. Convert **shares → payout coin** (always the `underlying_asset`), using current NAV and pro-rata TVV.
  3. **Payout assets** from **principal (marker)** → **owner** with transfer-agent context.
  4. **Burn shares**: move escrowed shares **vault → principal**, then `BurnCoin`.
  5. Emit `EventSwapOutCompleted`.

* **refundWithdrawal**
  On recoverable failure before payout, return escrowed shares **vault → owner** and emit `EventSwapOutRefunded(reason=…)`.

* **Critical errors & auto-pause**
  If a critical error occurs after payout (e.g., burn failed) or the refund itself fails, the vault is **auto-paused** with a stable reason; further user ops are blocked until admin intervention. Auto-pause emits `EventVaultPaused` with `forced = true` and the critical error captured in `reason` (it leaves `forced_error` empty); the manual `PauseVault` path produces the same `forced = true` signal only when called with `force = true`, recording the tolerated error in `forced_error` instead.

### Retry & Backoff

A request that can neither be settled nor refunded stays queued, because the entry is the record of who is owed the escrowed shares. It is not left under its original key: the failures that reach this path (a deactivated share marker, an owner added to a deny list, an owner who lost a required attribute) are deterministic, so an entry left at the front of the queue would be re-collected and re-fail on every block, and enough of them would consume the whole batch budget and stall the vault's queue.

Every path that preserves a request therefore re-keys it:

1. `failure_count` on the request is incremented and stored.
2. The entry is re-filed under `now + backoff(failure_count)`, which puts it behind the work that is currently due. The re-key is atomic; if it fails, the entry is left in place and retried on the next block.
3. `EventSwapOutRetryScheduled{ request_id, reason, failure_count, retry_time }` is emitted.

The delay grows with the failure count and is capped, so a permanently failing request is still revisited at a cost the budget can absorb:

| Failure count | Delay |
| --- | --- |
| 1 (`SwapOutImmediateRetries`) | none, retries on the next block |
| 2 | `SwapOutRetryBackoffBase` (10 minutes) |
| 3, 4, … | doubles each time |
| capped at | `SwapOutRetryBackoffMax` (6 hours) |

`MsgExpeditePendingSwapOut` clears `failure_count` and re-keys the entry to time 0, which is how an operator forces an immediate retry after fixing the underlying cause. Note that a re-keyed request reports its retry time as the `timeout` in the `PendingSwapOuts` and `VaultPendingSwapOuts` queries, so a rising `failure_count` there marks escrow that needs attention.

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

* **BeginBlocker / EndBlocker** do not process paused vaults:

  * Interest and AUM fees are **not** reconciled while paused. Pausing clears both period starts and
    removes the vault from the `PayoutVerificationSet` and from the `PayoutTimeoutQueue` and
    `VaultFeeTimeoutQueue` entries keyed by its recorded timeouts, so a paused vault normally holds no
    queue entries and accrues nothing. Removal is keyed rather than scanned to keep pause
    constant-time, so an entry filed under any other key survives until a blocker dequeues it. Forced
    and automatic pauses tolerate a failed removal, logging it rather than aborting the pause.
    Unpausing re-arms both: payout verification for interest and a fresh fee timeout, each starting at
    the unpause block time, so the paused span is never charged.
  * Pending swap-outs that come due while paused are **dequeued and refunded** with
    `EventSwapOutRefunded{ reason = "vault_paused" }`; owners resubmit after unpause.
* A paused vault freezes its value at the `PausedBalance` snapshot, so operations that would change that value are rejected:

  * **UpdateVaultNAV** is rejected — a NAV write would assert a price the frozen vault ignores until unpause.
  * **AcceptAsset** is rejected — settlement moves principal funds and the vault's value.
  * **RejectAsset** remains available — it only cancels a pending payment and refunds the source's escrow, with no vault state change.
* Admins can still:

  * **Deposit/Withdraw principal** (only while paused).
  * **ExpeditePendingSwapOut** (an expedited job on a paused vault becomes due immediately and is refunded at the next block).

---

## Events & Operational Signals

* **Interest**: `EventVaultReconcile`, `EventVaultInterestChange`
* **Swap-outs**:

  * Enqueue: `EventSwapOutRequested{request_id,…}`
  * Success: `EventSwapOutCompleted{request_id, assets,…}`
  * Refund: `EventSwapOutRefunded{request_id, reason,…}`
  * Retry deferred: `EventSwapOutRetryScheduled{request_id, reason, failure_count, retry_time}`
  * Admin expedite: `EventPendingSwapOutExpedited{request_id}`
* **Pause lifecycle**: `EventVaultPaused`, `EventVaultUnpaused`

**Client pattern for swap-out**
Submit `MsgSwapOut` → capture `request_id` → watch for `Completed` or `Refunded` with that `request_id` after the vault’s withdrawal delay.

---

## Safety & Invariants

* **Collect-then-mutate** iteration for all queues/sets (prevents iterator invalidation).
* **Per-block visit budgets** on every queue/set walk keep block execution time bounded regardless of backlog size.
* **Dequeue before mutate** when processing due items; paused vaults are dequeued at pause and again by the blockers if any entry survives, and paused swap-out jobs are dequeued and refunded.
* **Reconcile before supply-affecting ops** (e.g., swap-out payout) to keep NAV and TVV consistent.
* **Flooring** in conversions prevents over-distribution or share inflation.
* **Auto-pause on critical errors** creates a safe dead-stop until an admin resolves the issue.

---
