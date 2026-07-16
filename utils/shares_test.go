package utils_test

import (
	"fmt"
	"math/big"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/utils"
	"github.com/stretchr/testify/require"
)

// nearMaxInt returns an sdkmath.Int equal to 2^255, the largest power of two
// that still fits within sdkmath.Int's 256-bit ceiling. Multiplying two of
// these overflows, exercising the SafeMul guards on the NAV/TVV paths.
func nearMaxInt() sdkmath.Int {
	return sdkmath.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 255))
}

func TestCalculateAssetsFromShares(t *testing.T) {
	assetDenom := "asset"

	// This test name is retained for continuity. Internally it now routes to the
	// single-floor redeem path (CalculateRedeemProRata). The intent remains
	// identical: “given shares and totals, how many assets do I get back?”
	tests := []struct {
		name        string
		shares      sdkmath.Int
		totalShares sdkmath.Int
		totalAssets sdkmath.Int
		expected    sdk.Coin
		expectErr   bool
		errMsg      string
		errContains string
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
		{
			name:        "oversized shares and assets overflow returns error instead of panicking",
			shares:      nearMaxInt(),
			totalShares: sdkmath.NewInt(1_000_000),
			totalAssets: nearMaxInt(),
			expectErr:   true,
			errContains: "integer overflow",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := utils.CalculateRedeemProRata(
				tc.shares,
				tc.totalShares,
				tc.totalAssets,
				assetDenom,
			)
			if tc.expectErr {
				require.Error(t, err, "expected error for case: %s", tc.name)
				if tc.errContains != "" {
					require.ErrorContains(t, err, tc.errContains, "error message mismatch for case: %s", tc.name)
				} else {
					require.EqualErrorf(t, err, tc.errMsg, "error message mismatch for case: %s", tc.name)
				}
			} else {
				require.NoError(t, err, "unexpected error for case: %s", tc.name)
				require.Equal(t, tc.expected, result, fmt.Sprintf("unexpected assets for shares=%s totalShares=%s totalAssets=%s", tc.shares, tc.totalShares, tc.totalAssets))
			}
		})
	}
}

func TestCalculateSharesProRata(t *testing.T) {
	shareDenom := "vaultshare"

	tests := []struct {
		name            string
		amount          sdkmath.Int
		totalAssets     sdkmath.Int
		totalShares     sdkmath.Int
		expected        sdk.Coin
		expectErr       bool
		expectedErrText string
		errContains     string
	}{
		{
			name:        "first deposit mints amount * ShareScalar",
			amount:      sdkmath.NewInt(100),
			totalAssets: sdkmath.NewInt(0),
			totalShares: sdkmath.NewInt(0),
			expected:    sdk.NewCoin(shareDenom, sdkmath.NewInt(100_000_000)),
		},
		{
			name:        "assets zero but shares non-zero uses pro-rata path",
			amount:      sdkmath.NewInt(1),
			totalAssets: sdkmath.NewInt(0),
			totalShares: sdkmath.NewInt(1),
			expected:    sdk.NewCoin(shareDenom, sdkmath.NewInt(1_000_001)),
		},
		{
			name:        "proportional mint with virtual offsets",
			amount:      sdkmath.NewInt(50),
			totalAssets: sdkmath.NewInt(100),
			totalShares: sdkmath.NewInt(200),
			expected:    sdk.NewCoin(shareDenom, sdkmath.NewInt(495_148)),
		},
		{
			name:        "small deposit precision and offsets",
			amount:      sdkmath.NewInt(1),
			totalAssets: sdkmath.NewInt(3),
			totalShares: sdkmath.NewInt(10),
			expected:    sdk.NewCoin(shareDenom, sdkmath.NewInt(250_002)),
		},
		{
			name:            "reject negative amount",
			amount:          sdkmath.NewInt(-1),
			totalAssets:     sdkmath.NewInt(0),
			totalShares:     sdkmath.NewInt(0),
			expectErr:       true,
			expectedErrText: "invalid input: negative values not allowed",
		},
		{
			name:            "reject negative totals",
			amount:          sdkmath.NewInt(1),
			totalAssets:     sdkmath.NewInt(-1),
			totalShares:     sdkmath.NewInt(0),
			expectErr:       true,
			expectedErrText: "invalid input: negative values not allowed",
		},
		{
			name:        "oversized first deposit overflows share scalar and returns error",
			amount:      nearMaxInt(),
			totalAssets: sdkmath.NewInt(0),
			totalShares: sdkmath.NewInt(0),
			expectErr:   true,
			errContains: "integer overflow",
		},
		{
			name:        "oversized deposit overflows pro-rata path and returns error",
			amount:      nearMaxInt(),
			totalAssets: sdkmath.NewInt(100),
			totalShares: nearMaxInt(),
			expectErr:   true,
			errContains: "integer overflow",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := utils.CalculateSharesProRata(tc.amount, tc.totalAssets, tc.totalShares, shareDenom)

			if tc.expectErr {
				require.Error(t, err, "expected error for case: %s", tc.name)
				if tc.errContains != "" {
					require.ErrorContains(t, err, tc.errContains, "unexpected error text for case: %s", tc.name)
					return
				}
				require.EqualErrorf(t, err, tc.expectedErrText, "unexpected error text for case: %s", tc.name)
				return
			}

			require.NoError(t, err, "unexpected error for case: %s", tc.name)
			require.Equal(t, tc.expected, got, fmt.Sprintf("unexpected shares for amount=%s totalAssets=%s totalShares=%s", tc.amount, tc.totalAssets, tc.totalShares))
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
	firstShares, err := utils.CalculateSharesProRata(firstIn, totalAssets, totalShares, shareDenom)
	require.NoErrorf(t, err, "first swap-in conversion should not error")
	require.Equalf(t, firstIn.Mul(utils.ShareScalar), firstShares.Amount, "first swap-in should mint amount * ShareScalar")

	totalAssets = totalAssets.Add(firstIn)
	totalShares = totalShares.Add(firstShares.Amount)

	hugeIn := sdkmath.NewInt(1_000_000_000)
	// Simulate a second participant swapping in a massive amount of underlying.
	totalAssets = totalAssets.Add(hugeIn)

	assetDenom := "underlying"
	outAll, err := utils.CalculateRedeemProRata(firstShares.Amount, totalShares, totalAssets, assetDenom)
	require.NoErrorf(t, err, "swap-out conversion should not error")

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
	minted, err := utils.CalculateSharesProRata(largeIn, totalAssets, totalShares, shareDenom)
	require.NoErrorf(t, err, "large swap-in conversion should not error")
	require.Equalf(t, largeIn.Mul(utils.ShareScalar), minted.Amount, "minted shares should equal swap-in * ShareScalar")

	totalAssets = totalAssets.Add(largeIn)
	totalShares = totalShares.Add(minted.Amount)

	assetDenom := "underlying"
	out, err := utils.CalculateRedeemProRata(minted.Amount, totalShares, totalAssets, assetDenom)
	require.NoErrorf(t, err, "swap-out conversion should not error")

	price := totalAssets.Mul(utils.ShareScalar).Quo(totalShares)
	require.Equalf(t, sdkmath.NewInt(1), price, "implied price should be exactly 1 asset per ShareScalar shares at large scale")

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
