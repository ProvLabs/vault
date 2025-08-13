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

// TestSmallFirstDepositThenHugeSwapInThenSwapOut ensures the first depositor
// is protected from a dilution attack when making a very small deposit
// followed by a huge swap in.
//
// Without a virtual share offset, the first depositor’s shares could be worth
// far less after the large swap in. This test checks that:
//   - The first depositor’s shares are scaled for protection.
//   - Redeeming those shares after the large swap in returns at least the
//     original swap in and no more than the vault’s total assets.
func TestSmallFirstDepositThenHugeSwapInThenSwapOut(t *testing.T) {
	firstSwapIn := sdkmath.NewInt(1)
	totalAssets := sdkmath.ZeroInt()
	totalShares := sdkmath.ZeroInt()

	shareDenom := "shares"
	firstMint, err := utils.CalculateSharesFromAssets(firstSwapIn, totalAssets, totalShares, shareDenom)
	require.NoError(t, err, "first deposit conversion should not error")
	require.Equal(t, firstSwapIn.Mul(utils.ShareScalar), firstMint.Amount, "first deposit should mint deposit * ShareScalar")

	totalAssets = totalAssets.Add(firstSwapIn)
	totalShares = totalShares.Add(firstMint.Amount)

	hugeSwapIn := sdkmath.NewInt(1_000_000_000)
	totalAssets = totalAssets.Add(hugeSwapIn)

	assetDenom := "underlying"
	redeemAll, err := utils.CalculateAssetsFromShares(firstMint.Amount, totalShares, totalAssets, assetDenom)
	require.NoError(t, err, "redeem conversion should not error")

	require.Truef(t,
		redeemAll.Amount.GTE(firstSwapIn),
		"redeemed underlying should be >= original deposit (got=%s, want >= %s)",
		redeemAll.Amount, firstSwapIn,
	)

	require.Truef(t,
		redeemAll.Amount.LTE(totalAssets),
		"redeemed underlying should be <= vault total assets (got=%s, want <= %s)",
		redeemAll.Amount, totalAssets,
	)

}

// TestVeryLargeInitialSwapInRoundTrip ensures that a very large first deposit
// correctly mints proportional shares and can be redeemed without loss or overflow.
//
// It confirms that:
//   - Large initial deposits still use the precision scaling.
//   - Redeeming the minted shares returns an amount within an acceptable
//     rounding error.
func TestVeryLargeInitialSwapInRoundTrip(t *testing.T) {
	large := sdkmath.NewInt(1_000_000_000_000_000_000)
	totalAssets := sdkmath.ZeroInt()
	totalShares := sdkmath.ZeroInt()

	shareDenom := "shares"
	minted, err := utils.CalculateSharesFromAssets(large, totalAssets, totalShares, shareDenom)
	require.NoError(t, err, "large first deposit conversion should not error")
	require.Equal(t, large.Mul(utils.ShareScalar), minted.Amount, "minted shares should equal deposit * ShareScalar")

	totalAssets = totalAssets.Add(large)
	totalShares = totalShares.Add(minted.Amount)

	assetDenom := "underlying"
	redeemed, err := utils.CalculateAssetsFromShares(minted.Amount, totalShares, totalAssets, assetDenom)
	require.NoError(t, err, "redeem conversion should not error")
	require.Truef(t,
		redeemed.Amount.GTE(large.SubRaw(1)),
		"redeemed underlying should be >= deposit - 1 (got=%s, want >= %s)",
		redeemed.Amount, large.SubRaw(1),
	)

	require.Truef(t,
		redeemed.Amount.LTE(large),
		"redeemed underlying should be <= deposit (got=%s, want <= %s)",
		redeemed.Amount, large,
	)
}
