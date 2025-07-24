package utils_test

import (
	"fmt"
	"math"
	"testing"
	"time"

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
	t.Skip("Skipping test, used to explore edge cases")
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
