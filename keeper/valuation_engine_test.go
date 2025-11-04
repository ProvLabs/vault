package keeper_test

import (
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	markertypes "github.com/provenance-io/provenance/x/marker/types"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
)

func (s *TestSuite) TestUnitPriceFraction_Table() {
	const uyldsFccDenom = "uylds.fcc"

	underlyingDenom := "underlying"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	s.setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom, 1, 2)

	cases := []struct {
		name                  string
		fromDenom             string
		underlyingOverride    string
		paymentOverride       string
		setup                 func()
		expectedNumerator     int64
		expectedDenominator   int64
		expectedErrorContains string
	}{
		{
			name:                "identity-src-equals-underlying",
			fromDenom:           underlyingDenom,
			expectedNumerator:   1,
			expectedDenominator: 1,
		},
		{
			name:                "payment-denom-uylds-fcc-fastpath",
			fromDenom:           paymentDenom,
			paymentOverride:     uyldsFccDenom,
			expectedNumerator:   1,
			expectedDenominator: 1,
		},
		{
			name:                "underlying-uylds-fcc-fastpath",
			fromDenom:           paymentDenom,
			underlyingOverride:  uyldsFccDenom,
			expectedNumerator:   1,
			expectedDenominator: 1,
		},
		{
			name:                "forward-nav-present",
			fromDenom:           paymentDenom,
			expectedNumerator:   1,
			expectedDenominator: 2,
		},
		{
			name:      "reverse-nav-present-newer",
			fromDenom: paymentDenom,
			setup: func() {
				s.bumpHeight()
				s.setReverseNAV(underlyingDenom, paymentDenom, 2, 1)
			},
			expectedNumerator:   1,
			expectedDenominator: 2,
		},
		{
			name:      "reverse-nav-selected-zero-price-errors",
			fromDenom: paymentDenom,
			setup: func() {
				s.bumpHeight()
				s.setReverseNAV(underlyingDenom, paymentDenom, 0, 1)
			},
			expectedErrorContains: "nav price is zero",
		},
		{
			name:                  "nav-missing",
			fromDenom:             "unknown",
			expectedErrorContains: "nav not found",
		},
		{
			name:      "both-present-forward-newer-selected",
			fromDenom: paymentDenom,
			setup: func() {
				pmtMarkerAddr := markertypes.MustGetMarkerAddress(paymentDenom)
				pmtMarkerAcct, err := s.k.MarkerKeeper.GetMarker(s.ctx, pmtMarkerAddr)
				s.Require().NoError(err)
				err = s.k.MarkerKeeper.SetNetAssetValue(s.ctx, pmtMarkerAcct, markertypes.NetAssetValue{
					Price:  sdk.NewInt64Coin(underlyingDenom, 3),
					Volume: 2,
				}, "fwd-old")
				s.Require().NoError(err)
				s.setReverseNAV(underlyingDenom, paymentDenom, 5, 7)
				s.bumpHeight()
				err = s.k.MarkerKeeper.SetNetAssetValue(s.ctx, pmtMarkerAcct, markertypes.NetAssetValue{
					Price:  sdk.NewInt64Coin(underlyingDenom, 6),
					Volume: 4,
				}, "fwd-new")
				s.Require().NoError(err)
			},
			expectedNumerator:   6,
			expectedDenominator: 4,
		},
		{
			name:      "both-present-same-height-forward-wins",
			fromDenom: paymentDenom,
			setup: func() {
				pmtMarkerAddr := markertypes.MustGetMarkerAddress(paymentDenom)
				pmtMarkerAcct, err := s.k.MarkerKeeper.GetMarker(s.ctx, pmtMarkerAddr)
				s.Require().NoError(err)
				s.setReverseNAV(underlyingDenom, paymentDenom, 11, 5)
				err = s.k.MarkerKeeper.SetNetAssetValue(s.ctx, pmtMarkerAcct, markertypes.NetAssetValue{
					Price:  sdk.NewInt64Coin(underlyingDenom, 9),
					Volume: 3,
				}, "fwd-same-height")
				s.Require().NoError(err)
			},
			expectedNumerator:   9,
			expectedDenominator: 3,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			if tc.setup != nil {
				tc.setup()
			}
			vault := types.VaultAccount{
				UnderlyingAsset: underlyingDenom,
				PaymentDenom:    paymentDenom,
				TotalShares:     sdk.NewInt64Coin(shareDenom, 0),
			}
			if tc.underlyingOverride != "" {
				vault.UnderlyingAsset = tc.underlyingOverride
			}
			if tc.paymentOverride != "" {
				vault.PaymentDenom = tc.paymentOverride
			}
			testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
			num, den, err := testKeeper.UnitPriceFraction(s.ctx, tc.fromDenom, vault)
			if tc.expectedErrorContains != "" {
				s.Require().Error(err, "case %q", tc.name)
				s.Require().Contains(err.Error(), tc.expectedErrorContains, "case %q", tc.name)
				return
			}
			s.Require().NoError(err, "case %q", tc.name)
			s.Require().Equal(math.NewInt(tc.expectedNumerator), num, "case %q numerator", tc.name)
			s.Require().Equal(math.NewInt(tc.expectedDenominator), den, "case %q denominator", tc.name)
			s.Require().True(den.IsPositive(), "case %q denominator positive", tc.name)
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
	vault.TotalShares = sdk.NewInt64Coin(shareDenom, 1000)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

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
	vault.TotalShares = shareSupplyMint
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

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
	vault.TotalShares = sdk.NewInt64Coin(shareDenom, 500)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
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
	initialShares := sdk.NewCoin(shareDenom, tvv.Mul(utils.ShareScalar))
	s.Require().NoError(
		s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), initialShares),
		"should mint initial shares to match TVV",
	)
	vault.TotalShares = initialShares
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

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
	vault.TotalShares = sdk.NewCoin(shareDenom, math.NewInt(1))
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
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
	vault.TotalShares = sdk.NewCoin(shareDenom, shareSupply)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

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
	vault.TotalShares = sdk.NewCoin(shareDenom, totalShares)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
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
	vault.TotalShares = initialShareSupply
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	redeemCoin, err := testKeeper.ConvertSharesToRedeemCoin(s.ctx, *vault, utils.ShareScalar, underlyingDenom)
	s.Require().NoError(err, "redeeming with TVV=0 and shares>0 should not error")
	s.Require().Equal(underlyingDenom, redeemCoin.Denom, "redeem denom should be the underlying asset")
	s.Require().True(redeemCoin.Amount.IsZero(), "redeemed amount should be zero when TVV is zero")
}

func (s *TestSuite) TestGetTVVInUnderlyingAsset_PausedUsesPausedBalance() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom, 1, 2)

	principal := vault.PrincipalMarkerAddress()
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, principal, sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 9999),
		sdk.NewInt64Coin(paymentDenom, 9999),
	)), "funding principal balances before pause should succeed")

	vault.Paused = true
	vault.PausedBalance = sdk.NewInt64Coin(underlyingDenom, 42)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	tvv, err := testKeeper.GetTVVInUnderlyingAsset(s.ctx, *vault)
	s.Require().NoError(err, "GetTVVInUnderlyingAsset should not error when paused")
	s.Require().Equal(math.NewInt(42), tvv, "when paused, TVV should equal vault.PausedBalance.Amount regardless of principal contents")
}

func (s *TestSuite) TestEstimateTotalVaultValue_Paused() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 9999),
	)), "funding principal should succeed")
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.GetAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
	)), "funding reserves should succeed")

	vault.Paused = true
	vault.PausedBalance = sdk.NewInt64Coin(underlyingDenom, 42)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	estimatedTVV, err := testKeeper.EstimateTotalVaultValue(s.ctx, vault)

	s.Require().NoError(err, "estimation should not error when paused")
	s.Require().Equal(vault.PausedBalance, estimatedTVV, "estimated TVV should equal PausedBalance when vault is paused")
}

func (s *TestSuite) TestEstimateTotalVaultValue_SingleAsset() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
	)), "funding principal should succeed")
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.GetAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
	)), "funding reserves should succeed")

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	estimatedTVV, err := testKeeper.EstimateTotalVaultValue(s.ctx, vault)

	s.Require().NoError(err, "estimation should not error for single asset")
	expectedCoin := sdk.NewInt64Coin(underlyingDenom, 1000)
	s.Require().Equal(expectedCoin, estimatedTVV, "estimated TVV should equal principal balance")
}

func (s *TestSuite) TestEstimateTotalVaultValue_SingleAsset_WithNegativeInterest() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	const interestRate = "-0.1"
	const secondsToAccrue = int64(60 * 60 * 24 * 30)

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
	)), "funding principal should succeed")
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.GetAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
	)), "funding reserves should succeed")

	startTime := s.ctx.BlockTime()
	vault.PeriodStart = startTime.Unix()
	vault.CurrentInterestRate = interestRate
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	s.ctx = s.ctx.WithBlockTime(startTime.Add(time.Second * time.Duration(secondsToAccrue)))

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	estimatedTVV, err := testKeeper.EstimateTotalVaultValue(s.ctx, vault)

	s.Require().NoError(err, "estimation should not error for single asset")

	baseAmt := math.NewInt(1000)
	expectedTotalAmount, err := expectedWithSimpleAPY(baseAmt, interestRate, secondsToAccrue)
	s.Require().NoError(err, "calculating expected APY should not fail")

	expectedCoin := sdk.NewCoin(underlyingDenom, expectedTotalAmount)
	s.Require().Equal(expectedCoin, estimatedTVV, "estimated TVV should equal principal balance minus accrued negative interest")
}

func (s *TestSuite) TestEstimateTotalVaultValue_SingleAsset_WithInterest() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	const interestRate = "0.1"
	const secondsToAccrue = int64(60 * 60 * 24 * 30)

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
	)), "funding principal should succeed")
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.GetAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
	)), "funding reserves should succeed")

	startTime := s.ctx.BlockTime()
	vault.PeriodStart = startTime.Unix()
	vault.CurrentInterestRate = interestRate
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	s.ctx = s.ctx.WithBlockTime(startTime.Add(time.Second * time.Duration(secondsToAccrue)))

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	estimatedTVV, err := testKeeper.EstimateTotalVaultValue(s.ctx, vault)

	s.Require().NoError(err, "estimation should not error for single asset")

	baseAmt := math.NewInt(1000)
	expectedTotalAmount, err := expectedWithSimpleAPY(baseAmt, interestRate, secondsToAccrue)
	s.Require().NoError(err, "calculating expected APY should not fail")

	expectedCoin := sdk.NewCoin(underlyingDenom, expectedTotalAmount)
	s.Require().Equal(expectedCoin, estimatedTVV, "estimated TVV should equal principal balance plus accrued interest")
}

func (s *TestSuite) TestEstimateTotalVaultValue_MultiAsset_UnderlyingIsFcc() {
	underlyingDenom := "uylds.fcc"
	paymentDenom := "usdc"
	shareDenom := "vsharefcc"

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 1_000_000), s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 1_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, underlyingDenom, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 110)))
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, paymentDenom, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 50)))

	vault, err := s.k.CreateVault(s.ctx, vaultAttrs{admin: s.adminAddr.String(), share: shareDenom, underlying: underlyingDenom})
	s.Require().NoError(err, "vault creation should succeed")
	vault.PaymentDenom = paymentDenom
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
		sdk.NewInt64Coin(paymentDenom, 50),
	)), "funding principal should succeed")
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.GetAddress(), sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 10))), "funding reserves should succeed")

	testKeeper := s.k
	estimatedTVV, err := testKeeper.EstimateTotalVaultValue(s.ctx, vault)

	s.Require().NoError(err, "estimation should not error with uylds.fcc underlying")
	expectedCoin := sdk.NewInt64Coin(underlyingDenom, 150)
	s.Require().Equal(expectedCoin, estimatedTVV, "estimated TVV should sum assets at 1:1")
}

func (s *TestSuite) TestEstimateTotalVaultValue_MultiAsset_UnderlyingIsFcc_WithInterest() {
	underlyingDenom := "uylds.fcc"
	paymentDenom := "usdc"
	shareDenom := "vsharefcc"

	const interestRate = "0.1"
	const secondsToAccrue = int64(60 * 60 * 24 * 30)

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 1_000_000), s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 1_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, underlyingDenom, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 110)))
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, paymentDenom, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 50)))

	vault, err := s.k.CreateVault(s.ctx, vaultAttrs{admin: s.adminAddr.String(), share: shareDenom, underlying: underlyingDenom})
	s.Require().NoError(err, "vault creation should succeed")
	vault.PaymentDenom = paymentDenom

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
		sdk.NewInt64Coin(paymentDenom, 50),
	)), "funding principal should succeed")
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.GetAddress(), sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 10))), "funding reserves should succeed")

	startTime := s.ctx.BlockTime()
	vault.PeriodStart = startTime.Unix()
	vault.CurrentInterestRate = interestRate
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	s.ctx = s.ctx.WithBlockTime(startTime.Add(time.Second * time.Duration(secondsToAccrue)))

	testKeeper := s.k
	estimatedTVV, err := testKeeper.EstimateTotalVaultValue(s.ctx, vault)

	s.Require().NoError(err, "estimation should not error with uylds.fcc underlying")

	baseAmt := math.NewInt(150)
	expectedTotalAmount, err := expectedWithSimpleAPY(baseAmt, interestRate, secondsToAccrue)
	s.Require().NoError(err, "calculating expected APY should not fail")

	expectedCoin := sdk.NewCoin(underlyingDenom, expectedTotalAmount)
	s.Require().Equal(
		expectedCoin,
		estimatedTVV,
		"estimated TVV should sum assets at 1:1 and add accrued interest",
	)
}

func (s *TestSuite) TestEstimateTotalVaultValue_MultiAsset_UnderlyingIsFcc_WithNegativeInterest() {
	underlyingDenom := "uylds.fcc"
	paymentDenom := "usdc"
	shareDenom := "vsharefcc"

	const interestRate = "-0.1"
	const secondsToAccrue = int64(60 * 60 * 24 * 30)

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 1_000_000), s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 1_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, underlyingDenom, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 110)))
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, paymentDenom, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 50)))

	vault, err := s.k.CreateVault(s.ctx, vaultAttrs{admin: s.adminAddr.String(), share: shareDenom, underlying: underlyingDenom})
	s.Require().NoError(err, "vault creation should succeed")
	vault.PaymentDenom = paymentDenom

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
		sdk.NewInt64Coin(paymentDenom, 50),
	)), "funding principal should succeed")
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.GetAddress(), sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 10))), "funding reserves should succeed")

	startTime := s.ctx.BlockTime()
	vault.PeriodStart = startTime.Unix()
	vault.CurrentInterestRate = interestRate
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	s.ctx = s.ctx.WithBlockTime(startTime.Add(time.Second * time.Duration(secondsToAccrue)))

	testKeeper := s.k
	estimatedTVV, err := testKeeper.EstimateTotalVaultValue(s.ctx, vault)

	s.Require().NoError(err, "estimation should not error with uylds.fcc underlying")

	baseAmt := math.NewInt(150)
	expectedTotalAmount, err := expectedWithSimpleAPY(baseAmt, interestRate, secondsToAccrue)
	s.Require().NoError(err, "calculating expected APY should not fail")

	expectedCoin := sdk.NewCoin(underlyingDenom, expectedTotalAmount)
	s.Require().Equal(
		expectedCoin,
		estimatedTVV,
		"estimated TVV should sum assets at 1:1 and subtract negative interest",
	)
}

func (s *TestSuite) TestEstimateTotalVaultValue_MultiAsset_PaymentIsFcc() {
	underlyingDenom := "receipttoken"
	paymentDenom := "uylds.fcc"
	shareDenom := "vsharercpt"

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 1_000_000), s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 1_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, underlyingDenom, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 110)))
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, paymentDenom, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 50)))

	vault, err := s.k.CreateVault(
		s.ctx,
		vaultAttrs{admin: s.adminAddr.String(), share: shareDenom, underlying: underlyingDenom},
	)
	s.Require().NoError(err, "vault creation should succeed")
	vault.PaymentDenom = paymentDenom
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
		sdk.NewInt64Coin(paymentDenom, 50),
	)), "funding principal should succeed")
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.GetAddress(), sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 10))), "funding reserves should succeed")

	testKeeper := s.k
	estimatedTVV, err := testKeeper.EstimateTotalVaultValue(s.ctx, vault)

	s.Require().NoError(err, "estimation should not error with uylds.fcc payment")
	expectedCoin := sdk.NewInt64Coin(underlyingDenom, 150)
	s.Require().Equal(
		expectedCoin,
		estimatedTVV,
		"estimated TVV should sum assets at 1:1",
	)
}

func (s *TestSuite) TestEstimateTotalVaultValue_MultiAsset_PaymentIsFcc_WithInterest() {
	underlyingDenom := "receipttoken"
	paymentDenom := "uylds.fcc"
	shareDenom := "vsharercpt"

	const interestRate = "0.1"
	const secondsToAccrue = int64(60 * 60 * 24 * 30)

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 1_000_000), s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 1_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, underlyingDenom, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 110)))
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, paymentDenom, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 50)))

	vault, err := s.k.CreateVault(
		s.ctx,
		vaultAttrs{admin: s.adminAddr.String(), share: shareDenom, underlying: underlyingDenom},
	)
	s.Require().NoError(err, "vault creation should succeed")
	vault.PaymentDenom = paymentDenom

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
		sdk.NewInt64Coin(paymentDenom, 50),
	)), "funding principal should succeed")
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.GetAddress(), sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 10))), "funding reserves should succeed")

	startTime := s.ctx.BlockTime()
	vault.PeriodStart = startTime.Unix()
	vault.CurrentInterestRate = interestRate
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	s.ctx = s.ctx.WithBlockTime(startTime.Add(time.Second * time.Duration(secondsToAccrue)))

	testKeeper := s.k
	estimatedTVV, err := testKeeper.EstimateTotalVaultValue(s.ctx, vault)

	s.Require().NoError(err, "estimation should not error with uylds.fcc payment")

	baseAmt := math.NewInt(150)
	expectedTotalAmount, err := expectedWithSimpleAPY(baseAmt, interestRate, secondsToAccrue)
	s.Require().NoError(err, "calculating expected APY should not fail")

	expectedCoin := sdk.NewCoin(underlyingDenom, expectedTotalAmount)
	s.Require().Equal(
		expectedCoin,
		estimatedTVV,
		"estimated TVV should sum assets at 1:1 and add accrued interest",
	)
}

func (s *TestSuite) TestEstimateTotalVaultValue_MultiAsset_PaymentIsFcc_WithNegativeInterest() {
	underlyingDenom := "receipttoken"
	paymentDenom := "uylds.fcc"
	shareDenom := "vsharercpt"

	const interestRate = "-0.1"
	const secondsToAccrue = int64(60 * 60 * 24 * 30)

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 1_000_000), s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 1_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, underlyingDenom, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 110)))
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, paymentDenom, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 50)))

	vault, err := s.k.CreateVault(
		s.ctx,
		vaultAttrs{admin: s.adminAddr.String(), share: shareDenom, underlying: underlyingDenom},
	)
	s.Require().NoError(err, "vault creation should succeed")
	vault.PaymentDenom = paymentDenom

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
		sdk.NewInt64Coin(paymentDenom, 50),
	)), "funding principal should succeed")
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.GetAddress(), sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 10))), "funding reserves should succeed")

	startTime := s.ctx.BlockTime()
	vault.PeriodStart = startTime.Unix()
	vault.CurrentInterestRate = interestRate
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	s.ctx = s.ctx.WithBlockTime(startTime.Add(time.Second * time.Duration(secondsToAccrue)))

	testKeeper := s.k
	estimatedTVV, err := testKeeper.EstimateTotalVaultValue(s.ctx, vault)

	s.Require().NoError(err, "estimation should not error with uylds.fcc payment")

	baseAmt := math.NewInt(150)
	expectedTotalAmount, err := expectedWithSimpleAPY(baseAmt, interestRate, secondsToAccrue)
	s.Require().NoError(err, "calculating expected APY should not fail")

	expectedCoin := sdk.NewCoin(underlyingDenom, expectedTotalAmount)
	s.Require().Equal(
		expectedCoin,
		estimatedTVV,
		"estimated TVV should sum assets at 1:1 and subtract negative interest",
	)
}

func (s *TestSuite) TestEstimateTotalVaultValue_MultiAsset_WithNAV() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom, 1, 2)

	const interestRate = "0.1"
	const secondsToAccrue = int64(60 * 60 * 24 * 30)

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
		sdk.NewInt64Coin(paymentDenom, 50),
	)), "funding principal account should succeed")
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.GetAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 10),
	)), "funding vault account (reserves) should succeed")

	startTime := s.ctx.BlockTime()
	vault.PeriodStart = startTime.Unix()
	vault.CurrentInterestRate = interestRate
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	s.ctx = s.ctx.WithBlockTime(startTime.Add(time.Second * time.Duration(secondsToAccrue)))

	testKeeper := s.k
	estimatedTVV, err := testKeeper.EstimateTotalVaultValue(s.ctx, vault)

	s.Require().NoError(err, "EstimateTotalVaultValue should not error during NAV conversion")

	baseAmt := math.NewInt(125)
	expectedTotalAmount, err := expectedWithSimpleAPY(baseAmt, interestRate, secondsToAccrue)
	s.Require().NoError(err, "calculating expected APY should not fail")

	expectedCoin := sdk.NewCoin(underlyingDenom, expectedTotalAmount)
	s.Require().Equal(expectedCoin, estimatedTVV, "estimated TVV should equal base principal (with NAV) plus accrued interest")
}

func (s *TestSuite) TestEstimateTotalVaultValue_MultiAsset_WithNAV_WithNegativeInterest() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom, 1, 2)

	const interestRate = "-0.1"
	const secondsToAccrue = int64(60 * 60 * 24 * 30)

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
		sdk.NewInt64Coin(paymentDenom, 50),
	)), "funding principal account should succeed")
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.GetAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 10),
	)), "funding vault account (reserves) should succeed")

	startTime := s.ctx.BlockTime()
	vault.PeriodStart = startTime.Unix()
	vault.CurrentInterestRate = interestRate
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	s.ctx = s.ctx.WithBlockTime(startTime.Add(time.Second * time.Duration(secondsToAccrue)))

	testKeeper := s.k
	estimatedTVV, err := testKeeper.EstimateTotalVaultValue(s.ctx, vault)

	s.Require().NoError(err, "EstimateTotalVaultValue should not error during NAV conversion")

	baseAmt := math.NewInt(125)
	expectedTotalAmount, err := expectedWithSimpleAPY(baseAmt, interestRate, secondsToAccrue)
	s.Require().NoError(err, "calculating expected APY should not fail")

	expectedCoin := sdk.NewCoin(underlyingDenom, expectedTotalAmount)
	s.Require().Equal(expectedCoin, estimatedTVV, "estimated TVV should equal base principal (with NAV) and subtract negative interest")
}

func (s *TestSuite) TestEstimateTotalVaultValue_ErrorPropagation() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(paymentDenom, 1_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, paymentDenom, sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 10)))

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(paymentDenom, 10),
	)), "funding principal should succeed")

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	_, err := testKeeper.EstimateTotalVaultValue(s.ctx, vault)

	s.Require().Error(err, "estimation should error if GetTVV errors")
	s.Require().Contains(err.Error(), "get tvv: nav not found for usdc/ylds", "error should propagate from missing NAV")
}
