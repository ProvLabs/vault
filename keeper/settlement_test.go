package keeper_test

import (
	"math/big"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/provenance-io/provenance/x/exchange"
	"github.com/provlabs/vault/types"
)

func (s *TestSuite) TestKeeper_ApplySettlementNAV() {
	underlying := "under"
	share := "vshare"
	paymentDenom := "pay"
	asset := "rwacoin"

	tests := []struct {
		name                string
		registerAssetMarker bool
		fundPrincipal       sdk.Coins
		assetAmount         sdkmath.Int
		direction           string
		expectedErrContains string
		expectNavRemoved    bool
	}{
		{
			name:                "inbound settlement upserts the NAV and keeps it even when the principal holds none of the asset",
			registerAssetMarker: true,
			assetAmount:         sdkmath.NewInt(10),
			direction:           types.AssetDirectionInbound,
		},
		{
			name:                "outbound settlement with a remaining principal balance keeps the NAV entry",
			registerAssetMarker: true,
			fundPrincipal:       sdk.NewCoins(sdk.NewInt64Coin(asset, 5)),
			assetAmount:         sdkmath.NewInt(10),
			direction:           types.AssetDirectionOutbound,
		},
		{
			name:                "outbound settlement with a drained principal removes the NAV entry",
			registerAssetMarker: true,
			assetAmount:         sdkmath.NewInt(10),
			direction:           types.AssetDirectionOutbound,
			expectNavRemoved:    true,
		},
		{
			name:                "asset denom without a registered marker fails the upsert",
			registerAssetMarker: false,
			assetAmount:         sdkmath.NewInt(10),
			direction:           types.AssetDirectionInbound,
			expectedErrContains: "failed to update internal NAV from settlement",
		},
		{
			name:                "asset amount that overflows the marker NAV volume fails the publish",
			registerAssetMarker: true,
			assetAmount:         sdkmath.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 70)),
			direction:           types.AssetDirectionInbound,
			expectedErrContains: "failed to publish settlement NAV to marker",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			origCtx := s.ctx
			defer func() { s.ctx = origCtx }()
			s.ctx, _ = s.ctx.CacheContext()

			vault, principalAddr := s.setupAssetSettlementVault(underlying, share, paymentDenom)
			vaultAddr := vault.GetAddress()
			if tc.registerAssetMarker {
				s.requireSimpleMarker(asset)
			}
			if !tc.fundPrincipal.IsZero() {
				s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, principalAddr, tc.fundPrincipal), "failed to fund principal with %s", tc.fundPrincipal)
			}

			assetCoin := sdk.NewCoin(asset, tc.assetAmount)
			paymentCoin := sdk.NewInt64Coin(paymentDenom, 5)

			s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
			err := s.k.TestAccessor_applySettlementNAV(s.T(), s.ctx, vault, assetCoin, paymentCoin, tc.direction, s.adminAddr.String())

			var removedEvents []sdk.Event
			for _, ev := range s.ctx.EventManager().Events() {
				if ev.Type == "provlabs.vault.v1.EventNAVRemoved" {
					removedEvents = append(removedEvents, ev)
				}
			}

			if tc.expectedErrContains != "" {
				s.Require().ErrorContains(err, tc.expectedErrContains, "applySettlementNAV should fail for case %q", tc.name)
				return
			}
			s.Require().NoError(err, "applySettlementNAV should succeed for asset %s payment %s direction %s", assetCoin, paymentCoin, tc.direction)

			if tc.expectNavRemoved {
				_, err := s.k.GetVaultNAV(s.ctx, vaultAddr, asset)
				s.Assert().ErrorIs(err, collections.ErrNotFound, "NAV entry for %s should be removed after draining the principal", asset)
				s.Assert().Len(removedEvents, 1, "draining settlement should emit exactly one EventNAVRemoved")
				return
			}

			s.Assert().Empty(removedEvents, "non-draining settlement should not emit EventNAVRemoved for case %q", tc.name)
			stored, err := s.k.GetVaultNAV(s.ctx, vaultAddr, asset)
			s.Require().NoError(err, "NAV entry for %s should exist after settlement", asset)
			s.Assert().Equal(paymentCoin, stored.Price, "stored NAV price should be the payment coin for case %q", tc.name)
			s.Assert().Equal(tc.assetAmount, stored.Volume, "stored NAV volume should be the asset amount for case %q", tc.name)
			s.Assert().Equal(vaultAddr.String(), stored.Source, "stored NAV source should be the vault address for case %q", tc.name)
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
