package utils_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/utils"
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
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(1)), // (1 * 1e9 / 1e9)
			expectErr:   false,
		},
		{
			name:        "assets > totalAssets",
			assets:      sdkmath.NewInt(2000),
			totalAssets: sdkmath.NewInt(1000),
			totalShares: sdkmath.NewInt(1000),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(2000)), // (2000 * 1000 / 1000)
			expectErr:   false,
		},
		{
			name:        "assets < totalAssets",
			assets:      sdkmath.NewInt(100),
			totalAssets: sdkmath.NewInt(1000),
			totalShares: sdkmath.NewInt(1000),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(100)), // (100 * 1000 / 1000)
			expectErr:   false,
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
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(500)), // (50 * 1000) / 100 = 500
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
			name:        "zero total shares (error)",
			shares:      sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(0),
			totalAssets: sdkmath.NewInt(1000),
			expected:    sdk.Coin{},
			expectErr:   true,
		},
		{
			name:        "rounding down edge case",
			shares:      sdkmath.NewInt(1),
			totalShares: sdkmath.NewInt(3),
			totalAssets: sdkmath.NewInt(10),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(3)), // truncates (1*10)/3 = 3.333...
			expectErr:   false,
		},
		{
			name:        "all values zero (error)",
			shares:      sdkmath.NewInt(0),
			totalShares: sdkmath.NewInt(0),
			totalAssets: sdkmath.NewInt(0),
			expected:    sdk.Coin{},
			expectErr:   true,
		},
		{
			name:        "extremely large values (1:1)",
			shares:      sdkmath.NewIntFromUint64(math.MaxUint64),
			totalShares: sdkmath.NewIntFromUint64(math.MaxUint64),
			totalAssets: sdkmath.NewIntFromUint64(math.MaxUint64),
			expected:    sdk.NewCoin(denom, sdkmath.NewIntFromUint64(math.MaxUint64)), // (Max * Max) / Max = Max
			expectErr:   false,
		},
		{
			name:        "very small shares compared to large totalShares",
			shares:      sdkmath.NewInt(1),
			totalShares: sdkmath.NewInt(1_000_000_000),
			totalAssets: sdkmath.NewInt(1_000_000_000),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(1)), // 1:1, very small unit
			expectErr:   false,
		},
		{
			name:        "shares > totalShares",
			shares:      sdkmath.NewInt(2000),
			totalShares: sdkmath.NewInt(1000),
			totalAssets: sdkmath.NewInt(5000),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(10000)), // (2000 * 5000) / 1000
			expectErr:   false,
		},
		{
			name:        "totalAssets smaller than totalShares",
			shares:      sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(1000),
			totalAssets: sdkmath.NewInt(100),
			expected:    sdk.NewCoin(denom, sdkmath.NewInt(10)), // (100 * 100) / 1000
			expectErr:   false,
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

func TestExpDec(t *testing.T) {
	tests := []struct {
		name      string
		input     sdkmath.LegacyDec
		terms     int
		expected  float64 // Expected e^x using float64 for comparison
		tolerance float64
	}{
		{
			name:      "e^0 = 1",
			input:     sdkmath.LegacyZeroDec(),
			terms:     17,
			expected:  1.0,
			tolerance: 1e-17,
		},
		{
			name:      "e^1 ~= 2.71828",
			input:     sdkmath.LegacyNewDec(1),
			terms:     17,
			expected:  math.E,
			tolerance: 1e-17,
		},
		{
			name:      "e^-1 ~= 0.36788",
			input:     sdkmath.LegacyNewDec(-1),
			terms:     17,
			expected:  math.Exp(-1),
			tolerance: 1e-6,
		},
		{
			name:      "e^2 ~= 7.38906",
			input:     sdkmath.LegacyNewDec(2),
			terms:     17,
			expected:  math.Exp(2),
			tolerance: 1e-5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := utils.ExpDec(tc.input, tc.terms)
			got, _ := result.Float64()
			diff := math.Abs(got - tc.expected)
			require.LessOrEqual(t, diff, tc.tolerance, "expected %f, got %f", tc.expected, got)
		})
	}
}

func TestExpDecConvergenceToE(t *testing.T) {
	target := math.E
	tolerance := 1e-17
	maxTerms := 100
	x := sdkmath.LegacyNewDec(1)

	var closest float64
	var matched bool
	var termsUsed int

	for terms := 1; terms <= maxTerms; terms++ {
		start := time.Now()

		result := utils.ExpDec(x, terms)

		elapsed := time.Since(start).Microseconds()

		f, err := result.Float64()
		require.NoError(t, err)

		diff := math.Abs(f - target)
		t.Logf("terms=%d took %d µs, result=%.20f, diff=%.20f", terms, elapsed, f, diff)

		if diff < tolerance {
			matched = true
			closest = f
			termsUsed = terms
			break
		}
	}

	require.True(t, matched, "did not converge to math.E within %g tolerance", tolerance)
	t.Logf("Matched math.E (%.20f) with %d terms: %.20f", target, termsUsed, closest)

}
