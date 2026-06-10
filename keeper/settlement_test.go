package keeper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/provenance-io/provenance/x/exchange"
	"github.com/provlabs/vault/types"
)

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
			expectedErrContains: "exactly one coin",
		},
		{
			name:                "asset leg with multiple coins is rejected",
			sourceAmount:        sdk.NewCoins(sdk.NewInt64Coin("rwa", 10), sdk.NewInt64Coin("othercoin", 5)),
			targetAmount:        sdk.NewCoins(sdk.NewInt64Coin("pay", 5)),
			direction:           types.AssetDirectionInbound,
			expectedErrContains: "exactly one coin",
		},
		{
			name:                "empty payment leg is rejected",
			sourceAmount:        sdk.NewCoins(sdk.NewInt64Coin("rwa", 10)),
			targetAmount:        sdk.NewCoins(),
			direction:           types.AssetDirectionInbound,
			expectedErrContains: "exactly one coin",
		},
		{
			name:                "payment leg with multiple coins is rejected",
			sourceAmount:        sdk.NewCoins(sdk.NewInt64Coin("rwa", 10)),
			targetAmount:        sdk.NewCoins(sdk.NewInt64Coin("pay", 5), sdk.NewInt64Coin("othercoin", 5)),
			direction:           types.AssetDirectionInbound,
			expectedErrContains: "exactly one coin",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			payment := &exchange.Payment{SourceAmount: tc.sourceAmount, TargetAmount: tc.targetAmount}

			assetCoin, paymentCoin, err := s.k.TestAccessor_settlementLegCoins(s.T(), payment, tc.direction)

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
