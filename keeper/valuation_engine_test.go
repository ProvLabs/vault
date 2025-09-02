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
	shareDenom := "vshare"
	s.setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom, 1, 2)

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
			s.Require().True(denominator.IsPositive(), "denominator should be > 0")
		})
	}
}

func (s *TestSuite) TestToUnderlyingAssetAmount() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom, 1, 2)

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}

	val, err := testKeeper.ToUnderlyingAssetAmount(s.ctx, *vault, sdk.NewInt64Coin(paymentDenom, 4))
	s.Require().NoError(err, "to-underlying should succeed for valid NAV")
	s.Require().Equal(math.NewInt(2), val, "4 usdc at 1/2 should be 2 ylds")

	valFloor, err := testKeeper.ToUnderlyingAssetAmount(s.ctx, *vault, sdk.NewInt64Coin(paymentDenom, 5))
	s.Require().NoError(err, "to-underlying with floor should succeed")
	s.Require().Equal(math.NewInt(2), valFloor, "5 usdc at 1/2 should floor to 2 ylds (not 2.5)")

	valZero, err := testKeeper.ToUnderlyingAssetAmount(s.ctx, *vault, sdk.NewInt64Coin(paymentDenom, 0))
	s.Require().NoError(err, "to-underlying with zero amount should succeed")
	s.Require().Equal(math.ZeroInt(), valZero, "0 usdc should convert to 0 ylds")

	_, err = testKeeper.ToUnderlyingAssetAmount(s.ctx, *vault, sdk.NewInt64Coin("unknown", 5))
	s.Require().Error(err, "should error when NAV missing for input denom")
	s.Require().Contains(err.Error(), "nav not found", "error should mention missing NAV")
}

func (s *TestSuite) TestToUnderlyingAssetAmount_IdentityFastPath() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	inputAmount := int64(123456789)
	outAmount, err := testKeeper.ToUnderlyingAssetAmount(s.ctx, *vault, sdk.NewInt64Coin(underlyingDenom, inputAmount))
	s.Require().NoError(err, "identity conversion to underlying should not error for denom %s", underlyingDenom)
	s.Require().Equal(math.NewInt(inputAmount), outAmount, "identity conversion should preserve the input amount for denom %s", underlyingDenom)
}

func (s *TestSuite) TestFromUnderlyingAssetAmount() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom, 1, 2)

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}

	out1, err := testKeeper.FromUnderlyingAssetAmount(s.ctx, *vault, math.NewInt(3), underlyingDenom)
	s.Require().NoError(err, "from-underlying identity should succeed")
	s.Require().Equal(underlyingDenom, out1.Denom, "identity denom should be underlying")
	s.Require().Equal(math.NewInt(3), out1.Amount, "identity amount should match input")

	out2, err := testKeeper.FromUnderlyingAssetAmount(s.ctx, *vault, math.NewInt(3), paymentDenom)
	s.Require().NoError(err, "from-underlying to payment should succeed")
	s.Require().Equal(paymentDenom, out2.Denom, "output denom should be payment")
	s.Require().Equal(math.NewInt(6), out2.Amount, "3 underlying at 1/2 asset per 1 payment yields 6 payment")

	paymentMarkerAddr := markertypes.MustGetMarkerAddress(paymentDenom)
	paymentMarkerAccount, err := s.k.MarkerKeeper.GetMarker(s.ctx, paymentMarkerAddr)
	s.Require().NoError(err, "should fetch payment marker account for NAV setup")
	s.Require().NoError(
		s.k.MarkerKeeper.SetNetAssetValue(s.ctx, paymentMarkerAccount, markertypes.NetAssetValue{
			Price:  sdk.NewInt64Coin(underlyingDenom, 3),
			Volume: 2,
		}, "test"), "should set NAV usdc->ylds=3/2")

	outFloor, err := testKeeper.FromUnderlyingAssetAmount(s.ctx, *vault, math.NewInt(4), paymentDenom)
	s.Require().NoError(err, "from-underlying with floor should succeed")
	s.Require().Equal(paymentDenom, outFloor.Denom, "output denom should be payment")
	s.Require().Equal(math.NewInt(2), outFloor.Amount, "4 underlying at 3/2 asset per 1 payment yields 2 payment (floored from 2.66)")

	outZero, err := testKeeper.FromUnderlyingAssetAmount(s.ctx, *vault, math.ZeroInt(), paymentDenom)
	s.Require().NoError(err, "zero underlying should not error")
	s.Require().Equal(math.ZeroInt(), outZero.Amount, "zero underlying should yield zero output")

	_, err = testKeeper.FromUnderlyingAssetAmount(s.ctx, *vault, math.NewInt(5), "unknown")
	s.Require().Error(err, "should error when NAV missing for output denom")
	s.Require().Contains(err.Error(), "nav not found", "error should mention missing NAV")

	s.Require().NoError(
		s.k.MarkerKeeper.SetNetAssetValue(s.ctx, paymentMarkerAccount, markertypes.NetAssetValue{
			Price:  sdk.NewInt64Coin(underlyingDenom, 0),
			Volume: 2,
		}, "test"), "should set zero-price NAV")
	_, err = testKeeper.FromUnderlyingAssetAmount(s.ctx, *vault, math.NewInt(5), paymentDenom)
	s.Require().Error(err, "should error when price is zero")
	s.Require().Contains(err.Error(), "zero price", "error should indicate zero price")
}

func (s *TestSuite) TestFromUnderlyingAssetAmount_NegativeUnderlyingYieldsZero() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	s.setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom, 1, 2)

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	vaultAccount := types.VaultAccount{ShareDenom: shareDenom, UnderlyingAsset: underlyingDenom}

	outCoin, err := testKeeper.FromUnderlyingAssetAmount(s.ctx, vaultAccount, math.NewInt(-5), paymentDenom)
	s.Require().NoError(err, "converting negative underlying amount should not error and should yield a zero coin")
	s.Require().Equal(paymentDenom, outCoin.Denom, "output denom should match requested payout denom")
	s.Require().True(outCoin.IsZero(), "output coin should be zero when underlying amount is non-positive")
}

func (s *TestSuite) TestGetTVVInUnderlyingAsset_ExcludesSharesAndSumsInAsset() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom, 1, 2)

	principalAddress := vault.PrincipalMarkerAddress()
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, principalAddress, sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
		sdk.NewInt64Coin(paymentDenom, 10),
	)), "should fund vault with base and payment coins")

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	totalVaultValueInAsset, err := testKeeper.GetTVVInUnderlyingAsset(s.ctx, *vault)
	s.Require().NoError(err, "get TVV should succeed")
	s.Require().Equal(math.NewInt(1005), totalVaultValueInAsset, "1000 ylds + 10 usdc at 1/2 should equal 1005 ylds")
}

func (s *TestSuite) TestGetTVVInUnderlyingAsset_EmptyAndSharesOnly() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	tvvEmpty, err := testKeeper.GetTVVInUnderlyingAsset(s.ctx, *vault)
	s.Require().NoError(err, "get TVV of empty vault should succeed")
	s.Require().Equal(math.ZeroInt(), tvvEmpty, "TVV of empty vault should be zero")

	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewInt64Coin(shareDenom, 1000)), "should mint shares")
	tvvSharesOnly, err := testKeeper.GetTVVInUnderlyingAsset(s.ctx, *vault)
	s.Require().NoError(err, "get TVV of shares-only vault should succeed")
	s.Require().Equal(math.ZeroInt(), tvvSharesOnly, "TVV of shares-only vault should be zero")
}

func (s *TestSuite) TestGetTVVInUnderlyingAsset_ErrorPropagation() {
	underlyingDenom := "ylds"
	paymentDenomWithNAV := "usdc"
	paymentDenomWithoutNAV := "euro"
	shareDenom := "vshare"
	vault := s.setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenomWithNAV, 1, 2)

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenomWithoutNAV, 2_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, paymentDenomWithoutNAV, sdk.NewCoins(sdk.NewInt64Coin(paymentDenomWithoutNAV, 100_000)))

	principalAddress := vault.PrincipalMarkerAddress()
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, principalAddress, sdk.NewCoins(
		sdk.NewInt64Coin(paymentDenomWithNAV, 10),
		sdk.NewInt64Coin(paymentDenomWithoutNAV, 5),
	)), "should fund vault with both valid and invalid assets")

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	_, err := testKeeper.GetTVVInUnderlyingAsset(s.ctx, *vault)
	s.Require().Error(err, "get TVV should fail when an asset is missing a NAV")
	s.Require().Contains(err.Error(), "nav not found for euro/ylds", "error should propagate from the missing NAV")
}

func (s *TestSuite) TestGetTVVInUnderlyingAsset_MultiDenomFlooring() {
	underlyingDenom := "ylds"
	paymentADenom := "usdc"
	paymentBDenom := "eurc"
	shareDenom := "vshare"

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 2_000_000), s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentADenom, 2_000_000), s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentBDenom, 2_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, underlyingDenom, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 1)))
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, paymentADenom, sdk.NewCoins(sdk.NewInt64Coin(paymentADenom, 4)))
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, paymentBDenom, sdk.NewCoins(sdk.NewInt64Coin(paymentBDenom, 5)))

	paymentAMarkerAddr := markertypes.MustGetMarkerAddress(paymentADenom)
	paymentAMarkerAccount, err := s.k.MarkerKeeper.GetMarker(s.ctx, paymentAMarkerAddr)
	s.Require().NoError(err, "fetching payment A marker account should succeed")
	paymentBMarkerAddr := markertypes.MustGetMarkerAddress(paymentBDenom)
	paymentBMarkerAccount, err := s.k.MarkerKeeper.GetMarker(s.ctx, paymentBMarkerAddr)
	s.Require().NoError(err, "fetching payment B marker account should succeed")

	s.Require().NoError(
		s.k.MarkerKeeper.SetNetAssetValue(s.ctx, paymentAMarkerAccount, markertypes.NetAssetValue{Price: sdk.NewInt64Coin(underlyingDenom, 1), Volume: 3}, "set NAV A 1/3"),
		"setting NAV for %s should succeed", paymentADenom)
	s.Require().NoError(
		s.k.MarkerKeeper.SetNetAssetValue(s.ctx, paymentBMarkerAccount, markertypes.NetAssetValue{Price: sdk.NewInt64Coin(underlyingDenom, 2), Volume: 3}, "set NAV B 2/3"),
		"setting NAV for %s should succeed", paymentBDenom)

	vaultAccount, err := s.k.CreateVault(s.ctx, vaultAttrs{admin: s.adminAddr.String(), share: shareDenom, underlying: underlyingDenom})
	s.Require().NoError(err, "vault creation should succeed for multi-denom TVV test")

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vaultAccount.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1),
		sdk.NewInt64Coin(paymentADenom, 4),
		sdk.NewInt64Coin(paymentBDenom, 5),
	)), "sending mixed funding to principal should succeed")

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	tvv, err := testKeeper.GetTVVInUnderlyingAsset(s.ctx, *vaultAccount)
	s.Require().NoError(err, "computing TVV with multiple NAV mappings should not error")
	s.Require().Equal(math.NewInt(5), tvv, "TVV should equal 1 underlying + floor(4*1/3)=1 + floor(5*2/3)=3 => total 5")
}

func (s *TestSuite) TestGetTVVInUnderlyingAsset_ExcludesReserves() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.GetAddress(), sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 999))),
		"sending funds to the vault account (reserves) should succeed")

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	tvv, err := testKeeper.GetTVVInUnderlyingAsset(s.ctx, *vault)
	s.Require().NoError(err, "computing TVV should not error when only reserves are funded")
	s.Require().True(tvv.IsZero(), "TVV should be zero because reserves are excluded and principal is unfunded")
}

func (s *TestSuite) TestGetNAVPerShareInUnderlyingAsset_FloorsToZeroForTinyPerShare() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom, 1, 2)

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
		sdk.NewInt64Coin(paymentDenom, 10),
	)), "should fund vault marker for NAV calc")

	totalVaultValueInAsset := math.NewInt(1005)
	shareSupplyMint := sdk.NewCoin(shareDenom, utils.ShareScalar.Mul(totalVaultValueInAsset))
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), shareSupplyMint), "should mint share supply matching tvv*ShareScalar")

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	navPerShareAsset, err := testKeeper.GetNAVPerShareInUnderlyingAsset(s.ctx, *vault)
	s.Require().NoError(err, "nav per share should compute without error")
	s.Require().Equal(math.ZeroInt(), navPerShareAsset, "with scaled shares, integer NAV/share should floor to 0")
}

func (s *TestSuite) TestGetNAVPerShareInUnderlyingAsset_ZeroSupplyAndNormalNAV() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
	)), "should fund vault for TVV")

	navPerShareZeroSupply, err := testKeeper.GetNAVPerShareInUnderlyingAsset(s.ctx, *vault)
	s.Require().NoError(err, "should not error with zero share supply")
	s.Require().Equal(math.ZeroInt(), navPerShareZeroSupply, "NAV per share should be zero with zero share supply")

	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewInt64Coin(shareDenom, 500)), "should mint shares")
	navPerShareNormal, err := testKeeper.GetNAVPerShareInUnderlyingAsset(s.ctx, *vault)
	s.Require().NoError(err, "should compute normal NAV per share")
	s.Require().Equal(math.NewInt(2), navPerShareNormal, "1000 TVV / 500 shares should be 2 NAV per share")
}

func (s *TestSuite) TestConvertDepositToSharesInUnderlyingAsset_UsesNAV() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom, 1, 2)

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
		sdk.NewInt64Coin(paymentDenom, 10),
	)), "should fund vault marker for TVV")

	tvv, err := testKeeper.GetTVVInUnderlyingAsset(s.ctx, *vault)
	s.Require().NoError(err, "should compute TVV")
	initialShares := tvv.Mul(utils.ShareScalar)
	s.Require().NoError(
		s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewCoin(shareDenom, initialShares)),
		"should mint initial shares to match TVV",
	)

	mintedShares, err := testKeeper.ConvertDepositToSharesInUnderlyingAsset(s.ctx, *vault, sdk.NewInt64Coin(paymentDenom, 4))
	s.Require().NoError(err, "deposit conversion should succeed")
	s.Require().Equal(shareDenom, mintedShares.Denom, "minted shares denom should match vault share denom")
	s.Require().Equal(utils.ShareScalar.Mul(math.NewInt(2)), mintedShares.Amount, "4 usdc at 1/2 should mint 2*ShareScalar shares")
}

func (s *TestSuite) TestConvertDepositToSharesInUnderlyingAsset_InitialAndDustDeposits() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	paymentMarkerAddr := markertypes.MustGetMarkerAddress(paymentDenom)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 2_000_000), s.adminAddr)
	paymentMarkerAccount, err := s.k.MarkerKeeper.GetMarker(s.ctx, paymentMarkerAddr)
	s.Require().NoError(err, "should fetch payment marker for NAV setup")
	s.Require().NoError(s.k.MarkerKeeper.SetNetAssetValue(s.ctx, paymentMarkerAccount, markertypes.NetAssetValue{
		Price:  sdk.NewInt64Coin(underlyingDenom, 1),
		Volume: 2_000_000_000,
	}, "test"), "should set NAV usdc->ylds=1/2e9 (for dust test)")

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	initialDepositShares, err := testKeeper.ConvertDepositToSharesInUnderlyingAsset(s.ctx, *vault, sdk.NewInt64Coin(underlyingDenom, 100))
	s.Require().NoError(err, "initial deposit should succeed")
	s.Require().Equal(utils.ShareScalar.Mul(math.NewInt(100)), initialDepositShares.Amount, "initial deposit should mint shares at parity with ShareScalar")

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
	)), "should fund vault for TVV")
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewCoin(shareDenom, math.NewInt(1))), "should add a single share to circulation")
	dustDepositShares, err := testKeeper.ConvertDepositToSharesInUnderlyingAsset(s.ctx, *vault, sdk.NewInt64Coin(paymentDenom, 1))
	s.Require().NoError(err, "dust deposit should not error")
	s.Require().True(dustDepositShares.Amount.IsZero(), "dust deposit should floor to zero shares")
}

func (s *TestSuite) TestConvertSharesToRedeemCoin_AssetAndPaymentPaths() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom, 1, 2)

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
		sdk.NewInt64Coin(paymentDenom, 10),
	)), "should fund vault with base and payment coins")

	totalVaultValueInAsset := math.NewInt(1005)
	shareSupply := utils.ShareScalar.Mul(totalVaultValueInAsset)
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewCoin(shareDenom, shareSupply)), "should mint share supply equal to tvv*ShareScalar")

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	outAssetCoin, err := testKeeper.ConvertSharesToRedeemCoin(s.ctx, *vault, utils.ShareScalar, underlyingDenom)
	s.Require().NoError(err, "shares->asset conversion should succeed")
	s.Require().Equal(underlyingDenom, outAssetCoin.Denom, "redeem denom should be asset")
	s.Require().Equal(math.NewInt(1), outAssetCoin.Amount, "ShareScalar shares should redeem to 1 asset unit at parity")

	outPaymentCoin, err := testKeeper.ConvertSharesToRedeemCoin(s.ctx, *vault, utils.ShareScalar, paymentDenom)
	s.Require().NoError(err, "shares->payment conversion should succeed")
	s.Require().Equal(paymentDenom, outPaymentCoin.Denom, "redeem denom should be payment")
	s.Require().Equal(math.NewInt(2), outPaymentCoin.Amount, "1 asset at 1/2 asset per 1 payment should yield 2 payment units")

	_, err = testKeeper.ConvertSharesToRedeemCoin(s.ctx, *vault, utils.ShareScalar, "unknown")
	s.Require().Error(err, "should error when redeem denom has no NAV to underlying")
	s.Require().Contains(err.Error(), "nav not found", "error should indicate missing NAV mapping")
}

func (s *TestSuite) TestConvertSharesToRedeemCoin_ZeroAndDustRedemption() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}

	zeroCoin, err := testKeeper.ConvertSharesToRedeemCoin(s.ctx, *vault, math.ZeroInt(), underlyingDenom)
	s.Require().NoError(err, "redeeming zero shares should not error")
	s.Require().True(zeroCoin.IsZero(), "redeeming zero shares should result in a zero coin")

	negativeCoin, err := testKeeper.ConvertSharesToRedeemCoin(s.ctx, *vault, math.NewInt(-100), underlyingDenom)
	s.Require().NoError(err, "redeeming negative shares should not error")
	s.Require().True(negativeCoin.IsZero(), "redeeming negative shares should result in a zero coin")

	tvv := math.NewInt(1000)
	totalShares := tvv.Mul(utils.ShareScalar)
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewCoin(shareDenom, totalShares)), "should mint total shares")
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
	)), "should fund vault for TVV")
	dustCoin, err := testKeeper.ConvertSharesToRedeemCoin(s.ctx, *vault, math.NewInt(1), underlyingDenom)
	s.Require().NoError(err, "dust redemption should not error")
	s.Require().True(dustCoin.IsZero(), "dust redemption should floor to a zero coin")
}

func (s *TestSuite) TestConvertSharesToRedeemCoin_TVVZero_SupplyNonzero() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	initialShareSupply := sdk.NewCoin(shareDenom, utils.ShareScalar)
	s.Require().NoError(
		s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), initialShareSupply),
		"minting initial shares while TVV is zero should succeed",
	)

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	redeemCoin, err := testKeeper.ConvertSharesToRedeemCoin(s.ctx, *vault, utils.ShareScalar, underlyingDenom)
	s.Require().NoError(err, "redeeming with TVV=0 and shares>0 should not error")
	s.Require().Equal(underlyingDenom, redeemCoin.Denom, "redeem denom should be the underlying asset")
	s.Require().True(redeemCoin.Amount.IsZero(), "redeemed amount should be zero when TVV is zero")
}
