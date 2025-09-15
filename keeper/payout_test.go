package keeper_test

import (
	"fmt"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	markertypes "github.com/provenance-io/provenance/x/marker/types"

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

				expectedEvents := sdk.Events{}
				expectedEvents = append(expectedEvents, createSendCoinEvents(principalAddress.String(), ownerAddr.String(), assets.String())...)
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
		},
		{
			name: "successful payout of due request with reconcile",
			setup: func(shareDenom string, vaultAddr sdk.AccAddress, shares sdk.Coin) (sdk.AccAddress, uint64) {
				ownerAddr := s.CreateAndFundAccount(assets)
				vault := s.setupBaseVault(underlyingDenom, shareDenom)

				minted, err := s.k.SwapIn(s.ctx, vaultAddr, ownerAddr, assets)
				s.Require().NoError(err, "should successfully swap in assets")
				s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, ownerAddr, vault.GetAddress(), sdk.NewCoins(*minted)), "should escrow shares into vault account")

				vault.PeriodStart = 1
				s.Require().NoError(s.k.SetVaultAccount(s.ctx, vault), "must update vault account period")

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

				expectedEvents := sdk.Events{}
				reconcileEvent, err := sdk.TypedEventToEvent(types.NewEventVaultReconcile(vaultAddr.String(), assets, assets, vault.CurrentInterestRate, testBlockTime.Unix()-1, math.NewInt(0)))
				s.Require().NoError(err, "should not error converting typed EventVaultReconciled")
				expectedEvents = append(expectedEvents, reconcileEvent)
				expectedEvents = append(expectedEvents, createSendCoinEvents(principalAddress.String(), ownerAddr.String(), assets.String())...)
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
				s.Require().NoError(err)
				return ownerAddr, id
			},
			posthandler: func(ownerAddr sdk.AccAddress, reqID uint64, shareDenom string, vaultAddr sdk.AccAddress, principalAddress sdk.AccAddress, shares sdk.Coin, testBlockTime time.Time) {
				s.assertBalance(ownerAddr, shareDenom, shares.Amount)
				reason := types.RefundReasonRecipientMissingAttributes

				expectedEvents := sdk.Events{}
				expectedEvents = append(expectedEvents, sdk.NewEvent(
					banktypes.EventTypeCoinSpent,
					sdk.NewAttribute(banktypes.AttributeKeySpender, principalAddress.String()),
					sdk.NewAttribute(sdk.AttributeKeyAmount, assets.String()),
				))
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
		},
		{
			name: "refund when calculating redeem denom fails",
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
				s.assertBalance(ownerAddr, shareDenom, shares.Amount)
				reason := types.RefundReasonNavNotFound

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
				var entries []types.PendingSwapOut
				err := s.k.PendingSwapOutQueue.Walk(s.ctx, func(_ int64, _ uint64, _ sdk.AccAddress, req types.PendingSwapOut) (bool, error) {
					entries = append(entries, req)
					return false, nil
				})
				s.Require().NoError(err)
				s.Require().Empty(entries, "queue should be empty after processing the non-existent vault request")
			},
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

			err := s.k.ProcessPendingSwapOuts(s.ctx)

			if tc.expectedError != "" {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.expectedError)
			} else {
				s.Require().NoError(err)
			}

			if tc.posthandler != nil {
				tc.posthandler(ownerAddr, reqID, shareDenom, vaultAddr, principalAddress, shares, testBlockTime)
			}
		})
	}
}
