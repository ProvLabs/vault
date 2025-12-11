package keeper_test

import (
	"fmt"
	"time"

	"cosmossdk.io/math"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

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
				s.assertBalance(ownerAddr, underlyingDenom, assets.Amount)
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
				expectedEvents = append(expectedEvents, createMarkerSetNAV(shareDenom, assets, "vault", shares.Amount.Uint64()))
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
				var entries []types.PendingSwapOut
				err := s.k.PendingSwapOutQueue.Walk(s.ctx, func(_ int64, _ uint64, _ sdk.AccAddress, req types.PendingSwapOut) (bool, error) {
					entries = append(entries, req)
					return false, nil
				})
				s.Require().NoError(err)
				s.Require().Empty(entries, "queue should be empty after processing the non-existent vault request")
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
			name: "request is skipped if vault becomes paused after collection",
			setup: func(shareDenom string, vaultAddr sdk.AccAddress) (sdk.AccAddress, sdk.Coin, uint64, types.PendingSwapOut) {
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

				return ownerAddr, *minted, id, req
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
				var entries []uint64
				err := s.k.PendingSwapOutQueue.Walk(s.ctx, func(_ int64, id uint64, _ sdk.AccAddress, _ types.PendingSwapOut) (bool, error) {
					entries = append(entries, id)
					return false, nil
				})
				s.Require().NoError(err, "walking the queue should not error")
				s.Require().Len(entries, 1, "queue should not be empty because the vault was paused")

				s.assertBalance(ownerAddr, underlyingDenom, math.ZeroInt())
				s.assertBalance(vaultAddr, mintedShares.Denom, mintedShares.Amount)
				s.Assert().Empty(s.ctx.EventManager().Events(), "no events should be emitted for a job skipped due to pause")
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
			expectedError: "invalid vault address invalidvaultaddress: decoding bech32 failed: invalid separator index -1",
		},
		{
			name: "invalid owner address",
			req: types.PendingSwapOut{
				Owner:        "invalidowneraddress",
				VaultAddress: vaultAddr.String(),
				RedeemDenom:  underlyingDenom,
				Shares:       *minted,
			},
			expectedError: "invalid owner address invalidowneraddress: decoding bech32 failed: invalid separator index -1",
		},
		{
			name: "invalid principal address (share denom)",
			req: types.PendingSwapOut{
				Owner:        ownerAddr.String(),
				VaultAddress: vaultAddr.String(),
				RedeemDenom:  underlyingDenom,
				Shares:       sdk.Coin{"invalid!share", sdkmath.NewInt(1)},
			},
			expectedError: "invalid principal address for denom invalid!share: invalid denom: invalid!share",
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
			expectedError: "invalid vault address invalidvaultaddress: decoding bech32 failed: invalid separator index -1",
		},
		{
			name: "invalid owner address",
			req: types.PendingSwapOut{
				Owner:        "invalidowneraddress",
				VaultAddress: vaultAddr.String(),
				RedeemDenom:  underlyingDenom,
				Shares:       *minted,
			},
			expectedError: "invalid owner address invalidowneraddress: decoding bech32 failed: invalid separator index -1",
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
