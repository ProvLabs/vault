package utils_test

import (
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/utils"
	"github.com/stretchr/testify/require"
)

func TestCalculateAssetsFromShares(t *testing.T) {
	assetDenom := "asset"

	// This test name is retained for continuity. Internally it now routes to the
	// single-floor redeem path (CalculateRedeemProRataFraction) with price 1:1. The intent
	// remains identical: “given shares and totals, how many assets do I get back?”
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
			result, err := utils.CalculateRedeemProRataFraction(
				tc.shares,
				tc.totalShares,
				tc.totalAssets,
				sdkmath.NewInt(1), // price numerator (underlying->underlying)
				sdkmath.NewInt(1), // price denominator
				assetDenom,
			)
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

func TestCalculateSharesProRataFraction(t *testing.T) {
	shareDenom := "vaultshare"

	tests := []struct {
		name            string
		amountNum       sdkmath.Int
		amountDen       sdkmath.Int
		totalAssets     sdkmath.Int
		totalShares     sdkmath.Int
		expected        sdk.Coin
		expectErr       bool
		expectedErrText string
	}{
		{
			name:        "first deposit mints amount * ShareScalar (price 1:1)",
			amountNum:   sdkmath.NewInt(100),
			amountDen:   sdkmath.NewInt(1),
			totalAssets: sdkmath.NewInt(0),
			totalShares: sdkmath.NewInt(0),
			expected:    sdk.NewCoin(shareDenom, sdkmath.NewInt(100_000_000)),
		},
		{
			name:        "assets zero but shares non-zero uses pro-rata path",
			amountNum:   sdkmath.NewInt(1),
			amountDen:   sdkmath.NewInt(1),
			totalAssets: sdkmath.NewInt(0),
			totalShares: sdkmath.NewInt(1),
			expected:    sdk.NewCoin(shareDenom, sdkmath.NewInt(1_000_001)),
		},
		{
			name:        "proportional mint with virtual offsets (price 1:1)",
			amountNum:   sdkmath.NewInt(50),
			amountDen:   sdkmath.NewInt(1),
			totalAssets: sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(200),
			expected:    sdk.NewCoin(shareDenom, sdkmath.NewInt(495_148)),
		},
		{
			name:        "small deposit precision and offsets (price 1:1)",
			amountNum:   sdkmath.NewInt(1),
			amountDen:   sdkmath.NewInt(1),
			totalAssets: sdkmath.NewInt(3),
			totalShares: sdkmath.NewInt(10),
			expected:    sdk.NewCoin(shareDenom, sdkmath.NewInt(250_002)),
		},
		{
			name:        "priced deposit 3/2 into non-empty vault",
			amountNum:   sdkmath.NewInt(6),
			amountDen:   sdkmath.NewInt(2),
			totalAssets: sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(200),
			expected:    sdk.NewCoin(shareDenom, sdkmath.NewInt(29_708)),
		},
		{
			name:            "reject negative amount numerator",
			amountNum:       sdkmath.NewInt(-1),
			amountDen:       sdkmath.NewInt(1),
			totalAssets:     sdkmath.NewInt(0),
			totalShares:     sdkmath.NewInt(0),
			expectErr:       true,
			expectedErrText: "invalid input: negative values not allowed",
		},
		{
			name:            "reject negative denominator",
			amountNum:       sdkmath.NewInt(1),
			amountDen:       sdkmath.NewInt(-1),
			totalAssets:     sdkmath.NewInt(0),
			totalShares:     sdkmath.NewInt(0),
			expectErr:       true,
			expectedErrText: "invalid input: negative values not allowed",
		},
		{
			name:            "reject zero denominator",
			amountNum:       sdkmath.NewInt(1),
			amountDen:       sdkmath.NewInt(0),
			totalAssets:     sdkmath.NewInt(0),
			totalShares:     sdkmath.NewInt(0),
			expectErr:       true,
			expectedErrText: "invalid input: zero denominator",
		},
		{
			name:            "reject negative totals",
			amountNum:       sdkmath.NewInt(1),
			amountDen:       sdkmath.NewInt(1),
			totalAssets:     sdkmath.NewInt(-1),
			totalShares:     sdkmath.NewInt(0),
			expectErr:       true,
			expectedErrText: "invalid input: negative values not allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := utils.CalculateSharesProRataFraction(tc.amountNum, tc.amountDen, tc.totalAssets, tc.totalShares, shareDenom)

			if tc.expectErr {
				require.Error(t, err, "expected error for case: %s", tc.name)
				require.EqualError(t, err, tc.expectedErrText, "unexpected error text for case: %s", tc.name)
				return
			}

			require.NoError(t, err, "unexpected error for case: %s", tc.name)
			require.Equal(t, tc.expected, got, fmt.Sprintf("unexpected shares for amount=%s/%s totalAssets=%s totalShares=%s", tc.amountNum, tc.amountDen, tc.totalAssets, tc.totalShares))
		})
	}
}

// TestSmallFirstSwapInThenHugeSwapInThenSwapOut verifies that a very small
// initial swap in is protected from dilution when followed by a massive swap in
// from another participant.
//
// Without a virtual share offset, the first swapper’s shares could be worth far
// less after the large swap in. This test confirms that:
//   - The first swapper’s shares are scaled to maintain fair value.
//   - Swapping out those shares after the large swap in returns at least the
//     original swap-in amount.
//   - The swap-out never exceeds the vault’s total assets.
func TestSmallFirstSwapInThenHugeSwapInThenSwapOut(t *testing.T) {
	firstIn := sdkmath.NewInt(1)
	totalAssets := sdkmath.ZeroInt()
	totalShares := sdkmath.ZeroInt()

	shareDenom := "shares"
	firstShares, err := utils.CalculateSharesProRataFraction(firstIn, sdkmath.NewInt(1), totalAssets, totalShares, shareDenom)
	require.NoError(t, err, "first swap-in conversion should not error")
	require.Equal(t, firstIn.Mul(utils.ShareScalar), firstShares.Amount, "first swap-in should mint amount * ShareScalar")

	totalAssets = totalAssets.Add(firstIn)
	totalShares = totalShares.Add(firstShares.Amount)

	hugeIn := sdkmath.NewInt(1_000_000_000)
	// Simulate a second participant swapping in a massive amount of underlying.
	totalAssets = totalAssets.Add(hugeIn)

	assetDenom := "underlying"
	outAll, err := utils.CalculateRedeemProRataFraction(firstShares.Amount, totalShares, totalAssets, sdkmath.NewInt(1), sdkmath.NewInt(1), assetDenom)
	require.NoError(t, err, "swap-out conversion should not error")

	require.Truef(t,
		outAll.Amount.GTE(firstIn),
		"swap-out should return >= original swap-in (got=%s, want >= %s)",
		outAll.Amount, firstIn,
	)
	require.Truef(t,
		outAll.Amount.LTE(totalAssets),
		"swap-out should not exceed total vault assets (got=%s, want <= %s)",
		outAll.Amount, totalAssets,
	)
}

// TestVeryLargeInitialSwapInRoundTrip ensures that a very large first swap in
// correctly mints proportional shares and can be swapped out without loss or
// overflow.
//
// This test verifies that:
//   - Large initial swap-ins still apply precision scaling for consistency.
//   - Swapping out the minted shares returns an amount within an acceptable
//     rounding difference of the original swap-in.
//   - The vault handles extremely large initial swap-in without exceeding
//     asset totals or causing precision issues.
func TestVeryLargeInitialSwapInRoundTrip(t *testing.T) {
	largeIn := sdkmath.NewInt(1_000_000_000_000_000_000)
	totalAssets := sdkmath.ZeroInt()
	totalShares := sdkmath.ZeroInt()

	shareDenom := "shares"
	minted, err := utils.CalculateSharesProRataFraction(largeIn, sdkmath.NewInt(1), totalAssets, totalShares, shareDenom)
	require.NoError(t, err, "large swap-in conversion should not error")
	require.Equal(t, largeIn.Mul(utils.ShareScalar), minted.Amount, "minted shares should equal swap-in * ShareScalar")

	totalAssets = totalAssets.Add(largeIn)
	totalShares = totalShares.Add(minted.Amount)

	assetDenom := "underlying"
	out, err := utils.CalculateRedeemProRataFraction(minted.Amount, totalShares, totalAssets, sdkmath.NewInt(1), sdkmath.NewInt(1), assetDenom)
	require.NoError(t, err, "swap-out conversion should not error")

	price := totalAssets.Mul(utils.ShareScalar).Quo(totalShares)
	require.Equal(t, sdkmath.NewInt(1), price, "implied price should be exactly 1 asset per ShareScalar shares at large scale")

	require.Truef(t,
		out.Amount.GTE(largeIn.SubRaw(1)),
		"swap-out should be >= swap-in - 1 (got=%s, want >= %s)",
		out.Amount, largeIn.SubRaw(1),
	)
	require.Truef(t,
		out.Amount.LTE(largeIn),
		"swap-out should be <= original swap-in (got=%s, want <= %s)",
		out.Amount, largeIn,
	)
}
