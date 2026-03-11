**Context**

To prevent vaults from defaulting on fee payments, the system must accurately project when a vault's reserves will be depleted by both interest and AUM fees. This task involves updating the "runway" and "payout ability" checks to include projected fees.


**Dependencies**
- **Blocked by:** None.
- **Depends on:** `issues/01_fee_calculation_and_constants.md`.


**What needs to be done**

- **Update Payout Checks:** Modify `CanPayoutDuration` in `keeper/reconcile.go` to subtract the projected AUM fee from available reserves.
- **Update Expiration Logic:** Update `CalculateExpiration` in `interest/interest.go` to include the AUM fee as a recurring deduction.
- **Ensure Stability:** Verify that a zero interest rate still allows for fee-based depletion checks.


**Technical details**

- **Depletion Condition:** `Reserves < (Projected Interest + Projected AUM Fee)`.
- **Forecast Window:** The `AutoReconcilePayoutDuration` (24 hours) must now account for both costs.


**Acceptance Criteria**
- [ ] `CanPayoutDuration` correctly identifies vaults that cannot cover both fees and interest.
- [ ] `CalculateExpiration` returns a sooner expiration date when AUM fees are active.
- [ ] Vaults are correctly marked "depleted" if they have enough for interest but not for the AUM fee.
