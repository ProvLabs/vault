**Context**

Fees must be collected automatically during the vault's interest reconciliation lifecycle. This task involves updating the `reconcileVaultInterest` process to deduct the calculated AUM fee from the vault's reserves and route it to the ProvLabs wallet before interest is paid.


**Dependencies**
- **Blocked by:** None.
- **Depends on:** `issues/01_fee_calculation_and_constants.md`.


**What needs to be done**

- **Implement Transfer Logic:** Create `PerformVaultFeeTransfer` in `keeper/reconcile.go` using the `BankKeeper.SendCoins` method.
- **Update Reconciliation Flow:** Integrate the fee transfer into `reconcileVaultInterest`.
- **Emit Events:** Implement `EventVaultFeeCollected` with `vault_address`, `aum_snapshot`, `fee_amount`, and `period_seconds` for audit purposes.


**Technical details**

- **Source Account:** Fees are deducted from the vault's own address (reserves), NOT the principal marker account.
- **Asset Type:** Fees are always collected in the vault's `UnderlyingAsset`.
- **Order of Operations:** Fees should be collected *before* interest payments to prioritize the technology fee.


**Acceptance Criteria**
- [ ] `reconcileVaultInterest` successfully triggers fee collection.
- [ ] Fees are correctly routed to the ProvLabs wallet address.
- [ ] `EventVaultFeeCollected` is emitted for every successful collection.
- [ ] Reconciliation fails if reserves are insufficient to cover the fee.
