**Context**

The integrity of the revenue model must be verified through rigorous integration and end-to-end testing. This task ensures that fees are calculated, collected, and routed correctly across multiple blocks and various vault configurations.


**Dependencies**
- **Blocked by:** None.
- **Depends on:** `issues/02_fee_reconciliation_integration.md` and `issues/03_depletion_and_runway_logic.md`.


**What needs to be done**

- **Integration Tests:** Add multi-block fee accrual tests in `keeper/reconcile_test.go`.
- **Insufficient Funds Tests:** Add test cases for vaults with exactly zero reserves and vaults with partial reserves.
- **E2E Verification:** Verify that the ProvLabs wallet balance increases accurately over multiple reconciliation cycles using `simapp`.


**Technical details**

- **Simulation:** Use `simapp` to simulate block progression and verify the bank balances of the recipient account.
- **Audit Logs:** Verify that event attributes match the mathematical expectations for each period.


**Acceptance Criteria**
- [ ] Integration tests cover 1-minute, 1-hour, and 1-day fee accrual scenarios.
- [ ] Tests confirm that fees are not lost or duplicated during partial reconciliations.
- [ ] E2E verification demonstrates correct routing to the ProvLabs wallet in a simulated environment.
- [ ] Deployment documentation is updated with the required recipient address.
