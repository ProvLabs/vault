package keeper_test

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	markertypes "github.com/provenance-io/provenance/x/marker/types"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
)

func (s *TestSuite) TestUnitPriceFraction_Table() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 2_000_000), s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 2_000_000), s.adminAddr)

	paymentMarkerAddr := markertypes.MustGetMarkerAddress(paymentDenom)
	paymentMarkerAccount, err := s.k.MarkerKeeper.GetMarker(s.ctx, paymentMarkerAddr)
	s.Require().NoError(err, "should fetch payment marker account for NAV setup")
	navRecord := markertypes.NetAssetValue{
		Price:  sdk.NewInt64Coin(underlyingDenom, 1),
		Volume: 2,
	}
	s.Require().NoError(s.k.MarkerKeeper.SetNetAssetValue(s.ctx, paymentMarkerAccount, navRecord, "test"), "should set NAV usdc->ylds=1/2")

	vaultAccount := &types.VaultAccount{ShareDenom: "vshare", UnderlyingAsset: underlyingDenom}

	cases := []struct {
		name                  string
		fromDenom             string
		toDenom               string
		expectedNumerator     int64
		expectedDenominator   int64
		expectedErrorContains string
	}{
		{name: "identity", fromDenom: underlyingDenom, toDenom: underlyingDenom, expectedNumerator: 1, expectedDenominator: 1},
		{name: "nav-present", fromDenom: paymentDenom, toDenom: underlyingDenom, expectedNumerator: 1, expectedDenominator: 2},
		{name: "nav-missing", fromDenom: "unknown", toDenom: underlyingDenom, expectedErrorContains: "nav not found"},
	}

	for _, scenario := range cases {
		s.Run(scenario.name, func() {
			testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
			numerator, denominator, err := testKeeper.UnitPriceFraction(s.ctx, scenario.fromDenom, scenario.toDenom)
			if scenario.expectedErrorContains != "" {
				s.Require().Error(err, "should error when NAV missing")
				s.Require().Contains(err.Error(), scenario.expectedErrorContains, "error should mention NAV missing")
				return
			}
			s.Require().NoError(err, "should compute price fraction without error")
			s.Require().Equal(math.NewInt(scenario.expectedNumerator), numerator, "numerator should match expected")
			s.Require().Equal(math.NewInt(scenario.expectedDenominator), denominator, "denominator should match expected")

			if scenario.fromDenom == vaultAccount.UnderlyingAsset {
				s.Require().Equal(math.NewInt(1), numerator, "identity numerator should be 1")
				s.Require().Equal(math.NewInt(1), denominator, "identity denominator should be 1")
			}
		})
	}
}

func (s *TestSuite) TestToAssetAmount() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 2_000_000), s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 2_000_000), s.adminAddr)

	paymentMarkerAddr := markertypes.MustGetMarkerAddress(paymentDenom)
	paymentMarkerAccount, err := s.k.MarkerKeeper.GetMarker(s.ctx, paymentMarkerAddr)
	s.Require().NoError(err, "should fetch payment marker for NAV setup")
	s.Require().NoError(s.k.MarkerKeeper.SetNetAssetValue(s.ctx, paymentMarkerAccount, markertypes.NetAssetValue{
		Price:  sdk.NewInt64Coin(underlyingDenom, 1),
		Volume: 2,
	}, "test"), "should set NAV usdc->ylds=1/2")

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	vaultMeta := types.VaultAccount{ShareDenom: shareDenom, UnderlyingAsset: underlyingDenom}

	valueInAsset, err := testKeeper.ToAssetAmount(s.ctx, vaultMeta, sdk.NewInt64Coin(paymentDenom, 4))
	s.Require().NoError(err, "toAssetAmount should succeed for valid NAV")
	s.Require().Equal(2, valueInAsset, "4 usdc at 1/2 should be 2 ylds")
}

func (s *TestSuite) TestGetTVVInAsset_ExcludesSharesAndSumsInAsset() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 2_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, underlyingDenom, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 100_000)))
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 2_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, paymentDenom, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 100_000)))

	paymentMarkerAddr := markertypes.MustGetMarkerAddress(paymentDenom)
	paymentMarkerAccount, err := s.k.MarkerKeeper.GetMarker(s.ctx, paymentMarkerAddr)
	s.Require().NoError(err, "should fetch payment marker for NAV setup")
	s.Require().NoError(s.k.MarkerKeeper.SetNetAssetValue(s.ctx, paymentMarkerAccount, markertypes.NetAssetValue{
		Price:  sdk.NewInt64Coin(underlyingDenom, 1),
		Volume: 2,
	}, "test"), "should set NAV usdc->ylds=1/2")

	vaultCfg := vaultAttrs{admin: s.adminAddr.String(), share: shareDenom, base: underlyingDenom}
	vault, err := s.k.CreateVault(s.ctx, vaultCfg)
	s.Require().NoError(err, "vault creation should succeed")

	vaultAddress := types.GetVaultAddress(shareDenom)
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vaultAddress, sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
		sdk.NewInt64Coin(paymentDenom, 10),
	)), "should fund vault with base and payment coins")

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	totalVaultValueInAsset, err := testKeeper.GetTVVInAsset(s.ctx, *vault)
	s.Require().NoError(err, "get TVV should succeed")
	s.Require().Equal(math.NewInt(1005), totalVaultValueInAsset, "1000 ylds + 10 usdc at 1/2 should equal 1005 ylds")
}

func (s *TestSuite) TestGetNAVPerShareInAsset_FloorsToZeroForTinyPerShare() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 2_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, underlyingDenom, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 100_000)))
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 2_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, paymentDenom, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 100_000)))

	paymentMarkerAddr := markertypes.MustGetMarkerAddress(paymentDenom)
	paymentMarkerAccount, err := s.k.MarkerKeeper.GetMarker(s.ctx, paymentMarkerAddr)
	s.Require().NoError(err, "should fetch payment marker for NAV setup")
	s.Require().NoError(s.k.MarkerKeeper.SetNetAssetValue(s.ctx, paymentMarkerAccount, markertypes.NetAssetValue{
		Price:  sdk.NewInt64Coin(underlyingDenom, 1),
		Volume: 2,
	}, "test"), "should set NAV usdc->ylds=1/2")

	vaultCfg := vaultAttrs{admin: s.adminAddr.String(), share: shareDenom, base: underlyingDenom}
	vault, err := s.k.CreateVault(s.ctx, vaultCfg)
	s.Require().NoError(err, "vault creation should succeed")

	vaultAddress := types.GetVaultAddress(shareDenom)
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vaultAddress, sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
		sdk.NewInt64Coin(paymentDenom, 10),
	)), "should fund vault marker for NAV calc")

	totalVaultValueInAsset := math.NewInt(1005)
	shareSupplyMint := sdk.NewCoin(shareDenom, utils.ShareScalar.Mul(totalVaultValueInAsset))
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), shareSupplyMint), "should mint share supply matching tvv*ShareScalar")

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	navPerShareAsset, err := testKeeper.GetNAVPerShareInAsset(s.ctx, *vault)
	s.Require().NoError(err, "nav per share should compute without error")
	s.Require().Equal(math.ZeroInt(), navPerShareAsset, "with scaled shares, integer NAV/share should floor to 0")
}

func (s *TestSuite) TestConvertDepositToSharesInAsset_UsesNAV() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 2_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, underlyingDenom, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 100_000)))
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 2_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, paymentDenom, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 100_000)))

	paymentMarkerAddr := markertypes.MustGetMarkerAddress(paymentDenom)
	paymentMarkerAccount, err := s.k.MarkerKeeper.GetMarker(s.ctx, paymentMarkerAddr)
	s.Require().NoError(err, "should fetch payment marker for NAV setup")
	s.Require().NoError(s.k.MarkerKeeper.SetNetAssetValue(s.ctx, paymentMarkerAccount, markertypes.NetAssetValue{
		Price:  sdk.NewInt64Coin(underlyingDenom, 1),
		Volume: 2,
	}, "test"), "should set NAV usdc->ylds=1/2")

	vaultCfg := vaultAttrs{admin: s.adminAddr.String(), share: shareDenom, base: underlyingDenom}
	vault, err := s.k.CreateVault(s.ctx, vaultCfg)
	s.Require().NoError(err, "vault creation should succeed")

	principalAddress := markertypes.MustGetMarkerAddress(shareDenom)
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, principalAddress, sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
		sdk.NewInt64Coin(paymentDenom, 10),
	)), "should fund vault marker for TVV")

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	mintedShares, err := testKeeper.ConvertDepositToSharesInAsset(s.ctx, *vault, sdk.NewInt64Coin(paymentDenom, 4))
	s.Require().NoError(err, "deposit conversion should succeed")
	s.Require().Equal(shareDenom, mintedShares.Denom, "minted shares denom should match vault share denom")
	s.Require().Equal(utils.ShareScalar.Mul(math.NewInt(2)), mintedShares.Amount, "4 usdc at 1/2 should mint 2*ShareScalar shares")
}

func (s *TestSuite) TestConvertSharesToRedeemCoinInAsset_AssetAndPaymentPaths() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 2_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, underlyingDenom, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 100_000)))
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 2_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, paymentDenom, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 100_000)))

	paymentMarkerAddr := markertypes.MustGetMarkerAddress(paymentDenom)
	paymentMarkerAccount, err := s.k.MarkerKeeper.GetMarker(s.ctx, paymentMarkerAddr)
	s.Require().NoError(err, "should fetch payment marker for NAV setup")
	s.Require().NoError(s.k.MarkerKeeper.SetNetAssetValue(s.ctx, paymentMarkerAccount, markertypes.NetAssetValue{
		Price:  sdk.NewInt64Coin(underlyingDenom, 1),
		Volume: 2,
	}, "test"), "should set NAV usdc->ylds=1/2")

	vaultCfg := vaultAttrs{admin: s.adminAddr.String(), share: shareDenom, base: underlyingDenom}
	vault, err := s.k.CreateVault(s.ctx, vaultCfg)
	s.Require().NoError(err, "vault creation should succeed")

	vaultAddress := types.GetVaultAddress(shareDenom)
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vaultAddress, sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
		sdk.NewInt64Coin(paymentDenom, 10),
	)), "should fund vault with base and payment coins")

	totalVaultValueInAsset := math.NewInt(1005)
	shareSupply := utils.ShareScalar.Mul(totalVaultValueInAsset)
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewCoin(shareDenom, shareSupply)), "should mint share supply equal to tvv*ShareScalar")

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	outAssetCoin, err := testKeeper.ConvertSharesToRedeemCoinInAsset(s.ctx, *vault, utils.ShareScalar, underlyingDenom)
	s.Require().NoError(err, "shares->asset conversion should succeed")
	s.Require().Equal(underlyingDenom, outAssetCoin.Denom, "redeem denom should be asset")
	s.Require().Equal(math.NewInt(1), outAssetCoin.Amount, "ShareScalar shares should redeem to 1 asset unit at parity")

	outPaymentCoin, err := testKeeper.ConvertSharesToRedeemCoinInAsset(s.ctx, *vault, utils.ShareScalar, paymentDenom)
	s.Require().NoError(err, "shares->payment conversion should succeed")
	s.Require().Equal(paymentDenom, outPaymentCoin.Denom, "redeem denom should be payment")
	s.Require().Equal(math.NewInt(2), outPaymentCoin.Amount, "1 asset at 1/2 asset per 1 payment should yield 2 payment units")
}
