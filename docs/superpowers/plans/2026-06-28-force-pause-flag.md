# Force-Pause Flag Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `PauseVault` strict by default (a failed pre-pause reconcile or valuation aborts the tx and leaves the vault unpaused) and gate the existing best-effort behavior behind an explicit `force` flag, with the tolerated error recorded on `EventVaultPaused`.

**Architecture:** Add `bool force` to `MsgPauseVaultRequest`. The handler returns the wrapped error on any reconcile/valuation failure when `force=false`; when `force=true` it logs, tolerates the failure, snapshots a best-effort `PausedBalance` (zero on valuation failure), and persists via the validation-skipping `AuthKeeper.SetAccount` path that `autoPauseVault` already uses. `EventVaultPaused` gains `forced` (force path used) and `forced_error` (the tolerated reconcile/valuation error, joined when both fail).

**Tech Stack:** Go, Cosmos SDK, gogoproto (`make proto-all`), testify suite tests.

## Global Constraints

- Error wrapping: always `fmt.Errorf("failed to [action]: %w", err)`.
- Logging in `keeper/`: use `k.getLogger(ctx)`, never `ctx.Logger()`. Preserve existing key-value fields.
- Number literals: underscore separators (e.g. `100_000`).
- Tests: table-driven, descriptive case names, high-context `Require`/`Assert` messages, no godocs on `TestXxx`.
- Do NOT run `make format` (its gci config conflicts with lint). Use `make lint` + `gofmt` instead.
- Proto is the source of truth; never hand-edit generated `*.pb.go` / `api/`. Run `make proto-all` after proto edits.
- Event error strings land in consensus events, so they must be deterministic. `reconcileVault` and `GetNetTVVInUnderlyingAsset` errors are plain `fmt.Errorf` wraps (no map ordering / randomness) and are safe.

---

### Task 1: Add proto fields and regenerate

**Files:**
- Modify: `proto/provlabs/vault/v1/tx.proto:474` (MsgPauseVaultRequest)
- Modify: `proto/provlabs/vault/v1/events.proto:268` (EventVaultPaused)
- Regenerated (do not hand-edit): `types/tx.pb.go`, `types/events.pb.go`, `api/...`

**Interfaces:**
- Produces: `MsgPauseVaultRequest.Force bool`; `EventVaultPaused.Forced bool`, `EventVaultPaused.ForcedError string`.

- [ ] **Step 1: Add the `force` field to `MsgPauseVaultRequest`**

In `proto/provlabs/vault/v1/tx.proto`, inside `message MsgPauseVaultRequest`, after the `reason` field:

```proto
  // force, when true, allows the pause to proceed even if the pre-pause
  // reconcile or vault valuation fails. The pause is applied best-effort:
  // tolerated failures are logged and surfaced on EventVaultPaused, and
  // PausedBalance may be the net TVV, zero (when valuation itself failed), or
  // otherwise approximate. When false (default), any reconcile or valuation
  // failure aborts the pause and the vault remains unpaused.
  bool force = 4;
```

- [ ] **Step 2: Add `forced` and `forced_error` to `EventVaultPaused`**

In `proto/provlabs/vault/v1/events.proto`, inside `message EventVaultPaused`, after `total_vault_value`:

```proto
  // forced indicates the pause used the force path, waiving the strict
  // reconcile/valuation gate. False for a normal strict pause; true for a
  // forced manual pause and for automated auto-pauses.
  bool forced = 5;
  // forced_error records the reconcile and/or valuation error that was
  // tolerated when forced is true. Empty when nothing failed. When non-empty,
  // total_vault_value may be stale or zero.
  string forced_error = 6;
```

- [ ] **Step 3: Regenerate Go code**

Run: `make proto-all`
Expected: completes without error; `git status` shows changes in `types/tx.pb.go`, `types/events.pb.go`, `api/`.

- [ ] **Step 4: Verify the build still compiles**

Run: `go build ./...`
Expected: success. New fields are additive and not yet referenced, so the build stays green.

- [ ] **Step 5: Commit**

```bash
git add proto/ types/tx.pb.go types/events.pb.go api/
git commit -m "feat: add force flag to MsgPauseVaultRequest and forced/forced_error to EventVaultPaused"
```

---

### Task 2: Extend the event constructor and update existing call sites

**Files:**
- Modify: `types/events.go:202` (`NewEventVaultPaused`)
- Modify: `keeper/vault.go:526-543` (`autoPauseVault`)
- Modify: `keeper/msg_server.go:586` (`PauseVault` emit, interim)

**Interfaces:**
- Consumes: `EventVaultPaused.Forced`, `EventVaultPaused.ForcedError` (Task 1).
- Produces: `NewEventVaultPaused(vaultAddress, authority, reason string, totalVaultValue sdk.Coin, forced bool, forcedError string) *EventVaultPaused`.

- [ ] **Step 1: Update the constructor signature and body**

In `types/events.go`, replace `NewEventVaultPaused`:

```go
// NewEventVaultPaused creates a new EventVaultPaused event. forced reports
// whether the pause waived the strict reconcile/valuation gate, and forcedError
// carries the tolerated error (empty when nothing failed).
func NewEventVaultPaused(vaultAddress, authority, reason string, totalVaultValue sdk.Coin, forced bool, forcedError string) *EventVaultPaused {
	return &EventVaultPaused{
		VaultAddress:    vaultAddress,
		Authority:       authority,
		Reason:          reason,
		TotalVaultValue: totalVaultValue.String(),
		Forced:          forced,
		ForcedError:     forcedError,
	}
}
```

- [ ] **Step 2: Update `autoPauseVault` to record the tolerated valuation error**

In `keeper/vault.go`, in `autoPauseVault`, capture the TVV error and pass `forced=true`:

```go
	var forcedError string
	tvv, err := k.GetNetTVVInUnderlyingAsset(ctx, *vault)
	if err != nil {
		forcedError = fmt.Sprintf("valuation failed: %v", err)
		k.getLogger(ctx).Error("Failed to get net TVV in underlying asset", "vault_address", vault.GetAddress().String(), "error", err)
	}

	vault.Paused = true
	vault.PausedReason = reason
	vault.PausedBalance = sdk.Coin{Denom: vault.UnderlyingAsset, Amount: tvv}
	k.AuthKeeper.SetAccount(ctx, vault) // Updating via SetAccount to skip validation since auto-pausing is triggered by invalid state

	k.emitEvent(ctx, types.NewEventVaultPaused(vault.GetAddress().String(), vault.GetAddress().String(), reason, vault.PausedBalance, true, forcedError))
```

Confirm `fmt` is imported in `keeper/vault.go` (it is used elsewhere in the file); if not, add it.

- [ ] **Step 3: Update the `PauseVault` emit call (interim, replaced in Task 3)**

In `keeper/msg_server.go:586`, change the emit so the file compiles with the new signature. Pass `false, ""` for now; Task 3 wires the real values:

```go
	k.emitEvent(ctx, types.NewEventVaultPaused(msg.VaultAddress, msg.Authority, msg.Reason, vault.PausedBalance, false, ""))
```

- [ ] **Step 4: Build and run the affected package tests**

Run: `go build ./... && go test ./types/... ./keeper/... -run 'Pause|AutoPause' -count=1`
Expected: PASS. Existing tests assert the event *type* string only, so the new fields do not break them.

- [ ] **Step 5: Commit**

```bash
git add types/events.go keeper/vault.go keeper/msg_server.go
git commit -m "feat: thread forced/forced_error through EventVaultPaused constructor and auto-pause"
```

---

### Task 3: Gate best-effort pause behind `force` in the handler

This task carries its own TDD cycle. The existing branch test
`TestMsgServer_PauseVault_ReconcileFailureStillPauses`
(`keeper/msg_server_test.go:4416`) currently pauses through a failed reconcile
*without* `force`; it must be updated to set `Force: true`, and strict-mode
failure cases must be added.

**Files:**
- Modify: `keeper/msg_server.go:540-589` (`PauseVault`)
- Modify: `keeper/msg_server_test.go:4416` (`TestMsgServer_PauseVault_ReconcileFailureStillPauses`)

**Interfaces:**
- Consumes: `MsgPauseVaultRequest.Force` (Task 1); `NewEventVaultPaused(..., forced bool, forcedError string)` (Task 2); `types.ZeroInterestRate` (`types/vault.go:21`); `types.NewEventVaultInterestChange(addr, currentRate, desiredRate string)` (`types/events.go:80`).

- [ ] **Step 1: Rewrite the failing tests first**

Replace `TestMsgServer_PauseVault_ReconcileFailureStillPauses` in `keeper/msg_server_test.go` with a version that covers strict failure AND forced success. The two `setup` closures (insufficient reserves; broken TVV) are unchanged from the current branch version; only the request `Force` value, the expected error, and the new assertions differ.

```go
func (s *TestSuite) TestMsgServer_PauseVault_ForceVsStrict() {
	reason := "emergency"

	insufficientReservesSetup := func() (sdk.AccAddress, string) {
		underlying := "under"
		share := "vaultshares"
		s.requireAddFinalizeAndActivateMarker(sdk.NewCoin(underlying, math.NewInt(10_000)), s.adminAddr)
		_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
			Admin:           s.adminAddr.String(),
			ShareDenom:      share,
			UnderlyingAsset: underlying,
		})
		s.Require().NoError(err, "vault creation should succeed for share denom %s", share)
		vaultAddr := types.GetVaultAddress(share)
		vault, err := s.k.GetVault(s.ctx, vaultAddr)
		s.Require().NoError(err, "loading the vault for setup should succeed")

		oneDay := int64(24 * time.Hour / time.Second)
		vault.CurrentInterestRate = "1.0"
		vault.DesiredInterestRate = "1.0"
		vault.PeriodStart = s.ctx.BlockTime().Unix() - oneDay
		s.k.AuthKeeper.SetAccount(s.ctx, vault)

		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vault.PrincipalMarkerAddress(),
			sdk.NewCoins(sdk.NewInt64Coin(underlying, 100_000))),
			"funding the principal marker should succeed so reconcile owes unpayable interest")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		return vaultAddr, underlying
	}

	brokenTVVSetup := func() (sdk.AccAddress, string) {
		underlying := "ylds"
		share := "vshare"
		payment := "usdc"
		s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(payment, 1_000_000), s.adminAddr)
		vault := s.setupBaseVault(underlying, share, payment)
		s.Require().NoError(s.k.NAVs.Remove(s.ctx, collections.Join(vault.GetAddress(), payment)),
			"removing the bootstrap payment NAV should surface the conversion failure")
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vault.PrincipalMarkerAddress(),
			sdk.NewCoins(sdk.NewInt64Coin(payment, 10))),
			"funding the principal with an unpriced accepted denom should make TVV conversion error")
		s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
		return vault.GetAddress(), underlying
	}

	tests := []struct {
		name                string
		setup               func() (sdk.AccAddress, string)
		force               bool
		expectErr           bool
		expectedPauseAmount int64
		expectForcedError   bool
	}{
		{
			name:      "strict pause aborts when reconcile fails on insufficient reserves",
			setup:     insufficientReservesSetup,
			force:     false,
			expectErr: true,
		},
		{
			name:      "strict pause aborts when TVV conversion fails",
			setup:     brokenTVVSetup,
			force:     false,
			expectErr: true,
		},
		{
			name:                "force pause proceeds through insufficient-reserves reconcile failure",
			setup:               insufficientReservesSetup,
			force:               true,
			expectErr:           false,
			expectedPauseAmount: 100_000,
			expectForcedError:   true,
		},
		{
			name:                "force pause proceeds through broken TVV with zero snapshot",
			setup:               brokenTVVSetup,
			force:               true,
			expectErr:           false,
			expectedPauseAmount: 0,
			expectForcedError:   true,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			vaultAddr, underlying := tc.setup()

			_, err := keeper.NewMsgServer(s.simApp.VaultKeeper).PauseVault(s.ctx, &types.MsgPauseVaultRequest{
				Authority:    s.adminAddr.String(),
				VaultAddress: vaultAddr.String(),
				Reason:       reason,
				Force:        tc.force,
			})

			v, getErr := s.k.GetVault(s.ctx, vaultAddr)
			s.Require().NoError(getErr, "loading the vault should succeed for case %q", tc.name)

			if tc.expectErr {
				s.Require().Error(err, "strict pause must fail when pre-pause bookkeeping fails for case %q", tc.name)
				s.Assert().False(v.Paused, "vault must remain unpaused after a failed strict pause for case %q", tc.name)
				return
			}

			s.Require().NoError(err, "force pause must succeed even when the pre-pause reconcile fails for case %q", tc.name)
			s.Assert().True(v.Paused, "vault should be paused after a force pause for case %q", tc.name)
			s.Assert().Equal(reason, v.PausedReason, "paused reason should be recorded for case %q", tc.name)
			s.Assert().Equal(types.ZeroInterestRate, v.CurrentInterestRate, "pausing should zero the current interest rate for case %q", tc.name)
			s.Assert().Equal(underlying, v.PausedBalance.Denom, "paused balance should be denominated in the underlying asset for case %q", tc.name)
			s.Assert().Equal(tc.expectedPauseAmount, v.PausedBalance.Amount.Int64(), "paused balance should be the best-effort TVV snapshot for case %q", tc.name)

			pausedEvent := s.findLastEventVaultPaused()
			s.Require().NotNil(pausedEvent, "an EventVaultPaused should have been emitted for case %q", tc.name)
			s.Assert().True(pausedEvent.Forced, "the forced flag should be set on a force pause for case %q", tc.name)
			if tc.expectForcedError {
				s.Assert().NotEmpty(pausedEvent.ForcedError, "forced_error should record the tolerated failure for case %q", tc.name)
			}
		})
	}
}
```

- [ ] **Step 2: Add the `findLastEventVaultPaused` helper to `suite_test.go`**

Per the DRY mandate, add a typed-event lookup helper in `keeper/suite_test.go`:

```go
// findLastEventVaultPaused returns the most recent EventVaultPaused decoded from
// the context event manager, or nil when none has been emitted.
func (s *TestSuite) findLastEventVaultPaused() *types.EventVaultPaused {
	events := s.ctx.EventManager().Events()
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type != "provlabs.vault.v1.EventVaultPaused" {
			continue
		}
		msg, err := sdk.ParseTypedEvent(abci.Event(events[i]))
		s.Require().NoError(err, "parsing the EventVaultPaused typed event should succeed")
		paused, ok := msg.(*types.EventVaultPaused)
		s.Require().True(ok, "parsed event should be *EventVaultPaused")
		return paused
	}
	return nil
}
```

Ensure `keeper/suite_test.go` imports `abci "github.com/cometbft/cometbft/abci/types"` (add it if missing).

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./keeper/... -run 'TestMsgServer_PauseVault_ForceVsStrict|TestMsgServer_PauseVault_ReconcileFailureStillPauses' -count=1`
Expected: FAIL. Strict cases currently pass best-effort (no error), and the old test name no longer exists. This confirms the new behavior is not yet implemented.

- [ ] **Step 4: Implement the strict/force branch in `PauseVault`**

Replace the body of `PauseVault` in `keeper/msg_server.go` (from the `if vault.Paused` check through the final emit) with:

```go
	if vault.Paused {
		return nil, fmt.Errorf("vault %s is already paused", msg.VaultAddress)
	}

	var forcedErrors []string

	if err = k.reconcileVault(ctx, vault); err != nil {
		if !msg.Force {
			return nil, fmt.Errorf("failed to reconcile before pausing: %w", err)
		}
		forcedErrors = append(forcedErrors, fmt.Sprintf("reconcile failed: %v", err))
		k.getLogger(ctx).Error(
			"reconcile failed before forced pause; pausing best-effort",
			"vault", msg.VaultAddress,
			"err", err,
		)
	}

	tvv, err := k.GetNetTVVInUnderlyingAsset(ctx, *vault)
	if err != nil {
		if !msg.Force {
			return nil, fmt.Errorf("failed to get net TVV before pausing: %w", err)
		}
		forcedErrors = append(forcedErrors, fmt.Sprintf("valuation failed: %v", err))
		k.getLogger(ctx).Error(
			"failed to value vault before forced pause; snapshotting zero paused balance",
			"vault", msg.VaultAddress,
			"err", err,
		)
		tvv = sdkmath.ZeroInt()
	}

	vault.PausedBalance = sdk.NewCoin(vault.UnderlyingAsset, tvv)
	vault.Paused = true
	vault.PausedReason = msg.Reason
	vault.CurrentInterestRate = types.ZeroInterestRate
	k.emitEvent(ctx, types.NewEventVaultInterestChange(vault.GetAddress().String(), types.ZeroInterestRate, vault.DesiredInterestRate))

	if msg.Force {
		k.AuthKeeper.SetAccount(ctx, vault)
	} else if err := k.SetVaultAccount(ctx, vault); err != nil {
		return nil, fmt.Errorf("failed to set vault account: %w", err)
	}

	forcedError := strings.Join(forcedErrors, "; ")
	k.emitEvent(ctx, types.NewEventVaultPaused(msg.VaultAddress, msg.Authority, msg.Reason, vault.PausedBalance, msg.Force, forcedError))

	return &types.MsgPauseVaultResponse{}, nil
```

Notes for the implementer:
- This removes the `UpdateInterestRates` call. That helper persisted via the validating `SetVaultAccount` internally, which (a) double-persisted with the existing trailing `SetVaultAccount` and (b) made force-mode validation-skipping impossible. Zeroing `CurrentInterestRate` inline plus emitting `NewEventVaultInterestChange` preserves the prior observable behavior (rate set to `0.0`, interest-change event emitted) while persisting exactly once under mode control. `DesiredInterestRate` is intentionally left unchanged, matching the prior `UpdateInterestRates(ctx, vault, ZeroInterestRate, vault.DesiredInterestRate)` call.
- Add `"strings"` to the import block in `keeper/msg_server.go`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./keeper/... -run 'TestMsgServer_PauseVault' -count=1`
Expected: PASS, including the existing `TestMsgServer_PauseVault` and `TestMsgServer_PauseVault_Failures`.

- [ ] **Step 6: Update the godoc on `PauseVault`**

Replace the doc comment above `PauseVault` so it describes the gated behavior rather than unconditional best-effort:

```go
// PauseVault pauses a vault, disabling all user-facing operations.
//
// By default the pause is strict: it reconciles outstanding interest and fees
// and snapshots the net TVV first, and any failure (e.g. insufficient reserves
// to settle positive interest, or a broken TVV/NAV conversion) aborts the pause
// and leaves the vault unpaused. Setting Force on the request makes the pause an
// emergency control that mirrors autoPauseVault: tolerated reconcile/valuation
// failures are logged and surfaced via EventVaultPaused.forced_error, the frozen
// PausedBalance is the net TVV when valuable or zero when valuation itself failed,
// and the account is persisted without validation so an invalid-state vault can
// still be frozen.
```

- [ ] **Step 7: Commit**

```bash
git add keeper/msg_server.go keeper/msg_server_test.go keeper/suite_test.go
git commit -m "feat: make PauseVault strict by default, gate best-effort pause behind force flag"
```

---

### Task 4: Update the spec

**Files:**
- Modify: `spec/03_messages.md` (PauseVault row at line 70; PauseVault section at line 276)
- Modify: `spec/04_events.md` (EventVaultPaused section ~line 67)
- Modify: `spec/06_blocker.md` (auto-pause notes ~line 176)

- [ ] **Step 1: Update the message reference**

In `spec/03_messages.md`, replace the PauseVault summary-table row (line 70) Notes cell:

```
| `PauseVault`             | Admin or Asset Manager            |                   ✅ |                 ❌ | Strict by default: reconciles, snapshots `PausedBalance`, sets paused; aborts if reconcile/valuation fails. `force=true` pauses best-effort, tolerating failures and recording them on `EventVaultPaused`. |
```

Then replace the `## PauseVault` section body (lines 276-281):

```markdown
## PauseVault

Admin or Asset Manager. Pauses a vault, disabling swap-ins and swap-outs, and recording reason + balance snapshot.

By default the pause is **strict**: it reconciles outstanding interest and fees and values the vault first, and any failure (insufficient reserves to settle positive interest, or a broken TVV/NAV conversion) aborts the request and leaves the vault unpaused. The failed transaction is the operator's signal that the vault is in an unexpected state.

Setting `force = true` makes the pause an **emergency control**: a reconcile or valuation failure is logged and tolerated rather than blocking the freeze. The frozen `PausedBalance` is the net TVV when the vault can be valued, or zero when the valuation itself is what failed, so it may be approximate. The tolerated error is recorded on `EventVaultPaused.forced_error`.

* **Request:** `MsgPauseVaultRequest { authority, vault_address, reason, force }`
* **Response:** `MsgPauseVaultResponse {}`
```

- [ ] **Step 2: Document the new event fields**

In `spec/04_events.md`, in the `### EventVaultPaused` field list, add:

```markdown
* `forced` — true when the pause used the force path and waived the strict reconcile/valuation gate (also true for automated auto-pauses)
* `forced_error` — the reconcile and/or valuation error tolerated during a forced pause; empty when nothing failed. When non-empty, `total_vault_value` may be stale or zero.
```

- [ ] **Step 3: Note the auto-pause event semantics**

In `spec/06_blocker.md`, near the auto-pause description (~line 176), add a sentence:

```markdown
Auto-pause emits `EventVaultPaused` with `forced = true` and, when valuation failed, the tolerated error in `forced_error`; the manual `PauseVault` path produces the same signal only when called with `force = true`.
```

- [ ] **Step 4: Commit**

```bash
git add spec/
git commit -m "docs: document strict/force PauseVault semantics and EventVaultPaused forced fields"
```

---

### Task 5: Full local verification

**Files:** none (verification only).

- [ ] **Step 1: Lint**

Run: `make lint`
Expected: no findings. If `gofmt` differences appear, run `gofmt -w` on the changed files (do NOT run `make format`).

- [ ] **Step 2: Full unit test suite**

Run: `go test ./... -count=1`
Expected: PASS.

- [ ] **Step 3: Simplest sims**

Run: `make test-sim-simple && make test-sim-nondeterminism`
Expected: PASS. (Equivalent to invoking the `vault-lint-test` skill, which wraps format/lint/test/sims.)

- [ ] **Step 4: Final review of the diff**

Run: `git diff main...HEAD --stat`
Expected: changes limited to `proto/`, regenerated `types/*.pb.go` + `api/`, `types/events.go`, `keeper/vault.go`, `keeper/msg_server.go`, `keeper/msg_server_test.go`, `keeper/suite_test.go`, and `spec/`.

---

## Self-Review Notes

- **Spec coverage:** strict-default behavior (Task 3), force flag (Tasks 1, 3), event audit fields (Tasks 1-3), determinism of error strings (Global Constraints), spec sync (Task 4) — all covered.
- **Type consistency:** `NewEventVaultPaused(..., forced bool, forcedError string)` defined in Task 2 and called identically in Tasks 2-3; `MsgPauseVaultRequest.Force` defined in Task 1 and consumed in Task 3; `findLastEventVaultPaused` defined and used in Task 3.
- **Carried-over decision:** `forced_error` joins reconcile-then-valuation failures with `"; "` rather than keeping only the first error (per the design discussion). If only the primary error is preferred, drop the `forcedErrors` slice and assign the single error string at each failure site instead.
