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
	denom := "vaultshare"

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
			name:        "first deposit (1:1 mapping)",
			assets:      sdkmath.NewInt(100),
			totalAssets: sdkmath.NewInt(0),
			totalShares: sdkmath.NewInt(0),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(100)),
		},
		{
			name:        "proportional minting",
			assets:      sdkmath.NewInt(50),
			totalAssets: sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(200),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(100)),
		},
		{
			name:        "rounding down",
			assets:      sdkmath.NewInt(1),
			totalAssets: sdkmath.NewInt(3),
			totalShares: sdkmath.NewInt(10),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(3)),
		},
		{
			name:        "negative asset input",
			assets:      sdkmath.NewInt(-100),
			totalAssets: sdkmath.NewInt(1000),
			totalShares: sdkmath.NewInt(1000),
			expectErr:   true,
			errMsg:      "invalid input: negative values not allowed",
		},
		{
			name:        "negative totalAssets",
			assets:      sdkmath.NewInt(100),
			totalAssets: sdkmath.NewInt(-1000),
			totalShares: sdkmath.NewInt(1000),
			expectErr:   true,
			errMsg:      "invalid input: negative values not allowed",
		},
		{
			name:        "negative totalShares",
			assets:      sdkmath.NewInt(100),
			totalAssets: sdkmath.NewInt(1000),
			totalShares: sdkmath.NewInt(-1000),
			expectErr:   true,
			errMsg:      "invalid input: negative values not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			coin, err := utils.CalculateSharesFromAssets(tc.assets, tc.totalAssets, tc.totalShares, denom)
			if tc.expectErr {
				require.Error(t, err)
				require.EqualError(t, err, tc.errMsg)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, coin)
			}
		})
	}
}

func TestCalculateAssetsFromShares(t *testing.T) {
	denom := "asset"

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
			name:        "normal proportional case",
			shares:      sdkmath.NewInt(50),
			totalShares: sdkmath.NewInt(100),
			totalAssets: sdkmath.NewInt(1000),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(500)),
		},
		{
			name:        "zero shares input",
			shares:      sdkmath.NewInt(0),
			totalShares: sdkmath.NewInt(1000),
			totalAssets: sdkmath.NewInt(5000),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(0)),
		},
		{
			name:        "zero total shares returns 0 assets",
			shares:      sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(0),
			totalAssets: sdkmath.NewInt(1000),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(0)),
		},
		{
			name:        "negative shares",
			shares:      sdkmath.NewInt(-100),
			totalShares: sdkmath.NewInt(1000),
			totalAssets: sdkmath.NewInt(1000),
			expectErr:   true,
			errMsg:      "invalid input: negative values not allowed",
		},
		{
			name:        "negative totalAssets",
			shares:      sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(1000),
			totalAssets: sdkmath.NewInt(-1000),
			expectErr:   true,
			errMsg:      "invalid input: negative values not allowed",
		},
		{
			name:        "negative totalShares",
			shares:      sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(-1000),
			totalAssets: sdkmath.NewInt(1000),
			expectErr:   true,
			errMsg:      "invalid input: negative values not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := utils.CalculateAssetsFromShares(tc.shares, tc.totalShares, tc.totalAssets, denom)
			if tc.expectErr {
				require.Error(t, err)
				require.EqualError(t, err, tc.errMsg)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, result, fmt.Sprintf("expected %v, got %v", tc.expected, result))
			}
		})
	}
}
