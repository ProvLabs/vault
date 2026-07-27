package keeper_test

import (
	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provenance-io/provenance/x/exchange"
	markertypes "github.com/provenance-io/provenance/x/marker/types"

	"github.com/provlabs/vault/types"
)

func (s *TestSuite) TestKeeper_RemoveDrainedSettlementNAV() {
	underlying := "under"
	share := "vshare"
	asset := "rwacoin"
	seededPrice := sdk.NewInt64Coin(underlying, 5)
	seededVolume := sdkmath.NewInt(10)

	tests := []struct {
		name                string
		seedNav             bool
		fundPrincipal       sdk.Coins
		direction           string
		expectedErrContains string
		expectNavRemoved    bool
	}{
		{
			name:          "inbound settlement keeps the NAV entry the authority set",
			seedNav:       true,
			fundPrincipal: sdk.NewCoins(sdk.NewInt64Coin(asset, 10)),
			direction:     types.AssetDirectionInbound,
		},
		{
			name:          "outbound settlement with a remaining principal balance keeps the NAV entry",
			seedNav:       true,
			fundPrincipal: sdk.NewCoins(sdk.NewInt64Coin(asset, 5)),
			direction:     types.AssetDirectionOutbound,
		},
		{
			name:             "outbound settlement with a drained principal removes the NAV entry",
			seedNav:          true,
			direction:        types.AssetDirectionOutbound,
			expectNavRemoved: true,
		},
		{
			name:                "outbound settlement of a drained denom with no NAV entry surfaces the missing entry",
			direction:           types.AssetDirectionOutbound,
			expectedErrContains: "failed to remove internal NAV for drained denom",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			origCtx := s.ctx
			defer func() { s.ctx = origCtx }()
			s.ctx, _ = s.ctx.CacheContext()

			vault, principalAddr := s.setupAssetSettlementVault(underlying, share)
			vaultAddr := vault.GetAddress()
			s.requireSimpleMarker(asset)
			if tc.seedNav {
				s.setVaultNAV(vault, asset, seededPrice, seededVolume.Int64())
			}
			if !tc.fundPrincipal.IsZero() {
				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, principalAddr, tc.fundPrincipal), "failed to fund principal with %s", tc.fundPrincipal)
			}

			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			err := s.k.TestAccessor_removeDrainedSettlementNAV(s.T(), s.ctx, vault, asset, tc.direction)

			var removedEvents []sdk.Event
			for _, ev := range s.ctx.EventManager().Events() {
				if ev.Type == "provlabs.vault.v1.EventNAVRemoved" {
					removedEvents = append(removedEvents, ev)
				}
			}

			if tc.expectedErrContains != "" {
				s.Require().ErrorContains(err, tc.expectedErrContains, "removeDrainedSettlementNAV should fail for case %q", tc.name)
				return
			}
			s.Require().NoError(err, "removeDrainedSettlementNAV should succeed for denom %s direction %s", asset, tc.direction)

			if tc.expectNavRemoved {
				_, err := s.k.GetVaultNAV(s.ctx, vaultAddr, asset)
				s.Assert().ErrorIs(err, collections.ErrNotFound, "NAV entry for %s should be removed after draining the principal", asset)
				s.Assert().Len(removedEvents, 1, "draining settlement should emit exactly one EventNAVRemoved")
				return
			}

			s.Assert().Empty(removedEvents, "non-draining settlement should not emit EventNAVRemoved for case %q", tc.name)
			stored, err := s.k.GetVaultNAV(s.ctx, vaultAddr, asset)
			s.Require().NoError(err, "NAV entry for %s should still exist after settlement", asset)
			s.Assert().Equal(seededPrice, stored.Price, "settlement must not rewrite the NAV price the authority set for case %q", tc.name)
			s.Assert().Equal(seededVolume, stored.Volume, "settlement must not rewrite the NAV volume the authority set for case %q", tc.name)
			s.Assert().Equal("test", stored.Source, "settlement must not reattribute the NAV source away from the authority for case %q", tc.name)
		})
	}
}

func (s *TestSuite) TestSettlementLegCoins() {
	tests := []struct {
		name                string
		sourceAmount        sdk.Coins
		targetAmount        sdk.Coins
		direction           string
		expectedAssetCoin   sdk.Coin
		expectedPaymentCoin sdk.Coin
		expectedErrContains string
	}{
		{
			name:                "inbound payment yields source as the asset coin and target as the payment coin",
			sourceAmount:        sdk.NewCoins(sdk.NewInt64Coin("rwa", 10)),
			targetAmount:        sdk.NewCoins(sdk.NewInt64Coin("pay", 5)),
			direction:           types.AssetDirectionInbound,
			expectedAssetCoin:   sdk.NewInt64Coin("rwa", 10),
			expectedPaymentCoin: sdk.NewInt64Coin("pay", 5),
		},
		{
			name:                "outbound payment yields target as the asset coin and source as the payment coin",
			sourceAmount:        sdk.NewCoins(sdk.NewInt64Coin("pay", 5)),
			targetAmount:        sdk.NewCoins(sdk.NewInt64Coin("rwa", 10)),
			direction:           types.AssetDirectionOutbound,
			expectedAssetCoin:   sdk.NewInt64Coin("rwa", 10),
			expectedPaymentCoin: sdk.NewInt64Coin("pay", 5),
		},
		{
			name:                "empty asset leg is rejected",
			sourceAmount:        sdk.NewCoins(),
			targetAmount:        sdk.NewCoins(sdk.NewInt64Coin("pay", 5)),
			direction:           types.AssetDirectionInbound,
			expectedErrContains: "one asset coin",
		},
		{
			name:                "asset leg with multiple coins is rejected",
			sourceAmount:        sdk.NewCoins(sdk.NewInt64Coin("rwa", 10), sdk.NewInt64Coin("othercoin", 5)),
			targetAmount:        sdk.NewCoins(sdk.NewInt64Coin("pay", 5)),
			direction:           types.AssetDirectionInbound,
			expectedErrContains: "one asset coin",
		},
		{
			name:                "empty payment leg yields a zero payment coin for a zero-priced settlement",
			sourceAmount:        sdk.NewCoins(sdk.NewInt64Coin("rwa", 10)),
			targetAmount:        sdk.NewCoins(),
			direction:           types.AssetDirectionInbound,
			expectedAssetCoin:   sdk.NewInt64Coin("rwa", 10),
			expectedPaymentCoin: sdk.NewInt64Coin("pay", 0),
		},
		{
			name:                "payment leg with multiple coins is rejected",
			sourceAmount:        sdk.NewCoins(sdk.NewInt64Coin("rwa", 10)),
			targetAmount:        sdk.NewCoins(sdk.NewInt64Coin("pay", 5), sdk.NewInt64Coin("othercoin", 5)),
			direction:           types.AssetDirectionInbound,
			expectedErrContains: "at most one payment coin",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			payment := &exchange.Payment{SourceAmount: tc.sourceAmount, TargetAmount: tc.targetAmount}

			assetCoin, paymentCoin, err := s.k.TestAccessor_settlementLegCoins(s.T(), payment, tc.direction, "pay")

			if tc.expectedErrContains != "" {
				s.Require().ErrorContains(err, tc.expectedErrContains, "settlementLegCoins should reject source=%s target=%s", tc.sourceAmount, tc.targetAmount)
				return
			}

			s.Require().NoError(err, "settlementLegCoins should resolve source=%s target=%s direction=%s", tc.sourceAmount, tc.targetAmount, tc.direction)
			s.Assert().Equal(tc.expectedAssetCoin, assetCoin, "asset coin mismatch for direction %s", tc.direction)
			s.Assert().Equal(tc.expectedPaymentCoin, paymentCoin, "payment coin mismatch for direction %s", tc.direction)
		})
	}
}

func (s *TestSuite) TestKeeper_StageAndReturnPrincipal() {
	underlying, share := "under", "vshare"
	restricted, free := "restrictedrwa", "freerwa"

	tests := []struct {
		name    string
		deposit bool // true exercises returnToPrincipal (vault -> principal); false stageFromPrincipal (principal -> vault)
		// seed funds the endpoints for the case and returns the amount to transfer.
		seed  func(vault *types.VaultAccount) sdk.Coins
		denom string
		moved sdkmath.Int
	}{
		{
			name:    "zero amount is a no-op and moves no funds",
			deposit: true,
			seed: func(vault *types.VaultAccount) sdk.Coins {
				s.requireSimpleMarker(free)
				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vault.GetAddress(), sdk.NewCoins(sdk.NewInt64Coin(free, 10))), "failed to fund vault with %s", free)
				return sdk.NewCoins()
			},
			denom: free,
			moved: sdkmath.NewInt(0),
		},
		{
			name:    "unrestricted coins return from vault to principal",
			deposit: true,
			seed: func(vault *types.VaultAccount) sdk.Coins {
				s.requireSimpleMarker(free)
				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, vault.GetAddress(), sdk.NewCoins(sdk.NewInt64Coin(free, 10))), "failed to fund vault with %s", free)
				return sdk.NewCoins(sdk.NewInt64Coin(free, 10))
			},
			denom: free,
			moved: sdkmath.NewInt(10),
		},
		{
			name:    "restricted coins return into the principal marker via bypass",
			deposit: true,
			seed: func(vault *types.VaultAccount) sdk.Coins {
				s.requireRestrictedMarker(restricted)
				s.Require().NoError(s.simApp.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, vault.GetAddress(), restricted, sdk.NewCoins(sdk.NewInt64Coin(restricted, 10))), "failed to fund vault with %s", restricted)
				return sdk.NewCoins(sdk.NewInt64Coin(restricted, 10))
			},
			denom: restricted,
			moved: sdkmath.NewInt(10),
		},
		{
			name:    "restricted coins stage out of the principal marker via bypass",
			deposit: false,
			seed: func(vault *types.VaultAccount) sdk.Coins {
				s.requireRestrictedMarker(restricted)
				s.Require().NoError(s.simApp.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, vault.GetAddress(), restricted, sdk.NewCoins(sdk.NewInt64Coin(restricted, 10))), "failed to fund vault with %s", restricted)
				s.Require().NoError(s.simApp.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), vault.GetAddress(), vault.PrincipalMarkerAddress(), sdk.NewCoins(sdk.NewInt64Coin(restricted, 10))), "failed to seed principal with %s", restricted)
				return sdk.NewCoins(sdk.NewInt64Coin(restricted, 10))
			},
			denom: restricted,
			moved: sdkmath.NewInt(10),
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			origCtx := s.ctx
			defer func() { s.ctx = origCtx }()
			s.ctx, _ = s.ctx.CacheContext()

			vault, principalAddr := s.setupAssetSettlementVault(underlying, share)
			vaultAddr := vault.GetAddress()

			from, to := principalAddr, vaultAddr
			if tc.deposit {
				from, to = vaultAddr, principalAddr
			}

			amt := tc.seed(vault)
			fromBefore := s.simApp.BankKeeper.GetBalance(s.ctx, from, tc.denom).Amount
			toBefore := s.simApp.BankKeeper.GetBalance(s.ctx, to, tc.denom).Amount

			var err error
			if tc.deposit {
				err = s.k.TestAccessor_returnToPrincipal(s.T(), s.ctx, vault, amt)
			} else {
				err = s.k.TestAccessor_stageFromPrincipal(s.T(), s.ctx, vault, amt)
			}
			s.Require().NoError(err, "transfer should succeed for case %q", tc.name)

			s.assertBalance(from, tc.denom, fromBefore.Sub(tc.moved))
			s.assertBalance(to, tc.denom, toBefore.Add(tc.moved))
		})
	}
}

func (s *TestSuite) TestKeeper_ReturnToPrincipal_BypassIsLoadBearing() {
	underlying, share, restricted := "under", "vshare", "restrictedrwa"

	origCtx := s.ctx
	defer func() { s.ctx = origCtx }()
	s.ctx, _ = s.ctx.CacheContext()

	vault, principalAddr := s.setupAssetSettlementVault(underlying, share)
	vaultAddr := vault.GetAddress()

	s.requireRestrictedMarker(restricted)
	s.Require().NoError(s.simApp.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, vaultAddr, restricted, sdk.NewCoins(sdk.NewInt64Coin(restricted, 10))), "failed to fund vault with %s", restricted)

	amt := sdk.NewCoins(sdk.NewInt64Coin(restricted, 10))

	// A plain send (no bypass) into the principal marker is blocked by the marker send
	// restriction. Run it in a throwaway cache so its partial debit cannot affect the real send.
	plainCtx, _ := s.ctx.CacheContext()
	plainErr := s.simApp.BankKeeper.SendCoins(plainCtx, vaultAddr, principalAddr, amt)
	s.Require().Error(plainErr, "a plain send of a restricted denom into the principal marker should be blocked by the marker send restriction")

	s.Require().NoError(s.k.TestAccessor_returnToPrincipal(s.T(), s.ctx, vault, amt), "returnToPrincipal should bypass the marker restriction and move the funds")
	s.assertBalance(vaultAddr, restricted, sdkmath.NewInt(0))
	s.assertBalance(principalAddr, restricted, sdkmath.NewInt(10))
}
