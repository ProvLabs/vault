package utils_test

import (
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/utils"
	"github.com/stretchr/testify/require"
)

func TestCalculateSharesFromAssets(t *testing.T) {
	shareDenom := "vaultshare"

	tests := []struct {
		name        string
		assets      sdkmath.Int
		totalAssets sdkmath.Int
		totalShares sdkmath.Int
		expected    sdk.Coin
		expectErr   bool
		errMsg      string
	}{
		{
			name:        "first deposit mints assets times ShareScalar",
			assets:      sdkmath.NewInt(100),
			totalAssets: sdkmath.NewInt(0),
			totalShares: sdkmath.NewInt(0),
			expected:    sdk.NewCoin(shareDenom, sdkmath.NewInt(100_000_000)),
		},
		{
			name:        "proportional mint with virtual offsets",
			assets:      sdkmath.NewInt(50),
			totalAssets: sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(200),
			expected:    sdk.NewCoin(shareDenom, sdkmath.NewInt(495_148)),
		},
		{
			name:        "small deposit precision and offsets",
			assets:      sdkmath.NewInt(1),
			totalAssets: sdkmath.NewInt(3),
			totalShares: sdkmath.NewInt(10),
			expected:    sdk.NewCoin(shareDenom, sdkmath.NewInt(250_002)),
		},
		{
			name:        "reject negative assets",
			assets:      sdkmath.NewInt(-100),
			totalAssets: sdkmath.NewInt(1000),
			totalShares: sdkmath.NewInt(1000),
			expectErr:   true,
			errMsg:      "invalid input: negative values not allowed",
		},
		{
			name:        "reject negative total assets",
			assets:      sdkmath.NewInt(100),
			totalAssets: sdkmath.NewInt(-1000),
			totalShares: sdkmath.NewInt(1000),
			expectErr:   true,
			errMsg:      "invalid input: negative values not allowed",
		},
		{
			name:        "reject negative total shares",
			assets:      sdkmath.NewInt(100),
			totalAssets: sdkmath.NewInt(1000),
			totalShares: sdkmath.NewInt(-1000),
			expectErr:   true,
			errMsg:      "invalid input: negative values not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			coin, err := utils.CalculateSharesFromAssets(tc.assets, tc.totalAssets, tc.totalShares, shareDenom)
			if tc.expectErr {
				require.Error(t, err, "expected error for case: %s", tc.name)
				require.EqualError(t, err, tc.errMsg)
			} else {
				require.NoError(t, err, "unexpected error for case: %s", tc.name)
				require.Equal(t, tc.expected, coin, "unexpected shares for assets=%s totalAssets=%s totalShares=%s", tc.assets, tc.totalAssets, tc.totalShares)
			}
		})
	}
}

func TestCalculateAssetsFromShares(t *testing.T) {
	assetDenom := "asset"

	tests := []struct {
		name        string
		shares      sdkmath.Int
		totalShares sdkmath.Int
		totalAssets sdkmath.Int
		expected    sdk.Coin
		expectErr   bool
		errMsg      string
	}{
		{
			name:        "proportional redeem with virtual offsets",
			shares:      sdkmath.NewInt(50 * 1_000_000),
			totalShares: sdkmath.NewInt(100 * 1_000_000),
			totalAssets: sdkmath.NewInt(1_000_000_000),
			expected:    sdk.NewCoin(assetDenom, sdkmath.NewInt(495_049_505)),
		},
		{
			name:        "zero shares input returns zero assets",
			shares:      sdkmath.NewInt(0),
			totalShares: sdkmath.NewInt(1000),
			totalAssets: sdkmath.NewInt(5_000_000),
			expected:    sdk.NewCoin(assetDenom, sdkmath.NewInt(0)),
		},
		{
			name:        "virtual shares enable redeem when totalShares is zero",
			shares:      sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(0),
			totalAssets: sdkmath.NewInt(1_000_000),
			expected:    sdk.NewCoin(assetDenom, sdkmath.NewInt(100)),
		},
		{
			name:        "reject negative shares",
			shares:      sdkmath.NewInt(-100),
			totalShares: sdkmath.NewInt(1000),
			totalAssets: sdkmath.NewInt(1000),
			expectErr:   true,
			errMsg:      "invalid input: negative values not allowed",
		},
		{
			name:        "reject negative total assets",
			shares:      sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(1000),
			totalAssets: sdkmath.NewInt(-1000),
			expectErr:   true,
			errMsg:      "invalid input: negative values not allowed",
		},
		{
			name:        "reject negative total shares",
			shares:      sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(-1000),
			totalAssets: sdkmath.NewInt(1000),
			expectErr:   true,
			errMsg:      "invalid input: negative values not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := utils.CalculateAssetsFromShares(tc.shares, tc.totalShares, tc.totalAssets, assetDenom)
			if tc.expectErr {
				require.Error(t, err, "expected error for case: %s", tc.name)
				require.EqualError(t, err, tc.errMsg)
			} else {
				require.NoError(t, err, "unexpected error for case: %s", tc.name)
				require.Equal(t, tc.expected, result, fmt.Sprintf("unexpected assets for shares=%s totalShares=%s totalAssets=%s", tc.shares, tc.totalShares, tc.totalAssets))
			}
		})
	}
}
