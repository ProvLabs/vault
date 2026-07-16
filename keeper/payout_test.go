package keeper_test

import (
	"errors"
	"fmt"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	markertypes "github.com/provenance-io/provenance/x/marker/types"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
)

func (s *TestSuite) TestKeeper_ProcessPendingSwapOuts() {
	underlyingDenom := "ylds"
	assets := sdk.NewInt64Coin(underlyingDenom, 50)
	testBlockTime := time.Now().UTC()
	duePayoutTime := testBlockTime.Add(-1 * time.Hour).Unix()

	tests := []struct {
		name          string
		setup         func(shareDenom string, vaultAddr sdk.AccAddress, shares sdk.Coin) (ownerAddr sdk.AccAddress, reqID uint64)
		posthandler   func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, principalAddress sdk.AccAddress, shares sdk.Coin, testBlockTime time.Time)
		expectedError string
		batchSize     int
	}{
		{
			name: "successful payout of due request",
			setup: func(shareDenom string, vaultAddr sdk.AccAddress, shares sdk.Coin) (sdk.AccAddress, uint64) {
				ownerAddr := s.CreateAndFundAccount(assets)
				vault := s.setupBaseVault(underlyingDenom, shareDenom)

				minted, err := s.k.SwapIn(s.ctx, vaultAddr, ownerAddr, assets)
				s.Require().NoError(err, "should successfully swap in assets")
				s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, ownerAddr, vault.GetAddress(), sdk.NewCoins(*minted)), "should escrow shares into vault account")

				req := types.PendingSwapOut{
					Owner:        ownerAddr.String(),
					VaultAddress: vaultAddr.String(),
					RedeemDenom:  underlyingDenom,
					Shares:       *minted,
				}
				id, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, duePayoutTime, &req)
				s.Require().NoError(err, "should successfully enqueue request")
				return ownerAddr, id
			},
			posthandler: func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, principalAddress sdk.AccAddress, shares sdk.Coin, testBlockTime time.Time) {
				s.assertBalance(ownerAddr, underlyingDenom, assets.Amount)
				supply := s.k.BankKeeper.GetSupply(s.ctx, shareDenom)
				s.Require().True(supply.Amount.IsZero(), "total supply of shares should be zero after burn")

				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "should successfully get vault")
				s.Require().NotNil(vault, "vault should not be nil")
				s.Require().Equal(supply, vault.TotalShares, "vault TotalShares should match supply")

				expectedEvents := sdk.Events{}
				expectedEvents = append(expectedEvents, createSendCoinEvents(principalAddress.String(), ownerAddr.String(), sdk.NewCoins(assets).String())...)
				expectedEvents = append(expectedEvents, createSendCoinEvents(vaultAddr.String(), principalAddress.String(), shares.String())...)
				expectedEvents = append(expectedEvents, createMarkerBurn(vaultAddr, principalAddress, shares)...)
				typedEvent, err := sdk.TypedEventToEvent(types.NewEventSwapOutCompleted(vaultAddr.String(), ownerAddr.String(), assets, reqID))
				s.Require().NoError(err, "should not error converting typed EventSwapOutCompleted")
				expectedEvents = append(expectedEvents, typedEvent)
				s.Assert().Equal(
					normalizeEvents(expectedEvents),
					normalizeEvents(s.ctx.EventManager().Events()),
					"a single EventSwapOutCompleted should be emitted",
				)
			},
			batchSize: keeper.MaxSwapOutBatchSize,
		},
		{
			name: "successful payout of due request with 0 assets",
			setup: func(shareDenom string, vaultAddr sdk.AccAddress, shares sdk.Coin) (sdk.AccAddress, uint64) {
				ownerAddr := s.CreateAndFundAccount(assets)
				vault := s.setupBaseVault(underlyingDenom, shareDenom)

				minted, err := s.k.SwapIn(s.ctx, vaultAddr, ownerAddr, assets)
				s.Require().NoError(err, "should successfully swap in assets")
				s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, ownerAddr, vault.GetAddress(), sdk.NewCoins(*minted)), "should escrow shares into vault account")

				req := types.PendingSwapOut{
					Owner:        ownerAddr.String(),
					VaultAddress: vaultAddr.String(),
					RedeemDenom:  underlyingDenom,
					Shares:       sdk.NewInt64Coin(shareDenom, 1),
				}
				id, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, duePayoutTime, &req)
				s.Require().NoError(err, "should successfully enqueue request")
				return ownerAddr, id
			},
			posthandler: func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, principalAddress sdk.AccAddress, shares sdk.Coin, testBlockTime time.Time) {
				expectedAssets := sdk.NewInt64Coin(underlyingDenom, 0)
				sharesBurned := sdk.NewInt64Coin(shareDenom, 1)
				s.assertBalance(ownerAddr, underlyingDenom, math.ZeroInt())
				supply := s.k.BankKeeper.GetSupply(s.ctx, shareDenom)
				s.Require().Equal(supply.Amount, shares.Sub(sharesBurned).Amount, "the single submitted share should be burned")

				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "should successfully get vault")
				s.Require().NotNil(vault, "vault should not be nil")
				s.Require().Equal(supply, vault.TotalShares, "vault TotalShares should match supply")

				expectedEvents := sdk.Events{}
				expectedEvents = append(expectedEvents, createSendCoinEvents(principalAddress.String(), ownerAddr.String(), sdk.NewCoins(expectedAssets).String())...)
				expectedEvents = append(expectedEvents, createSendCoinEvents(vaultAddr.String(), principalAddress.String(), sharesBurned.String())...)
				expectedEvents = append(expectedEvents, createMarkerBurn(vaultAddr, principalAddress, sharesBurned)...)
				typedEvent, err := sdk.TypedEventToEvent(types.NewEventSwapOutCompleted(vaultAddr.String(), ownerAddr.String(), expectedAssets, reqID))
				s.Require().NoError(err, "should not error converting typed EventSwapOutCompleted")
				expectedEvents = append(expectedEvents, typedEvent)
				s.Assert().Equal(
					normalizeEvents(expectedEvents),
					normalizeEvents(s.ctx.EventManager().Events()),
					"a single EventSwapOutCompleted should be emitted",
				)
			},
			batchSize: keeper.MaxSwapOutBatchSize,
		},
		{
			name: "successful payout of due request with reconcile",
			setup: func(shareDenom string, vaultAddr sdk.AccAddress, shares sdk.Coin) (sdk.AccAddress, uint64) {
				ownerAddr := s.CreateAndFundAccount(assets)
				vault := s.setupBaseVault(underlyingDenom, shareDenom)

				minted, err := s.k.SwapIn(s.ctx, vaultAddr, ownerAddr, assets)
				s.Require().NoError(err, "should successfully swap in assets")
				s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, ownerAddr, vault.GetAddress(), sdk.NewCoins(*minted)), "should escrow shares into vault account")

				vault, err = s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "should successfully get vault")
				vault.PeriodStart = 1
				vault.FeePeriodStart = 1
				s.Require().NoError(s.k.SetVaultAccount(s.ctx, vault), "must update vault account period")
				s.Require().NotNil(vault, "vault should not be nil")

				req := types.PendingSwapOut{
					Owner:        ownerAddr.String(),
					VaultAddress: vaultAddr.String(),
					RedeemDenom:  underlyingDenom,
					Shares:       *minted,
				}
				id, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, duePayoutTime, &req)
				s.Require().NoError(err, "should successfully enqueue request")
				return ownerAddr, id
			},
			posthandler: func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, principalAddress sdk.AccAddress, shares sdk.Coin, testBlockTime time.Time) {
				// Fee calculation: 50 * 0.0015 * testBlockTime.Unix() / 31,536,000
				// For current epoch (~1.7B), fee is 4.
				expectedAssets := sdk.NewInt64Coin(underlyingDenom, 46)
				s.assertBalance(ownerAddr, underlyingDenom, expectedAssets.Amount)
				supply := s.k.BankKeeper.GetSupply(s.ctx, shareDenom)
				s.Require().True(supply.Amount.IsZero(), "total supply of shares should be zero after burn")

				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "should successfully get vault")
				s.Require().NotNil(vault, "vault should not be nil")
				s.Require().Equal(supply, vault.TotalShares, "vault TotalShares should match supply")

				expectedEvents := sdk.Events{}
				reconcileEvent, err := sdk.TypedEventToEvent(types.NewEventVaultReconcile(vaultAddr.String(), assets, assets, vault.CurrentInterestRate, testBlockTime.Unix()-1, math.NewInt(0)))
				s.Require().NoError(err, "should not error converting typed EventVaultReconciled")
				expectedEvents = append(expectedEvents, reconcileEvent)

				// AUM Fee events
				provLabsAddr, err := s.k.GetAUMFeeAddress(s.ctx)
				s.Require().NoError(err, "failed to get AUM fee address")
				expectedEvents = append(expectedEvents, createSendCoinEvents(principalAddress.String(), provLabsAddr.String(), "4ylds")...)
				feeEvent, err := sdk.TypedEventToEvent(&types.EventVaultFeeCollected{
					VaultAddress:      vaultAddr.String(),
					CollectedAmount:   "4ylds",
					RequestedAmount:   "4ylds",
					AumSnapshot:       "50ylds",
					DurationSeconds:   testBlockTime.Unix() - 1,
					OutstandingAmount: "0ylds",
				})
				s.Require().NoError(err, "TypedEventToEvent should not error for EventVaultFeeCollected")
				expectedEvents = append(expectedEvents, feeEvent)

				expectedEvents = append(expectedEvents, createMarkerSetNAV(shareDenom, expectedAssets, "vault", shares.Amount.Uint64()))
				expectedEvents = append(expectedEvents, createSendCoinEvents(principalAddress.String(), ownerAddr.String(), sdk.NewCoins(expectedAssets).String())...)
				expectedEvents = append(expectedEvents, createSendCoinEvents(vaultAddr.String(), principalAddress.String(), shares.String())...)
				expectedEvents = append(expectedEvents, createMarkerBurn(vaultAddr, principalAddress, shares)...)
				typedEvent, err := sdk.TypedEventToEvent(types.NewEventSwapOutCompleted(vaultAddr.String(), ownerAddr.String(), expectedAssets, reqID))
				s.Require().NoError(err, "should not error converting typed EventSwapOutCompleted")
				expectedEvents = append(expectedEvents, typedEvent)
				s.Assert().Equal(
					normalizeEvents(expectedEvents),
					normalizeEvents(s.ctx.EventManager().Events()),
					"a single EventSwapOutCompleted should be emitted",
				)
			},
			batchSize: keeper.MaxSwapOutBatchSize,
		},
		{
			name: "successful limit by batch size of 0",
			setup: func(shareDenom string, vaultAddr sdk.AccAddress, shares sdk.Coin) (sdk.AccAddress, uint64) {
				ownerAddr := s.CreateAndFundAccount(assets)
				vault := s.setupBaseVault(underlyingDenom, shareDenom)

				minted, err := s.k.SwapIn(s.ctx, vaultAddr, ownerAddr, assets)
				s.Require().NoError(err, "should successfully swap in assets")
				s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, ownerAddr, vault.GetAddress(), sdk.NewCoins(*minted)), "should escrow shares into vault account")

				req := types.PendingSwapOut{
					Owner:        ownerAddr.String(),
					VaultAddress: vaultAddr.String(),
					RedeemDenom:  underlyingDenom,
					Shares:       *minted,
				}
				id, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, duePayoutTime, &req)
				s.Require().NoError(err, "should successfully enqueue request")
				return ownerAddr, id
			},
			posthandler: func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, principalAddress sdk.AccAddress, shares sdk.Coin, testBlockTime time.Time) {
				supply := s.k.BankKeeper.GetSupply(s.ctx, shareDenom)
				s.Require().False(supply.Amount.IsZero(), "shares should still exist since nothing was processed")
				s.assertBalance(ownerAddr, underlyingDenom, math.ZeroInt())
				s.assertBalance(vaultAddr, shareDenom, shares.Amount)

				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "should successfully get vault")
				s.Require().NotNil(vault, "vault should not be nil")
				s.Require().Equal(supply, vault.TotalShares, "vault TotalShares should match supply")

				expectedEvents := sdk.Events{}
				s.Assert().Equal(
					normalizeEvents(expectedEvents),
					normalizeEvents(s.ctx.EventManager().Events()),
					"a single EventSwapOutCompleted should be emitted",
					"no events should be emitted when batch size is 0",
				)
			},
			batchSize: 0,
		},
		{
			name: "failed payout refunds shares",
			setup: func(shareDenom string, vaultAddr sdk.AccAddress, shares sdk.Coin) (sdk.AccAddress, uint64) {
				ownerAddr := s.CreateAndFundAccount(assets)
				vault := s.setupBaseVaultRestricted(underlyingDenom, shareDenom)

				minted, err := s.k.SwapIn(s.ctx, vaultAddr, ownerAddr, assets)
				s.Require().NoError(err, "should successfully swap in assets")
				s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, ownerAddr, vault.GetAddress(), sdk.NewCoins(*minted)), "should escrow shares into vault account")

				req := types.PendingSwapOut{
					Owner:        ownerAddr.String(),
					VaultAddress: vaultAddr.String(),
					RedeemDenom:  underlyingDenom,
					Shares:       *minted,
				}
				id, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, duePayoutTime, &req)
				s.Require().NoError(err, "should successfully enqueue request")
				return ownerAddr, id
			},
			posthandler: func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, principalAddress sdk.AccAddress, shares sdk.Coin, testBlockTime time.Time) {
				s.assertBalance(ownerAddr, shareDenom, shares.Amount)
				reason := types.RefundReasonRecipientMissingAttributes

				s.Require().Zero(s.countPendingSwapOuts(), "queue should be empty after a failed payout is refunded")

				expectedEvents := sdk.Events{}
				expectedEvents = append(expectedEvents, createSendCoinEvents(vaultAddr.String(), ownerAddr.String(), shares.String())...)
				expectedEvent, err := sdk.TypedEventToEvent(types.NewEventSwapOutRefunded(vaultAddr.String(), ownerAddr.String(), shares, reqID, reason))
				s.Require().NoError(err, "should not error converting typed EventSwapOutRefunded")
				expectedEvents = append(expectedEvents, expectedEvent)
				s.Assert().Equal(
					normalizeEvents(expectedEvents),
					normalizeEvents(s.ctx.EventManager().Events()),
					"a single EventSwapOutRefunded should be emitted",
				)
			},
			batchSize: keeper.MaxSwapOutBatchSize,
		},
		{
			name: "failed payout reconcile refunds shares",
			setup: func(shareDenom string, vaultAddr sdk.AccAddress, shares sdk.Coin) (sdk.AccAddress, uint64) {
				ownerAddr := s.CreateAndFundAccount(assets)
				vault := s.setupBaseVault(underlyingDenom, shareDenom)

				minted, err := s.k.SwapIn(s.ctx, vaultAddr, ownerAddr, assets)
				s.Require().NoError(err, "should successfully swap in assets")
				s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, ownerAddr, vault.GetAddress(), sdk.NewCoins(*minted)), "should escrow shares into vault account")

				vault.PeriodStart = 1
				vault.CurrentInterestRate = "abc"
				s.k.AuthKeeper.SetAccount(s.ctx, vault)

				req := types.PendingSwapOut{
					Owner:        ownerAddr.String(),
					VaultAddress: vaultAddr.String(),
					RedeemDenom:  underlyingDenom,
					Shares:       *minted,
				}
				id, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, duePayoutTime, &req)
				s.Require().NoError(err, "should successfully enqueue request")
				return ownerAddr, id
			},
			posthandler: func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, principalAddress sdk.AccAddress, shares sdk.Coin, testBlockTime time.Time) {
				s.assertBalance(ownerAddr, shareDenom, shares.Amount)
				reason := types.RefundReasonReconcileFailure

				expectedEvents := sdk.Events{}
				expectedEvents = append(expectedEvents, createSendCoinEvents(vaultAddr.String(), ownerAddr.String(), shares.String())...)
				expectedEvent, err := sdk.TypedEventToEvent(types.NewEventSwapOutRefunded(vaultAddr.String(), ownerAddr.String(), shares, reqID, reason))
				s.Require().NoError(err, "should not error converting typed EventSwapOutRefunded")
				expectedEvents = append(expectedEvents, expectedEvent)
				s.Assert().Equal(
					normalizeEvents(expectedEvents),
					normalizeEvents(s.ctx.EventManager().Events()),
					"a single EventSwapOutRefunded should be emitted",
				)
			},
			batchSize: keeper.MaxSwapOutBatchSize,
		},
		{
			name: "stale foreign redeem denom in queue entry pays out in the underlying",
			setup: func(shareDenom string, vaultAddr sdk.AccAddress, shares sdk.Coin) (sdk.AccAddress, uint64) {
				ownerAddr := s.CreateAndFundAccount(assets)
				vault := s.setupBaseVault(underlyingDenom, shareDenom)

				minted, err := s.k.SwapIn(s.ctx, vaultAddr, ownerAddr, assets)
				s.Require().NoError(err, "should successfully swap in assets")
				s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, ownerAddr, vault.GetAddress(), sdk.NewCoins(*minted)), "should escrow shares into vault account")

				req := types.PendingSwapOut{
					Owner:        ownerAddr.String(),
					VaultAddress: vaultAddr.String(),
					RedeemDenom:  "badredeemdenom",
					Shares:       *minted,
				}
				id, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, duePayoutTime, &req)
				s.Require().NoError(err, "should successfully enqueue request")
				return ownerAddr, id
			},
			posthandler: func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, principalAddress sdk.AccAddress, shares sdk.Coin, testBlockTime time.Time) {
				payout := s.k.BankKeeper.GetBalance(s.ctx, ownerAddr, underlyingDenom)
				s.Require().True(payout.Amount.IsPositive(), "owner should be paid in the underlying asset even when the stored redeem denom is foreign")
				s.assertBalance(ownerAddr, "badredeemdenom", math.ZeroInt())
				supply := s.k.BankKeeper.GetSupply(s.ctx, shareDenom)
				s.Require().True(supply.Amount.IsZero(), "escrowed shares should be burned after the underlying payout")
			},
			batchSize: keeper.MaxSwapOutBatchSize,
		},
		{
			name: "due request for paused vault is dequeued and refunded",
			setup: func(shareDenom string, vaultAddr sdk.AccAddress, shares sdk.Coin) (sdk.AccAddress, uint64) {
				ownerAddr := s.CreateAndFundAccount(assets)
				vault := s.setupBaseVault(underlyingDenom, shareDenom)

				minted, err := s.k.SwapIn(s.ctx, vaultAddr, ownerAddr, assets)
				s.Require().NoError(err, "should successfully swap in assets")
				s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, ownerAddr, vault.GetAddress(), sdk.NewCoins(*minted)), "should escrow shares into vault account")

				req := types.PendingSwapOut{
					Owner:        ownerAddr.String(),
					VaultAddress: vaultAddr.String(),
					RedeemDenom:  underlyingDenom,
					Shares:       *minted,
				}
				id, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, duePayoutTime, &req)
				s.Require().NoError(err, "should successfully enqueue request")

				vault, err = s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "should successfully get vault")
				vault.Paused = true
				s.Require().NoError(s.k.SetVaultAccount(s.ctx, vault), "should successfully pause vault")
				return ownerAddr, id
			},
			posthandler: func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, principalAddress sdk.AccAddress, shares sdk.Coin, testBlockTime time.Time) {
				s.assertBalance(ownerAddr, shareDenom, shares.Amount)
				s.assertBalance(vaultAddr, shareDenom, math.ZeroInt())
				s.Require().Zero(s.countPendingSwapOuts(), "queue should be empty after the paused-vault request is refunded")

				expectedEvents := sdk.Events{}
				expectedEvents = append(expectedEvents, createSendCoinEvents(vaultAddr.String(), ownerAddr.String(), shares.String())...)
				refundEvent, err := sdk.TypedEventToEvent(types.NewEventSwapOutRefunded(vaultAddr.String(), ownerAddr.String(), shares, reqID, types.RefundReasonVaultPaused))
				s.Require().NoError(err, "should not error converting typed EventSwapOutRefunded")
				expectedEvents = append(expectedEvents, refundEvent)
				s.Assert().Equal(
					normalizeEvents(expectedEvents),
					normalizeEvents(s.ctx.EventManager().Events()),
					"a single EventSwapOutRefunded with reason %s should be emitted", types.RefundReasonVaultPaused,
				)
			},
			batchSize: keeper.MaxSwapOutBatchSize,
		},
		{
			name: "paused vault refund failure leaves request queued",
			setup: func(shareDenom string, vaultAddr sdk.AccAddress, shares sdk.Coin) (sdk.AccAddress, uint64) {
				ownerAddr := s.CreateAndFundAccount(assets)
				vault := s.setupBaseVault(underlyingDenom, shareDenom)

				minted, err := s.k.SwapIn(s.ctx, vaultAddr, ownerAddr, assets)
				s.Require().NoError(err, "should successfully swap in assets")
				s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, ownerAddr, vault.GetAddress(), sdk.NewCoins(*minted)), "should escrow shares into vault account")

				req := types.PendingSwapOut{
					Owner:        ownerAddr.String(),
					VaultAddress: vaultAddr.String(),
					RedeemDenom:  underlyingDenom,
					Shares:       *minted,
				}
				id, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, duePayoutTime, &req)
				s.Require().NoError(err, "should successfully enqueue request")

				vault, err = s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "should successfully get vault")
				vault.Paused = true
				s.Require().NoError(s.k.SetVaultAccount(s.ctx, vault), "should successfully pause vault")

				s.Require().NoError(
					s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), vaultAddr, s.adminAddr, sdk.NewCoins(*minted)),
					"should drain escrowed shares to force the refund to fail",
				)
				return ownerAddr, id
			},
			posthandler: func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, principalAddress sdk.AccAddress, shares sdk.Coin, testBlockTime time.Time) {
				s.assertBalance(ownerAddr, shareDenom, math.ZeroInt())
				s.Require().Equal(1, s.countPendingSwapOuts(), "request should stay queued when the paused-vault refund fails")
				s.Assert().Empty(s.ctx.EventManager().Events(), "no events should be committed when the refund cache context is discarded")
			},
			batchSize: keeper.MaxSwapOutBatchSize,
		},
		{
			name: "request for non-existent vault is skipped and dequeued",
			setup: func(shareDenom string, vaultAddr sdk.AccAddress, shares sdk.Coin) (sdk.AccAddress, uint64) {
				ownerAddr := s.CreateAndFundAccount(sdk.Coin{})
				req := types.PendingSwapOut{
					Owner:        ownerAddr.String(),
					VaultAddress: vaultAddr.String(),
					RedeemDenom:  underlyingDenom,
					Shares:       shares,
				}
				id, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, duePayoutTime, &req)
				s.Require().NoError(err, "should successfully enqueue request for non-existent vault")
				return ownerAddr, id
			},
			posthandler: func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, principalAddress sdk.AccAddress, shares sdk.Coin, testBlockTime time.Time) {
				s.Require().Zero(s.countPendingSwapOuts(), "queue should be empty after processing the non-existent vault request")
			},
			batchSize: keeper.MaxSwapOutBatchSize,
		},
	}

	for i, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			shareDenom := fmt.Sprintf("vshare%d", i)
			vaultAddr := types.GetVaultAddress(shareDenom)
			principalAddress := markertypes.MustGetMarkerAddress(shareDenom)
			shares := sdk.NewInt64Coin(shareDenom, 50_000_000)

			var ownerAddr sdk.AccAddress
			var reqID uint64
			s.ctx = s.ctx.WithBlockTime(testBlockTime)
			if tc.setup != nil {
				ownerAddr, reqID = tc.setup(shareDenom, vaultAddr, shares)
			}
			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			s.ctx = s.ctx.WithBlockTime(testBlockTime)

			err := s.k.TestAccessor_processPendingSwapOuts(s.T(), s.ctx, tc.batchSize)

			if tc.expectedError != "" {
				s.Require().Error(err, "expected error during processPendingSwapOuts")
				s.Require().ErrorContains(err, tc.expectedError, "error message mismatch")
			} else {
				s.Require().NoError(err, "unexpected error during processPendingSwapOuts")
			}

			if tc.posthandler != nil {
				tc.posthandler(ownerAddr, reqID, shareDenom, vaultAddr, principalAddress, shares, testBlockTime)
			}
		})
	}
}

func (s *TestSuite) TestKeeper_ProcessPendingSwapOuts_PausedFrontSharesBatchBudget() {
	testBlockTime := time.Now().UTC()
	duePayoutTime := testBlockTime.Add(-1 * time.Hour).Unix()
	expeditedTime := int64(0)

	s.SetupTest()
	s.ctx = s.ctx.WithBlockTime(testBlockTime)

	enqueueEscrowedSwapOut := func(underlyingDenom, shareDenom string, pendingTime int64) (ownerAddr sdk.AccAddress, assets, minted sdk.Coin) {
		vaultAddr := types.GetVaultAddress(shareDenom)
		assets = sdk.NewInt64Coin(underlyingDenom, 50)
		ownerAddr = s.CreateAndFundAccount(assets)
		vault := s.setupBaseVault(underlyingDenom, shareDenom)

		mintedShares, err := s.k.SwapIn(s.ctx, vaultAddr, ownerAddr, assets)
		s.Require().NoError(err, "should swap in assets for share denom %s", shareDenom)
		s.Require().NoError(
			s.k.BankKeeper.SendCoins(s.ctx, ownerAddr, vault.GetAddress(), sdk.NewCoins(*mintedShares)),
			"should escrow shares into vault account for share denom %s", shareDenom,
		)

		req := types.PendingSwapOut{
			Owner:        ownerAddr.String(),
			VaultAddress: vaultAddr.String(),
			RedeemDenom:  underlyingDenom,
			Shares:       *mintedShares,
		}
		_, err = s.k.PendingSwapOutQueue.Enqueue(s.ctx, pendingTime, &req)
		s.Require().NoError(err, "should enqueue swap out for share denom %s at time %d", shareDenom, pendingTime)
		return ownerAddr, assets, *mintedShares
	}

	pausedShareDenom := "vsharefrontpaused"
	activeShareDenom := "vsharebackactive"
	pausedOwner, _, pausedShares := enqueueEscrowedSwapOut("pausedylds", pausedShareDenom, expeditedTime)
	activeOwner, activeAssets, _ := enqueueEscrowedSwapOut("activeylds", activeShareDenom, duePayoutTime)

	pausedVaultAddr := types.GetVaultAddress(pausedShareDenom)
	pausedVault, err := s.k.GetVault(s.ctx, pausedVaultAddr)
	s.Require().NoError(err, "should get the vault camped at the front of the queue")
	pausedVault.Paused = true
	s.Require().NoError(s.k.SetVaultAccount(s.ctx, pausedVault), "should pause the vault camped at the front of the queue")

	s.Require().NoError(s.k.TestAccessor_processPendingSwapOuts(s.T(), s.ctx, 1), "first block with batch size 1 should not error")

	s.assertBalance(pausedOwner, pausedShareDenom, pausedShares.Amount)
	s.assertBalance(activeOwner, activeAssets.Denom, math.ZeroInt())
	s.Require().Equal(1, s.countPendingSwapOuts(), "the paused entry should consume the batch budget and be refunded, leaving only the active request queued")

	s.Require().NoError(s.k.TestAccessor_processPendingSwapOuts(s.T(), s.ctx, 1), "second block with batch size 1 should not error")

	s.assertBalance(activeOwner, activeAssets.Denom, activeAssets.Amount)
	s.Require().Zero(s.countPendingSwapOuts(), "the active request should be paid out once the paused entry no longer camps at the queue front")
}

func (s *TestSuite) TestKeeper_ProcessSwapOutJobs() {
	underlyingDenom := "ylds"
	assets := sdk.NewInt64Coin(underlyingDenom, 50)
	testBlockTime := time.Now().UTC()
	duePayoutTime := testBlockTime.Add(-1 * time.Hour).Unix()

	tests := []struct {
		name        string
		setup       func(shareDenom string, vaultAddr sdk.AccAddress) (ownerAddr sdk.AccAddress, mintedShares sdk.Coin, reqID uint64, req types.PendingSwapOut)
		act         func(shareDenom string, vaultAddr sdk.AccAddress, ownerAddr sdk.AccAddress, mintedShares sdk.Coin, reqID uint64, req types.PendingSwapOut)
		posthandler func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, mintedShares sdk.Coin)
	}{
		{
			name: "request is dequeued and refunded if vault becomes paused after collection",
			setup: func(shareDenom string, vaultAddr sdk.AccAddress) (sdk.AccAddress, sdk.Coin, uint64, types.PendingSwapOut) {
				return s.enqueueDueSwapOut(underlyingDenom, shareDenom, assets, duePayoutTime)
			},
			act: func(shareDenom string, vaultAddr sdk.AccAddress, ownerAddr sdk.AccAddress, mintedShares sdk.Coin, reqID uint64, req types.PendingSwapOut) {
				jobs := []types.PayoutJob{
					types.NewPayoutJob(duePayoutTime, reqID, vaultAddr, req),
				}

				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "should successfully get vault")
				vault.Paused = true
				s.Require().NoError(s.k.SetVaultAccount(s.ctx, vault), "should successfully set vault to paused")

				s.k.TestAccessor_processSwapOutJobs(s.T(), s.ctx, jobs)
			},
			posthandler: func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, mintedShares sdk.Coin) {
				s.Require().Zero(s.countPendingSwapOuts(), "queue should be empty after the paused-vault request is refunded")

				s.assertBalance(ownerAddr, underlyingDenom, math.ZeroInt())
				s.assertBalance(ownerAddr, mintedShares.Denom, mintedShares.Amount)
				s.assertBalance(vaultAddr, mintedShares.Denom, math.ZeroInt())

				expectedEvents := sdk.Events{}
				expectedEvents = append(expectedEvents, createSendCoinEvents(vaultAddr.String(), ownerAddr.String(), mintedShares.String())...)
				refundEvent, err := sdk.TypedEventToEvent(types.NewEventSwapOutRefunded(vaultAddr.String(), ownerAddr.String(), mintedShares, reqID, types.RefundReasonVaultPaused))
				s.Require().NoError(err, "should not error converting typed EventSwapOutRefunded")
				expectedEvents = append(expectedEvents, refundEvent)
				s.Assert().Equal(
					normalizeEvents(expectedEvents),
					normalizeEvents(s.ctx.EventManager().Events()),
					"a single EventSwapOutRefunded with reason %s should be emitted", types.RefundReasonVaultPaused,
				)
			},
		},
		{
			name: "critical share-transfer failure preserves entry, escrow, and pauses vault",
			setup: func(shareDenom string, vaultAddr sdk.AccAddress) (sdk.AccAddress, sdk.Coin, uint64, types.PendingSwapOut) {
				return s.enqueueDueSwapOut(underlyingDenom, shareDenom, assets, duePayoutTime)
			},
			act: func(shareDenom string, vaultAddr sdk.AccAddress, ownerAddr sdk.AccAddress, mintedShares sdk.Coin, reqID uint64, req types.PendingSwapOut) {
				principalAddress := markertypes.MustGetMarkerAddress(shareDenom)
				s.installBankSendFault(func(from, to sdk.AccAddress, amt sdk.Coins) error {
					if from.Equals(vaultAddr) && to.Equals(principalAddress) && amt.AmountOf(shareDenom).Equal(mintedShares.Amount) {
						return fmt.Errorf("forced share transfer failure")
					}
					return nil
				})
				s.k.TestAccessor_processSwapOutJobs(s.T(), s.ctx, []types.PayoutJob{types.NewPayoutJob(duePayoutTime, reqID, vaultAddr, req)})
			},
			posthandler: func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, mintedShares sdk.Coin) {
				s.assertSwapOutEntryPreservedAndPaused(reqID, vaultAddr, ownerAddr, mintedShares, underlyingDenom)
			},
		},
		{
			name: "critical burn failure preserves entry, escrow, and pauses vault",
			setup: func(shareDenom string, vaultAddr sdk.AccAddress) (sdk.AccAddress, sdk.Coin, uint64, types.PendingSwapOut) {
				return s.enqueueDueSwapOut(underlyingDenom, shareDenom, assets, duePayoutTime)
			},
			act: func(shareDenom string, vaultAddr sdk.AccAddress, ownerAddr sdk.AccAddress, mintedShares sdk.Coin, reqID uint64, req types.PendingSwapOut) {
				s.installMarkerBurnFault(func(_ sdk.AccAddress, coin sdk.Coin) error {
					if coin.Denom == shareDenom {
						return fmt.Errorf("forced burn failure")
					}
					return nil
				})
				s.k.TestAccessor_processSwapOutJobs(s.T(), s.ctx, []types.PayoutJob{types.NewPayoutJob(duePayoutTime, reqID, vaultAddr, req)})
			},
			posthandler: func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, mintedShares sdk.Coin) {
				s.assertSwapOutEntryPreservedAndPaused(reqID, vaultAddr, ownerAddr, mintedShares, underlyingDenom)
			},
		},
		{
			name: "critical refund failure preserves entry, escrow, and pauses vault",
			setup: func(shareDenom string, vaultAddr sdk.AccAddress) (sdk.AccAddress, sdk.Coin, uint64, types.PendingSwapOut) {
				return s.enqueueDueSwapOut(underlyingDenom, shareDenom, assets, duePayoutTime)
			},
			act: func(shareDenom string, vaultAddr sdk.AccAddress, ownerAddr sdk.AccAddress, mintedShares sdk.Coin, reqID uint64, req types.PendingSwapOut) {
				principalAddress := markertypes.MustGetMarkerAddress(shareDenom)
				s.installBankSendFault(func(from, to sdk.AccAddress, amt sdk.Coins) error {
					if from.Equals(principalAddress) && to.Equals(ownerAddr) {
						return fmt.Errorf("forced payout failure")
					}
					if from.Equals(vaultAddr) && to.Equals(ownerAddr) {
						return fmt.Errorf("forced refund failure")
					}
					return nil
				})
				s.k.TestAccessor_processSwapOutJobs(s.T(), s.ctx, []types.PayoutJob{types.NewPayoutJob(duePayoutTime, reqID, vaultAddr, req)})
			},
			posthandler: func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, mintedShares sdk.Coin) {
				s.assertSwapOutEntryPreservedAndPaused(reqID, vaultAddr, ownerAddr, mintedShares, underlyingDenom)
			},
		},
		{
			name: "preserved entry completes after vault is unpaused",
			setup: func(shareDenom string, vaultAddr sdk.AccAddress) (sdk.AccAddress, sdk.Coin, uint64, types.PendingSwapOut) {
				return s.enqueueDueSwapOut(underlyingDenom, shareDenom, assets, duePayoutTime)
			},
			act: func(shareDenom string, vaultAddr sdk.AccAddress, ownerAddr sdk.AccAddress, mintedShares sdk.Coin, reqID uint64, req types.PendingSwapOut) {
				principalAddress := markertypes.MustGetMarkerAddress(shareDenom)
				failOnce := true
				s.installBankSendFault(func(from, to sdk.AccAddress, amt sdk.Coins) error {
					if failOnce && from.Equals(vaultAddr) && to.Equals(principalAddress) && amt.AmountOf(shareDenom).Equal(mintedShares.Amount) {
						failOnce = false
						return fmt.Errorf("forced one-time share transfer failure")
					}
					return nil
				})
				jobs := []types.PayoutJob{types.NewPayoutJob(duePayoutTime, reqID, vaultAddr, req)}
				s.k.TestAccessor_processSwapOutJobs(s.T(), s.ctx, jobs)

				vault, err := s.k.GetVault(s.ctx, vaultAddr)
				s.Require().NoError(err, "should successfully get vault after critical failure")
				s.Require().True(vault.Paused, "vault should be paused after the critical failure")
				vault.Paused = false
				vault.PausedReason = ""
				s.Require().NoError(s.k.SetVaultAccount(s.ctx, vault), "should successfully unpause vault")

				s.k.TestAccessor_processSwapOutJobs(s.T(), s.ctx, jobs)
			},
			posthandler: func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, mintedShares sdk.Coin) {
				var entries []uint64
				err := s.k.PendingSwapOutQueue.Walk(s.ctx, func(_ int64, id uint64, _ sdk.AccAddress, _ types.PendingSwapOut) (bool, error) {
					entries = append(entries, id)
					return false, nil
				})
				s.Require().NoError(err, "walking the queue should not error")
				s.Require().Empty(entries, "entry should be removed after a successful payout on the second pass")

				s.assertBalance(ownerAddr, underlyingDenom, assets.Amount)
				s.assertBalance(vaultAddr, mintedShares.Denom, math.ZeroInt())
				supply := s.k.BankKeeper.GetSupply(s.ctx, shareDenom)
				s.Require().True(supply.Amount.IsZero(), "share supply should be zero after the burn completes")
			},
		},
	}

	for i, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			shareDenom := fmt.Sprintf("vsharep%d", i)
			vaultAddr := types.GetVaultAddress(shareDenom)

			s.ctx = s.ctx.WithBlockTime(testBlockTime)
			ownerAddr, mintedShares, reqID, req := tc.setup(shareDenom, vaultAddr)

			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			s.ctx = s.ctx.WithBlockTime(testBlockTime)

			tc.act(shareDenom, vaultAddr, ownerAddr, mintedShares, reqID, req)

			if tc.posthandler != nil {
				tc.posthandler(ownerAddr, reqID, shareDenom, vaultAddr, mintedShares)
			}
		})
	}
}

func (s *TestSuite) TestKeeper_ProcessSingleWithdrawal_AddressErrors() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	assets := sdk.NewInt64Coin(underlyingDenom, 50)
	vaultAddr := types.GetVaultAddress(shareDenom)
	ownerAddr := s.CreateAndFundAccount(assets)
	vault := s.setupBaseVault(underlyingDenom, shareDenom)
	minted, err := s.k.SwapIn(s.ctx, vaultAddr, ownerAddr, assets)
	s.Require().NoError(err, "should successfully swap in assets for setup")

	tests := []struct {
		name          string
		req           types.PendingSwapOut
		expectedError string
	}{
		{
			name: "invalid vault address",
			req: types.PendingSwapOut{
				Owner:        ownerAddr.String(),
				VaultAddress: "invalidvaultaddress",
				RedeemDenom:  underlyingDenom,
				Shares:       *minted,
			},
			expectedError: "invalid vault address invalidvaultaddress:",
		},
		{
			name: "invalid owner address",
			req: types.PendingSwapOut{
				Owner:        "invalidowneraddress",
				VaultAddress: vaultAddr.String(),
				RedeemDenom:  underlyingDenom,
				Shares:       *minted,
			},
			expectedError: "invalid owner address invalidowneraddress:",
		},
		{
			name: "invalid principal address (share denom)",
			req: types.PendingSwapOut{
				Owner:        ownerAddr.String(),
				VaultAddress: vaultAddr.String(),
				RedeemDenom:  underlyingDenom,
				Shares:       sdk.Coin{"invalid!share", math.NewInt(1)},
			},
			expectedError: "invalid principal address for denom invalid!share:",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			err := s.k.TestAccessor_processSingleWithdrawal(s.T(), s.ctx, 1, tc.req, *vault)
			s.Require().Error(err, "should return an error for invalid address/denom parsing")
			s.Require().ErrorContains(err, tc.expectedError, "the error message should contain the expected address parsing failure")
		})
	}
}

func (s *TestSuite) TestKeeper_RefundWithdrawal_AddressErrors() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	assets := sdk.NewInt64Coin(underlyingDenom, 50)
	vaultAddr := types.GetVaultAddress(shareDenom)
	ownerAddr := s.CreateAndFundAccount(assets)
	s.setupBaseVault(underlyingDenom, shareDenom)
	minted, err := s.k.SwapIn(s.ctx, vaultAddr, ownerAddr, assets)
	s.Require().NoError(err, "should successfully swap in assets for setup")

	tests := []struct {
		name          string
		req           types.PendingSwapOut
		expectedError string
	}{
		{
			name: "invalid vault address",
			req: types.PendingSwapOut{
				Owner:        ownerAddr.String(),
				VaultAddress: "invalidvaultaddress",
				RedeemDenom:  underlyingDenom,
				Shares:       *minted,
			},
			expectedError: "invalid vault address invalidvaultaddress:",
		},
		{
			name: "invalid owner address",
			req: types.PendingSwapOut{
				Owner:        "invalidowneraddress",
				VaultAddress: vaultAddr.String(),
				RedeemDenom:  underlyingDenom,
				Shares:       *minted,
			},
			expectedError: "invalid owner address invalidowneraddress:",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			err := s.k.TestAccessor_refundWithdrawal(s.T(), s.ctx, 1, tc.req, types.RefundReasonRecipientMissingAttributes)
			s.Require().Error(err, "should return an error for invalid address parsing")
			s.Require().ErrorContains(err, tc.expectedError, "the error message should contain the expected address parsing failure")
		})
	}
}

func (s *TestSuite) TestKeeper_GetRefundReason() {
	tests := []struct {
		name           string
		err            error
		expectedReason string
	}{
		{
			name:           "insufficient funds is classified from the typed sdk error",
			err:            fmt.Errorf("failed to payout assets to owner: %w", sdkerrors.ErrInsufficientFunds),
			expectedReason: types.RefundReasonInsufficientFunds,
		},
		{
			name:           "nav not found is classified from the typed sentinel",
			err:            fmt.Errorf("failed to convert shares to redeem coin: %w", types.ErrNavNotFound),
			expectedReason: types.RefundReasonNavNotFound,
		},
		{
			name:           "reconcile failure is classified from the typed sentinel",
			err:            fmt.Errorf("%w: failed to reconcile vault: boom", types.ErrReconcileFailure),
			expectedReason: types.RefundReasonReconcileFailure,
		},
		{
			name:           "marker not active matches the real upstream send message",
			err:            errors.New("cannot send vaultshares coins: marker status (proposed) is not active"),
			expectedReason: types.RefundReasonMarkerNotActive,
		},
		{
			name:           "marker not active matches the real upstream withdraw message",
			err:            errors.New("cannot withdraw 100vaultshares from vaultshares marker (cosmos1abc): marker status (finalized) is not active"),
			expectedReason: types.RefundReasonMarkerNotActive,
		},
		{
			name:           "single missing required attribute matches the real upstream message",
			err:            errors.New(`address cosmos1abc does not contain the "vaultshares" required attribute: "kyc.pb"`),
			expectedReason: types.RefundReasonRecipientMissingAttributes,
		},
		{
			name:           "multiple missing required attributes matches the pluralized upstream message",
			err:            errors.New(`address cosmos1abc does not contain the "vaultshares" required attributes: "kyc.pb", "accredited.pb"`),
			expectedReason: types.RefundReasonRecipientMissingAttributes,
		},
		{
			name:           "sender on deny list matches the real upstream message",
			err:            errors.New("cosmos1abc is on deny list for sending restricted marker"),
			expectedReason: types.RefundReasonPermissionDenied,
		},
		{
			name:           "sender without transfer permission matches the real upstream message",
			err:            errors.New("cosmos1abc does not have transfer permissions for vaultshares"),
			expectedReason: types.RefundReasonPermissionDenied,
		},
		{
			name:           "restricted denom sent to fee collector matches the real upstream message",
			err:            errors.New("restricted denom vaultshares cannot be sent to the fee collector"),
			expectedReason: types.RefundReasonRecipientInvalid,
		},
		{
			name:           "unrecognized error falls through to unknown",
			err:            errors.New("a brand new failure mode we have never seen"),
			expectedReason: types.RefundReasonUnknown,
		},
		{
			name:           "nil error returns unknown without panicking",
			err:            nil,
			expectedReason: types.RefundReasonUnknown,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			reason := s.k.TestAccessor_getRefundReason(tc.err)
			s.Equal(tc.expectedReason, reason, "getRefundReason should classify %q as %s", tc.err, tc.expectedReason)
		})
	}
}

// TestKeeper_ProcessSingleWithdrawal_TypedSentinels verifies that the in-module failure paths wrap
// their errors with the typed sentinels, so the %w chain stays intact from the point of failure all
// the way to getRefundReason. A broken chain would silently downgrade the refund reason to unknown.
func (s *TestSuite) TestKeeper_ProcessSingleWithdrawal_TypedSentinels() {
	underlyingDenom := "ylds"
	assets := sdk.NewInt64Coin(underlyingDenom, 50)

	escrowVault := func(shareDenom string, vaultAddr sdk.AccAddress) (*types.VaultAccount, sdk.AccAddress, sdk.Coin) {
		ownerAddr := s.CreateAndFundAccount(assets)
		vault := s.setupBaseVault(underlyingDenom, shareDenom)
		minted, err := s.k.SwapIn(s.ctx, vaultAddr, ownerAddr, assets)
		s.Require().NoError(err, "should swap in assets for share denom %s", shareDenom)
		s.Require().NoError(
			s.k.BankKeeper.SendCoins(s.ctx, ownerAddr, vault.GetAddress(), sdk.NewCoins(*minted)),
			"should escrow shares into vault account for share denom %s", shareDenom,
		)
		return vault, ownerAddr, *minted
	}

	tests := []struct {
		name             string
		setup            func() (types.PendingSwapOut, types.VaultAccount)
		expectedSentinel error
		expectedReason   string
	}{
		{
			name: "reconcile failure surfaces the typed ErrReconcileFailure sentinel",
			setup: func() (types.PendingSwapOut, types.VaultAccount) {
				shareDenom := "vsharerecon"
				vaultAddr := types.GetVaultAddress(shareDenom)
				vault, ownerAddr, minted := escrowVault(shareDenom, vaultAddr)
				vault.PeriodStart = 1
				vault.CurrentInterestRate = "abc"
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
				return types.PendingSwapOut{
					Owner:        ownerAddr.String(),
					VaultAddress: vaultAddr.String(),
					RedeemDenom:  underlyingDenom,
					Shares:       minted,
				}, *vault
			},
			expectedSentinel: types.ErrReconcileFailure,
			expectedReason:   types.RefundReasonReconcileFailure,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			req, vault := tc.setup()
			err := s.k.TestAccessor_processSingleWithdrawal(s.T(), s.ctx, 1, req, vault)
			s.Require().Error(err, "processSingleWithdrawal should fail for %q", tc.name)
			s.Require().ErrorIs(err, tc.expectedSentinel, "error should carry the typed sentinel for %q", tc.name)
			s.Equal(tc.expectedReason, s.k.TestAccessor_getRefundReason(err), "refund reason mismatch for %q", tc.name)
		})
	}
}
