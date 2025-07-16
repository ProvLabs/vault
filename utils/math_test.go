package utils_test

import (
	"fmt"
	"math"
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
	}{
		{
			name:        "first deposit (1:1 mapping)",
			assets:      sdkmath.NewInt(100),
			totalAssets: sdkmath.NewInt(0),
			totalShares: sdkmath.NewInt(0),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(100)),
			expectErr:   false,
		},
		{
			name:        "proportional minting",
			assets:      sdkmath.NewInt(50),
			totalAssets: sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(200),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(100)),
			expectErr:   false,
		},
		{
			name:        "rounding down",
			assets:      sdkmath.NewInt(1),
			totalAssets: sdkmath.NewInt(3),
			totalShares: sdkmath.NewInt(10),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(3)),
			expectErr:   false,
		},
		{
			name:        "zero asset input",
			assets:      sdkmath.NewInt(0),
			totalAssets: sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(1000),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(0)),
			expectErr:   false,
		},
		{
			name:        "zero totalAssets and totalShares, zero input",
			assets:      sdkmath.NewInt(0),
			totalAssets: sdkmath.NewInt(0),
			totalShares: sdkmath.NewInt(0),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(0)),
			expectErr:   false,
		},
		{
			name:        "extremely large values (1:1)",
			assets:      sdkmath.NewIntFromUint64(math.MaxUint64),
			totalAssets: sdkmath.NewIntFromUint64(math.MaxUint64),
			totalShares: sdkmath.NewIntFromUint64(math.MaxUint64),
			expected:    sdk.NewCoin(denom, sdkmath.NewIntFromUint64(math.MaxUint64)),
			expectErr:   false,
		},
		{
			name:        "very small asset compared to large totalAssets",
			assets:      sdkmath.NewInt(1),
			totalAssets: sdkmath.NewInt(1_000_000_000),
			totalShares: sdkmath.NewInt(1_000_000_000),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(1)),
			expectErr:   false,
		},
		{
			name:        "assets > totalAssets",
			assets:      sdkmath.NewInt(2000),
			totalAssets: sdkmath.NewInt(1000),
			totalShares: sdkmath.NewInt(1000),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(2000)),
			expectErr:   false,
		},
		{
			name:        "assets < totalAssets",
			assets:      sdkmath.NewInt(100),
			totalAssets: sdkmath.NewInt(1000),
			totalShares: sdkmath.NewInt(1000),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(100)),
			expectErr:   false,
		},
		{
			name:        "negative asset input",
			assets:      sdkmath.NewInt(-100),
			totalAssets: sdkmath.NewInt(1000),
			totalShares: sdkmath.NewInt(1000),
			expectErr:   true,
		},
		{
			name:        "negative totalAssets",
			assets:      sdkmath.NewInt(100),
			totalAssets: sdkmath.NewInt(-1000),
			totalShares: sdkmath.NewInt(1000),
			expectErr:   true,
		},
		{
			name:        "negative totalShares",
			assets:      sdkmath.NewInt(100),
			totalAssets: sdkmath.NewInt(1000),
			totalShares: sdkmath.NewInt(-1000),
			expectErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			coin, err := utils.CalculateSharesFromAssets(tc.assets, tc.totalAssets, tc.totalShares, denom)

			if tc.expectErr {
				require.Error(t, err)
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
	}{
		{
			name:        "normal proportional case",
			shares:      sdkmath.NewInt(50),
			totalShares: sdkmath.NewInt(100),
			totalAssets: sdkmath.NewInt(1000),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(500)),
			expectErr:   false,
		},
		{
			name:        "zero shares input",
			shares:      sdkmath.NewInt(0),
			totalShares: sdkmath.NewInt(1000),
			totalAssets: sdkmath.NewInt(5000),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(0)),
			expectErr:   false,
		},
		{
			name:        "zero total shares returns 0 assets",
			shares:      sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(0),
			totalAssets: sdkmath.NewInt(1000),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(0)),
			expectErr:   false,
		},
		{
			name:        "rounding down edge case",
			shares:      sdkmath.NewInt(1),
			totalShares: sdkmath.NewInt(3),
			totalAssets: sdkmath.NewInt(10),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(3)),
			expectErr:   false,
		},
		{
			name:        "all values zero",
			shares:      sdkmath.NewInt(0),
			totalShares: sdkmath.NewInt(0),
			totalAssets: sdkmath.NewInt(0),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(0)),
			expectErr:   false,
		},
		{
			name:        "extremely large values (1:1)",
			shares:      sdkmath.NewIntFromUint64(math.MaxUint64),
			totalShares: sdkmath.NewIntFromUint64(math.MaxUint64),
			totalAssets: sdkmath.NewIntFromUint64(math.MaxUint64),
			expected:    sdk.NewCoin(denom, sdkmath.NewIntFromUint64(math.MaxUint64)),
			expectErr:   false,
		},
		{
			name:        "very small shares compared to large totalShares",
			shares:      sdkmath.NewInt(1),
			totalShares: sdkmath.NewInt(1_000_000_000),
			totalAssets: sdkmath.NewInt(1_000_000_000),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(1)),
			expectErr:   false,
		},
		{
			name:        "shares > totalShares",
			shares:      sdkmath.NewInt(2000),
			totalShares: sdkmath.NewInt(1000),
			totalAssets: sdkmath.NewInt(5000),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(10000)),
			expectErr:   false,
		},
		{
			name:        "totalAssets smaller than totalShares",
			shares:      sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(1000),
			totalAssets: sdkmath.NewInt(100),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(10)),
			expectErr:   false,
		},
		{
			name:        "negative shares",
			shares:      sdkmath.NewInt(-100),
			totalShares: sdkmath.NewInt(1000),
			totalAssets: sdkmath.NewInt(1000),
			expectErr:   true,
		},
		{
			name:        "negative totalAssets",
			shares:      sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(1000),
			totalAssets: sdkmath.NewInt(-1000),
			expectErr:   true,
		},
		{
			name:        "negative totalShares",
			shares:      sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(-1000),
			totalAssets: sdkmath.NewInt(1000),
			expectErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := utils.CalculateAssetsFromShares(tc.shares, tc.totalShares, tc.totalAssets, denom)

			if tc.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expected, result, fmt.Sprintf("expected %v, got %v", tc.expected, result))
		})
	}
}
