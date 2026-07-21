package keeper_test

import (
	"fmt"
	"time"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/google/uuid"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
	metadatatypes "github.com/provenance-io/provenance/x/metadata/types"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
)

func (s *TestSuite) TestUnitPriceFraction_Table() {
	underlyingDenom := "underlying"
	heldDenom := "usdc"
	shareDenom := "vshare"
	heldAsset := metadatatypes.ScopeMetadataAddress(uuid.MustParse("00000000-0000-4000-8000-0000000000e5")).Denom()
	vault := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)

	cases := []struct {
		name                  string
		fromDenom             string
		setup                 func()
		expectedNumerator     int64
		expectedDenominator   int64
		expectedErrorContains string
		expectedSentinel      error
	}{
		{
			name:                "source denom equals underlying returns identity fraction without a lookup",
			fromDenom:           underlyingDenom,
			expectedNumerator:   1,
			expectedDenominator: 1,
		},
		{
			name:                "held denom converts to underlying via its internal NAV",
			fromDenom:           heldDenom,
			expectedNumerator:   1,
			expectedDenominator: 2,
		},
		{
			name:                  "missing internal NAV for denom returns not-found error",
			fromDenom:             "unknown",
			expectedErrorContains: "no internal NAV entry for denom",
			expectedSentinel:      keeper.ErrInternalNAVNotFound,
		},
		{
			// A multi-hop price chain (heldAsset -> heldDenom -> underlying) can only
			// exist in state written outside SetVaultNAV's validation (which forces every
			// price denom to be the underlying), e.g. a migration or a direct write. The
			// engine still walks such a chain, so seed both legs directly to exercise it.
			name:      "held asset chains through an intermediate denom NAV to underlying",
			fromDenom: heldAsset,
			setup: func() {
				s.bumpHeight()
				s.Require().NoError(s.k.NAVs.Set(
					s.ctx,
					collections.Join(vault.GetAddress(), heldDenom),
					types.VaultNAV{Denom: heldDenom, Price: sdk.NewInt64Coin(underlyingDenom, 1), Volume: math.NewInt(2)},
				), "should write the intermediate leg of the price chain directly to storage")
				s.Require().NoError(s.k.NAVs.Set(
					s.ctx,
					collections.Join(vault.GetAddress(), heldAsset),
					types.VaultNAV{Denom: heldAsset, Price: sdk.NewInt64Coin(heldDenom, 6), Volume: math.NewInt(1)},
				), "should write the held-asset leg of the price chain directly to storage")
			},
			expectedNumerator:   6,
			expectedDenominator: 2,
		},
		{
			name:      "overwritten internal NAV uses the latest entry",
			fromDenom: heldDenom,
			setup: func() {
				s.bumpHeight()
				s.setVaultNAV(vault, heldDenom, sdk.NewInt64Coin(underlyingDenom, 5), 7)
			},
			expectedNumerator:   5,
			expectedDenominator: 7,
		},
		{
			// Defensive guard: write-time validation should reject a non-positive
			// volume, but UnitPriceFraction also defends against it in case state
			// is ever corrupted (e.g. by a future migration). Bypass write-side
			// validation by writing directly to the NAVs collection.
			name:      "non-positive internal NAV volume is rejected by the defensive guard",
			fromDenom: heldDenom,
			setup: func() {
				s.bumpHeight()
				s.Require().NoError(s.k.NAVs.Set(
					s.ctx,
					collections.Join(vault.GetAddress(), heldDenom),
					types.VaultNAV{
						Denom:  heldDenom,
						Price:  sdk.NewInt64Coin(underlyingDenom, 1),
						Volume: math.ZeroInt(),
					},
				), "should write a zero-volume NAV directly to storage to exercise the defensive guard")
			},
			expectedErrorContains: "internal NAV volume must be positive",
		},
		{
			// A zero price is a legitimate write-down of a held asset to zero. The
			// engine must accept it and yield a zero unit price rather than erroring.
			name:      "zero internal NAV price yields a zero unit price",
			fromDenom: heldDenom,
			setup: func() {
				s.bumpHeight()
				s.Require().NoError(s.k.NAVs.Set(
					s.ctx,
					collections.Join(vault.GetAddress(), heldDenom),
					types.VaultNAV{
						Denom:  heldDenom,
						Price:  sdk.NewInt64Coin(underlyingDenom, 0),
						Volume: math.NewInt(1),
					},
				), "should write a zero-price NAV directly to storage to exercise the zero-value path")
			},
			expectedNumerator:   0,
			expectedDenominator: 1,
		},
		{
			// Defensive guard: a negative price can only arise from corrupted state
			// and must be rejected.
			name:      "negative internal NAV price is rejected by the defensive guard",
			fromDenom: heldDenom,
			setup: func() {
				s.bumpHeight()
				s.Require().NoError(s.k.NAVs.Set(
					s.ctx,
					collections.Join(vault.GetAddress(), heldDenom),
					types.VaultNAV{
						Denom:  heldDenom,
						Price:  sdk.Coin{Denom: underlyingDenom, Amount: math.NewInt(-1)},
						Volume: math.NewInt(1),
					},
				), "should write a negative-price NAV directly to storage to exercise the defensive guard")
			},
			expectedErrorContains: "internal NAV price must not be negative",
		},
		{
			// A nil price amount round-trips through storage as zero, so it is read
			// back as a legitimate zero-value NAV rather than tripping the guard.
			name:      "nil internal NAV price normalizes to zero through storage",
			fromDenom: heldDenom,
			setup: func() {
				s.bumpHeight()
				s.Require().NoError(s.k.NAVs.Set(
					s.ctx,
					collections.Join(vault.GetAddress(), heldDenom),
					types.VaultNAV{
						Denom:  heldDenom,
						Price:  sdk.Coin{Denom: underlyingDenom, Amount: math.Int{}},
						Volume: math.NewInt(1),
					},
				), "should write a nil-amount price NAV directly to storage")
			},
			expectedNumerator:   0,
			expectedDenominator: 1,
		},
		{
			name:      "self-priced denom seeded outside validation reports cycle instead of recursing without bound",
			fromDenom: "selfpriced",
			setup: func() {
				s.bumpHeight()
				s.Require().NoError(s.k.NAVs.Set(
					s.ctx,
					collections.Join(vault.GetAddress(), "selfpriced"),
					types.VaultNAV{
						Denom:  "selfpriced",
						Price:  sdk.NewInt64Coin("selfpriced", 1),
						Volume: math.NewInt(1),
					},
				), "should write a self-priced NAV directly to storage to exercise the cycle guard")
			},
			expectedErrorContains: "contains a cycle",
		},
		{
			name:      "two denom loop with a priced in b and b priced in a reports cycle instead of overflowing the stack",
			fromDenom: "cyclea",
			setup: func() {
				s.bumpHeight()
				s.Require().NoError(s.k.NAVs.Set(
					s.ctx,
					collections.Join(vault.GetAddress(), "cyclea"),
					types.VaultNAV{
						Denom:  "cyclea",
						Price:  sdk.NewInt64Coin("cycleb", 1),
						Volume: math.NewInt(1),
					},
				), "should write the first leg of the price cycle directly to storage")
				s.Require().NoError(s.k.NAVs.Set(
					s.ctx,
					collections.Join(vault.GetAddress(), "cycleb"),
					types.VaultNAV{
						Denom:  "cycleb",
						Price:  sdk.NewInt64Coin("cyclea", 1),
						Volume: math.NewInt(1),
					},
				), "should write the second leg of the price cycle directly to storage")
			},
			expectedErrorContains: "contains a cycle",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			if tc.setup != nil {
				tc.setup()
			}
			num, den, err := s.k.UnitPriceFraction(s.ctx, tc.fromDenom, *vault)
			if tc.expectedErrorContains != "" {
				s.Require().Error(err, "expected error for case %q", tc.name)
				s.Require().Contains(err.Error(), tc.expectedErrorContains, "error message mismatch for case %q", tc.name)
				if tc.expectedSentinel != nil {
					s.Require().ErrorIs(err, tc.expectedSentinel, "error should carry the typed sentinel for case %q", tc.name)
				}
				return
			}
			s.Require().NoError(err, "unexpected error for case %q", tc.name)
			s.Require().Equal(math.NewInt(tc.expectedNumerator), num, "numerator mismatch for case %q", tc.name)
			s.Require().Equal(math.NewInt(tc.expectedDenominator), den, "denominator mismatch for case %q", tc.name)
			s.Require().True(den.IsPositive(), "denominator must be positive for case %q", tc.name)
		})
	}
}

func (s *TestSuite) TestToUnderlyingAssetAmount() {
	underlyingDenom := "ylds"
	heldDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)

	testKeeper := s.k

	val, err := testKeeper.ToUnderlyingAssetAmount(s.ctx, *vault, sdk.NewInt64Coin(heldDenom, 4))
	s.Require().NoError(err, "to-underlying should succeed for valid NAV")
	s.Require().Equal(math.NewInt(2), val, "4 usdc at 1/2 should be 2 ylds")

	valFloor, err := testKeeper.ToUnderlyingAssetAmount(s.ctx, *vault, sdk.NewInt64Coin(heldDenom, 5))
	s.Require().NoError(err, "to-underlying with floor should succeed")
	s.Require().Equal(math.NewInt(2), valFloor, "5 usdc at 1/2 should floor to 2 ylds (not 2.5)")

	valZero, err := testKeeper.ToUnderlyingAssetAmount(s.ctx, *vault, sdk.NewInt64Coin(heldDenom, 0))
	s.Require().NoError(err, "to-underlying with zero amount should succeed")
	s.Require().Equal(math.ZeroInt(), valZero, "0 usdc should convert to 0 ylds")

	_, err = testKeeper.ToUnderlyingAssetAmount(s.ctx, *vault, sdk.NewInt64Coin("unknown", 5))
	s.Require().Error(err, "should error when NAV missing for input denom")
	s.Require().Contains(err.Error(), "no internal NAV entry for denom", "error should mention missing internal NAV")
}

func (s *TestSuite) TestNAVConversion_OversizedNAVReturnsErrorNotPanic() {
	vault, testKeeper, underlyingDenom, heldDenom := s.setupOversizedNAVVault()

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "valuing a held-asset balance in underlying overflows the forward NAV multiply",
			run: func() error {
				s.seedOversizedNAV(vault, heldDenom, underlyingDenom, oversizedNAVPrice(), math.OneInt())
				_, err := testKeeper.ToUnderlyingAssetAmount(s.ctx, *vault, sdk.NewCoin(heldDenom, oversizedNAVPrice()))
				return err
			},
		},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			err := tc.run()
			s.Require().Error(err, "oversized NAV must degrade to an error, not panic: %s", tc.name)
			s.Require().ErrorContains(err, "integer overflow", "error should originate from the SafeMul overflow guard: %s", tc.name)
		})
	}
}

func (s *TestSuite) TestToUnderlyingAssetAmount_IdentityFastPath() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	testKeeper := s.k
	inputAmount := int64(123456789)
	outAmount, err := testKeeper.ToUnderlyingAssetAmount(s.ctx, *vault, sdk.NewInt64Coin(underlyingDenom, inputAmount))
	s.Require().NoError(err, "identity conversion to underlying should not error for denom %s", underlyingDenom)
	s.Require().Equal(math.NewInt(inputAmount), outAmount, "identity conversion should preserve the input amount for denom %s", underlyingDenom)
}

func (s *TestSuite) TestGetTVV_ExcludesSharesAndSumsInAsset() {
	underlyingDenom := "ylds"
	heldDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)

	principalAddress := vault.PrincipalMarkerAddress()
	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, principalAddress, sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
		sdk.NewInt64Coin(heldDenom, 10),
	)), "should fund vault with underlying and held-asset coins")

	testKeeper := s.k
	totalVaultValueInAsset, err := testKeeper.GetTVV(s.ctx, *vault)
	s.Require().NoError(err, "get TVV should succeed")
	s.Require().Equal(math.NewInt(1005), totalVaultValueInAsset, "1000 ylds + 10 usdc at 1/2 should equal 1005 ylds")
}

func (s *TestSuite) TestGetTVV_EmptyAndSharesOnly() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	testKeeper := s.k
	tvvEmpty, err := testKeeper.GetTVV(s.ctx, *vault)
	s.Require().NoError(err, "get TVV of empty vault should succeed")
	s.Require().Equal(math.ZeroInt(), tvvEmpty, "TVV of empty vault should be zero")

	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewInt64Coin(shareDenom, 1000)), "should mint shares")
	vault.TotalShares = sdk.NewInt64Coin(shareDenom, 1000)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	tvvSharesOnly, err := testKeeper.GetTVV(s.ctx, *vault)
	s.Require().NoError(err, "get TVV of shares-only vault should succeed")
	s.Require().Equal(math.ZeroInt(), tvvSharesOnly, "TVV of shares-only vault should be zero")
}

func (s *TestSuite) TestGetTVV_AccumulatorOverflowReturnsErrorNotPanic() {
	vault, testKeeper, underlyingDenom, heldDenom := s.setupOversizedNAVVault()
	s.seedOversizedNAV(vault, heldDenom, underlyingDenom, maxValidNAVPrice(), math.OneInt())

	principalAddress := vault.PrincipalMarkerAddress()
	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, principalAddress, sdk.NewCoins(
		sdk.NewInt64Coin(heldDenom, 1),
	)), "funding principal with one held-asset unit should succeed")
	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, principalAddress, sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
	)), "funding principal with a small underlying balance should succeed")

	_, err := testKeeper.GetTVV(s.ctx, *vault)
	s.Require().Error(err, "summing balances past the 256-bit ceiling must degrade to an error, not panic")
	s.Require().ErrorContains(err, "integer overflow", "error should originate from the SafeAdd accumulator guard")
	s.Require().ErrorContains(err, "total vault value", "error should carry the accumulator's wrapping context")
}

func (s *TestSuite) TestGetTVV_IncludesUnderlyingPricedHeldAsset() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	heldAsset := metadatatypes.ScopeMetadataAddress(uuid.MustParse("00000000-0000-4000-8000-0000000000a1")).Denom()
	s.setVaultNAV(vault, heldAsset, sdk.NewInt64Coin(underlyingDenom, 3), 2)

	principal := vault.PrincipalMarkerAddress()
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, principal, sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
		sdk.NewInt64Coin(heldAsset, 10),
	)), "funding principal with underlying and a NAV-priced held asset should succeed")

	tvv, err := s.k.GetTVV(s.ctx, *vault)
	s.Require().NoError(err, "TVV should value the NAV-priced held asset without error")
	s.Require().Equal(math.NewInt(115), tvv, "TVV should be 100 underlying + floor(10 * 3 / 2) = 115")
}

func (s *TestSuite) TestGetTVV_SkipsHeldAssetWithoutNAV() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	heldAsset := metadatatypes.ScopeMetadataAddress(uuid.MustParse("00000000-0000-4000-8000-0000000000c3")).Denom()

	principal := vault.PrincipalMarkerAddress()
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, principal, sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
		sdk.NewInt64Coin(heldAsset, 10),
	)), "funding principal with underlying and a held asset lacking a NAV should succeed")

	tvv, err := s.k.GetTVV(s.ctx, *vault)
	s.Require().NoError(err, "a held asset with no internal NAV must be skipped without failing TVV")
	s.Require().Equal(math.NewInt(100), tvv, "TVV should count only the 100 underlying and ignore the un-priced held asset")
}

func (s *TestSuite) TestGetTVV_UnvaluedPrincipalDenomsDoNotChangeValue() {
	underlyingDenom := "ylds"
	heldDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)

	principal := vault.PrincipalMarkerAddress()
	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, principal, sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
		sdk.NewInt64Coin(heldDenom, 10),
	)), "funding the principal with the valued underlying and held-asset balances should succeed")

	spy := &countingBankKeeper{BankKeeper: s.k.BankKeeper}
	spyKeeper := s.k
	spyKeeper.BankKeeper = spy

	valuedOnlyTVV, err := spyKeeper.GetTVV(s.ctx, *vault)
	s.Require().NoError(err, "computing TVV over only the valued balances should succeed")
	s.Require().Equal(math.NewInt(1005), valuedOnlyTVV, "TVV of the valued balances should be 1000 underlying + floor(10 usdc * 1/2) = 1005")
	s.Require().Zero(spy.getAllBalancesCalls, "valuing only the funded balances must not invoke the unbounded GetAllBalances walk")
	valuedOnlyBalanceLookups := spy.getBalanceCalls

	const parkedUnvaluedDenoms = 25
	for i := range parkedUnvaluedDenoms {
		junkDenom := fmt.Sprintf("junk%d", i)
		s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, principal,
			sdk.NewCoins(sdk.NewInt64Coin(junkDenom, int64(100+i)))),
			"parking unvalued denom %q at the principal should succeed", junkDenom)
	}

	spy.getAllBalancesCalls = 0
	spy.getBalanceCalls = 0

	withUnvaluedTVV, err := spyKeeper.GetTVV(s.ctx, *vault)
	s.Require().NoError(err, "computing TVV after parking unvalued denoms should succeed")
	s.Require().Equal(valuedOnlyTVV, withUnvaluedTVV,
		"parking %d unvalued (no-NAV) denoms at the principal must not change TVV", parkedUnvaluedDenoms)
	s.Require().Zero(spy.getAllBalancesCalls,
		"GetTVV must value denoms from the NAV table, never the unbounded GetAllBalances walk")
	s.Require().Equal(valuedOnlyBalanceLookups, spy.getBalanceCalls,
		"balance lookups must stay bounded to the valued denoms and not scale with the %d parked unvalued denoms", parkedUnvaluedDenoms)
}

func (s *TestSuite) TestGetTVV_InterestAndFeeAccrueOnHeldAssetBase() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)
	vault.AumFeeBips = 100
	s.SetVaultRatesAndPeriod(vault, "0.10", "0.10", 1, 0)
	vault.PeriodStart = 1

	heldAsset := metadatatypes.ScopeMetadataAddress(uuid.MustParse("00000000-0000-4000-8000-0000000000d4")).Denom()
	s.setVaultNAV(vault, heldAsset, sdk.NewInt64Coin(underlyingDenom, 1), 1)

	principal := vault.PrincipalMarkerAddress()
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, principal,
		sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 1_000_000))), "funding principal with underlying should succeed")

	baseWithoutAsset, err := s.k.GetTVV(s.ctx, *vault)
	s.Require().NoError(err, "computing the underlying-only TVV base should succeed")
	interestWithoutAsset, err := s.k.CalculateAccruedInterest(s.ctx, *vault, sdk.NewCoin(underlyingDenom, baseWithoutAsset))
	s.Require().NoError(err, "computing interest on the underlying-only base should succeed")
	feeWithoutAsset, err := s.k.CalculateAccruedAUMFee(s.ctx, *vault, baseWithoutAsset)
	s.Require().NoError(err, "computing the AUM fee on the underlying-only base should succeed")

	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, principal,
		sdk.NewCoins(sdk.NewInt64Coin(heldAsset, 500_000))), "funding principal with a NAV-priced held asset should succeed")

	baseWithAsset, err := s.k.GetTVV(s.ctx, *vault)
	s.Require().NoError(err, "computing the TVV base including the held asset should succeed")
	interestWithAsset, err := s.k.CalculateAccruedInterest(s.ctx, *vault, sdk.NewCoin(underlyingDenom, baseWithAsset))
	s.Require().NoError(err, "computing interest on the larger base should succeed")
	feeWithAsset, err := s.k.CalculateAccruedAUMFee(s.ctx, *vault, baseWithAsset)
	s.Require().NoError(err, "computing the AUM fee on the larger base should succeed")

	s.Require().Equal(math.NewInt(1_500_000), baseWithAsset, "TVV base should grow by the held asset's NAV value (1_000_000 + 500_000)")
	s.Require().True(baseWithAsset.GT(baseWithoutAsset), "held asset should enlarge the TVV base")
	s.Require().True(interestWithAsset.GT(interestWithoutAsset), "interest should accrue on the larger base that includes the held asset")
	s.Require().True(feeWithAsset.GT(feeWithoutAsset), "the AUM fee should accrue on the larger base that includes the held asset")
}

func (s *TestSuite) TestGetTVV_ExcludesReserves() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.GetAddress(), sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 999))),
		"sending funds to the vault account (reserves) should succeed")

	testKeeper := s.k
	tvv, err := testKeeper.GetTVV(s.ctx, *vault)
	s.Require().NoError(err, "computing TVV should not error when only reserves are funded")
	s.Require().True(tvv.IsZero(), "TVV should be zero because reserves are excluded and principal is unfunded")
}

func (s *TestSuite) TestGetNAVPerShare_FloorsToZeroForTinyPerShare() {
	underlyingDenom := "ylds"
	heldDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)

	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
		sdk.NewInt64Coin(heldDenom, 10),
	)), "should fund vault marker for NAV calc")

	totalVaultValueInAsset := math.NewInt(1005)
	shareSupplyMint := sdk.NewCoin(shareDenom, utils.ShareScalar.Mul(totalVaultValueInAsset))
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), shareSupplyMint), "should mint share supply matching tvv*ShareScalar")
	vault.TotalShares = shareSupplyMint
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	testKeeper := s.k
	navPerShareAsset, err := testKeeper.GetNAVPerShare(s.ctx, *vault)
	s.Require().NoError(err, "nav per share should compute without error")
	s.Require().True(navPerShareAsset.IsZero(), "with scaled shares, integer NAV/share should floor to 0")
}

func (s *TestSuite) TestGetNAVPerShare_ZeroSupplyAndNormalNAV() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	testKeeper := s.k
	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
	)), "should fund vault for TVV")

	navPerShareZeroSupply, err := testKeeper.GetNAVPerShare(s.ctx, *vault)
	s.Require().NoError(err, "should not error with zero share supply")
	s.Require().Equal(math.ZeroInt(), navPerShareZeroSupply, "NAV per share should be zero with zero share supply")

	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewInt64Coin(shareDenom, 500)), "should mint shares")
	vault.TotalShares = sdk.NewInt64Coin(shareDenom, 500)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	navPerShareNormal, err := testKeeper.GetNAVPerShare(s.ctx, *vault)
	s.Require().NoError(err, "should compute normal NAV per share")
	s.Require().Equal(math.NewInt(2), navPerShareNormal, "1000 TVV / 500 shares should be 2 NAV per share")
}

func (s *TestSuite) TestConvertDepositToShares_ValuesHeldAssetsInTVV() {
	underlyingDenom := "ylds"
	heldDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)

	testKeeper := s.k
	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
		sdk.NewInt64Coin(heldDenom, 10),
	)), "should fund vault marker for TVV")

	tvv, err := testKeeper.GetTVV(s.ctx, *vault)
	s.Require().NoError(err, "should compute TVV")
	s.Require().Equal(math.NewInt(1005), tvv, "TVV should value the held-asset balance through its NAV")
	initialShares := sdk.NewCoin(shareDenom, tvv.Mul(utils.ShareScalar))
	s.Require().NoError(
		s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), initialShares),
		"should mint initial shares to match TVV",
	)
	vault.TotalShares = initialShares
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	mintedShares, err := testKeeper.ConvertDepositToShares(s.ctx, *vault, sdk.NewInt64Coin(underlyingDenom, 4))
	s.Require().NoError(err, "deposit conversion should succeed")
	s.Require().Equal(shareDenom, mintedShares.Denom, "minted shares denom should match vault share denom")
	s.Require().Equal(utils.ShareScalar.Mul(math.NewInt(4)), mintedShares.Amount, "4 underlying should mint 4*ShareScalar shares at parity against the NAV-valued TVV")
}

func (s *TestSuite) TestConvertDepositToShares_InitialDepositMintsAtParity() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	initialDepositShares, err := s.k.ConvertDepositToShares(s.ctx, *vault, sdk.NewInt64Coin(underlyingDenom, 100))
	s.Require().NoError(err, "initial deposit should succeed")
	s.Require().Equal(utils.ShareScalar.Mul(math.NewInt(100)), initialDepositShares.Amount, "initial deposit should mint shares at parity with ShareScalar")
}

func (s *TestSuite) TestConvertSharesToRedeemCoin_RedeemsInUnderlying() {
	underlyingDenom := "ylds"
	heldDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)

	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
		sdk.NewInt64Coin(heldDenom, 10),
	)), "should fund vault with underlying and held-asset coins")

	totalVaultValueInAsset := math.NewInt(1005)
	shareSupply := utils.ShareScalar.Mul(totalVaultValueInAsset)
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewCoin(shareDenom, shareSupply)), "should mint share supply equal to tvv*ShareScalar")
	vault.TotalShares = sdk.NewCoin(shareDenom, shareSupply)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	outAssetCoin, err := s.k.ConvertSharesToRedeemCoin(s.ctx, *vault, utils.ShareScalar)
	s.Require().NoError(err, "shares->underlying conversion should succeed")
	s.Require().Equal(underlyingDenom, outAssetCoin.Denom, "redeem coin should be denominated in the underlying asset")
	s.Require().Equal(math.NewInt(1), outAssetCoin.Amount, "ShareScalar shares should redeem to 1 underlying unit at parity, with the held-asset balance valued into TVV via its NAV")
}

func (s *TestSuite) TestConvertAndNav_PriceNetOfOutstandingAumFee() {
	underlyingDenom := "ylds"
	heldDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)

	testKeeper := s.k
	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
		sdk.NewInt64Coin(heldDenom, 10),
	)), "should fund vault marker for gross TVV")

	grossTVV, err := testKeeper.GetTVV(s.ctx, *vault)
	s.Require().NoError(err, "should compute gross TVV")
	s.Require().Equal(math.NewInt(1005), grossTVV, "gross TVV = 1000 underlying + 10 held asset at 1/2")

	totalShares := sdk.NewCoin(shareDenom, grossTVV.Mul(utils.ShareScalar))
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), totalShares), "should mint share supply")
	vault.TotalShares = totalShares
	vault.OutstandingAumFee = sdk.NewInt64Coin(underlyingDenom, 5)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	netTVV, err := testKeeper.GetNetTVV(s.ctx, *vault)
	s.Require().NoError(err, "should compute net TVV")
	s.Require().Equal(math.NewInt(1000), netTVV, "net TVV = gross 1005 minus outstanding fee of 5 underlying")

	deposit := sdk.NewInt64Coin(underlyingDenom, 1000)
	mintedShares, err := testKeeper.ConvertDepositToShares(s.ctx, *vault, deposit)
	s.Require().NoError(err, "deposit conversion should succeed")
	expectedNetMint, err := utils.CalculateSharesProRata(deposit.Amount, netTVV, totalShares.Amount, shareDenom)
	s.Require().NoError(err, "should compute net-priced deposit shares")
	expectedGrossMint, err := utils.CalculateSharesProRata(deposit.Amount, grossTVV, totalShares.Amount, shareDenom)
	s.Require().NoError(err, "should compute gross-priced deposit shares")
	s.Require().Equal(expectedNetMint.Amount, mintedShares.Amount, "deposit must be priced off net TVV")
	s.Require().True(mintedShares.Amount.GT(expectedGrossMint.Amount), "net pricing mints more shares per deposit than gross pricing would (gross overstates TVV)")

	redeemShares := grossTVV.Mul(utils.ShareScalar)
	redeemed, err := testKeeper.ConvertSharesToRedeemCoin(s.ctx, *vault, redeemShares)
	s.Require().NoError(err, "redeem conversion should succeed")
	expectedNetRedeem, err := utils.CalculateRedeemProRata(redeemShares, totalShares.Amount, netTVV, underlyingDenom)
	s.Require().NoError(err, "should compute net-priced redemption")
	expectedGrossRedeem, err := utils.CalculateRedeemProRata(redeemShares, totalShares.Amount, grossTVV, underlyingDenom)
	s.Require().NoError(err, "should compute gross-priced redemption")
	s.Require().Equal(expectedNetRedeem.Amount, redeemed.Amount, "redemption must be priced off net TVV")
	s.Require().True(redeemed.Amount.LT(expectedGrossRedeem.Amount), "net pricing pays out less per share than gross pricing would (gross overstates TVV)")
}

func (s *TestSuite) TestGetNetTVV_FloorsAtZeroWhenOutstandingExceedsGross() {
	underlyingDenom := "ylds"
	heldDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)

	testKeeper := s.k
	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
	)), "should fund vault marker for gross TVV")

	grossTVV, err := testKeeper.GetTVV(s.ctx, *vault)
	s.Require().NoError(err, "should compute gross TVV")
	s.Require().Equal(math.NewInt(100), grossTVV, "gross TVV should equal funded underlying balance")

	vault.OutstandingAumFee = sdk.NewInt64Coin(underlyingDenom, 1000)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	netTVV, err := testKeeper.GetNetTVV(s.ctx, *vault)
	s.Require().NoError(err, "net TVV should not error when outstanding fee exceeds gross TVV")
	s.Require().True(netTVV.IsZero(), "net TVV should floor at zero when outstanding fee (1000 underlying) exceeds gross TVV (100)")
}

func (s *TestSuite) TestConvertSharesToRedeemCoin_ZeroAndDustRedemption() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	testKeeper := s.k

	zeroCoin, err := testKeeper.ConvertSharesToRedeemCoin(s.ctx, *vault, math.ZeroInt())
	s.Require().NoError(err, "redeeming zero shares should not error")
	s.Require().True(zeroCoin.IsZero(), "redeeming zero shares should result in a zero coin")

	negativeCoin, err := testKeeper.ConvertSharesToRedeemCoin(s.ctx, *vault, math.NewInt(-100))
	s.Require().NoError(err, "redeeming negative shares should not error")
	s.Require().True(negativeCoin.IsZero(), "redeeming negative shares should result in a zero coin")

	tvv := math.NewInt(1000)
	totalShares := tvv.Mul(utils.ShareScalar)
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewCoin(shareDenom, totalShares)), "should mint total shares")
	vault.TotalShares = sdk.NewCoin(shareDenom, totalShares)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
	)), "should fund vault for TVV")
	dustCoin, err := testKeeper.ConvertSharesToRedeemCoin(s.ctx, *vault, math.NewInt(1))
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

	testKeeper := s.k
	redeemCoin, err := testKeeper.ConvertSharesToRedeemCoin(s.ctx, *vault, utils.ShareScalar)
	s.Require().NoError(err, "redeeming with TVV=0 and shares>0 should not error")
	s.Require().Equal(underlyingDenom, redeemCoin.Denom, "redeem denom should be the underlying asset")
	s.Require().True(redeemCoin.Amount.IsZero(), "redeemed amount should be zero when TVV is zero")
}

func (s *TestSuite) TestGetTVV_PausedUsesPausedBalance() {
	underlyingDenom := "ylds"
	heldDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)

	principal := vault.PrincipalMarkerAddress()
	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, principal, sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 9999),
		sdk.NewInt64Coin(heldDenom, 9999),
	)), "funding principal balances before pause should succeed")

	vault.Paused = true
	vault.PausedBalance = sdk.NewInt64Coin(underlyingDenom, 42)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	testKeeper := s.k
	tvv, err := testKeeper.GetTVV(s.ctx, *vault)
	s.Require().NoError(err, "GetTVV should not error when paused")
	s.Require().Equal(math.NewInt(42), tvv, "when paused, TVV should equal vault.PausedBalance.Amount regardless of principal contents")
}

func (s *TestSuite) TestGetNetTVV_PausedReturnsPausedBalanceWithoutNAV() {
	underlyingDenom := "ylds"
	heldDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)

	principal := vault.PrincipalMarkerAddress()
	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, principal, sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 9999),
		sdk.NewInt64Coin(heldDenom, 9999),
	)), "funding principal balances before pause should succeed")

	vault.Paused = true
	vault.PausedBalance = sdk.NewInt64Coin(underlyingDenom, 42)
	vault.OutstandingAumFee = sdk.NewInt64Coin("nonav", 100)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	testKeeper := keeper.Keeper{MarkerKeeper: s.k.MarkerKeeper, BankKeeper: s.k.BankKeeper}
	netTVV, err := testKeeper.GetNetTVV(s.ctx, *vault)
	s.Require().NoError(err, "paused net TVV must not require a NAV lookup for the outstanding fee")
	s.Require().Equal(math.NewInt(42), netTVV, "when paused, net TVV should equal vault.PausedBalance.Amount without subtracting the outstanding fee")
}

func (s *TestSuite) TestGetTVV_MixedPricedAndUnpricedHeldBalances() {
	underlyingDenom := "ylds"
	heldDenom := "usdc"
	unpricedDenom := "paymenow"
	shareDenom := "vshare"

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(heldDenom, 1_000_000), s.adminAddr)
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	initialShareSupply := sdk.NewCoin(shareDenom, utils.ShareScalar)
	s.Require().NoError(
		s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), initialShareSupply),
		"minting initial shares while TVV is zero should succeed",
	)
	vault.TotalShares = initialShareSupply
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	coinsToSend := sdk.NewCoins(
		sdk.NewInt64Coin(heldDenom, 100),
		sdk.NewInt64Coin(unpricedDenom, 50),
		sdk.NewInt64Coin(underlyingDenom, 5),
	)

	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, heldDenom, sdk.NewCoins(sdk.NewInt64Coin(heldDenom, 100)))
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(unpricedDenom, 1_000_000), s.adminAddr)
	s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, unpricedDenom, sdk.NewCoins(sdk.NewInt64Coin(unpricedDenom, 50)))

	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, vault.PrincipalMarkerAddress(), coinsToSend), "funding principal should succeed")

	s.setVaultNAV(vault, heldDenom, sdk.NewInt64Coin(underlyingDenom, 2), 1)

	testKeeper := s.k
	tvv, err := testKeeper.GetTVV(s.ctx, *vault)
	s.Require().NoError(err, "TVV calculation should succeed")

	expectedTVV := math.NewInt(205)

	s.Require().Equal(expectedTVV, tvv, "TVV should count the underlying and NAV-priced held balances while skipping the unpriced denom")
}

func (s *TestSuite) TestEstimateTotalVaultValue_Paused() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 9999),
	)), "funding principal should succeed")
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.GetAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
	)), "funding reserves should succeed")

	vault.Paused = true
	vault.PausedBalance = sdk.NewInt64Coin(underlyingDenom, 42)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	testKeeper := s.k
	estimatedTVV, err := testKeeper.EstimateTotalVaultValue(s.ctx, vault)

	s.Require().NoError(err, "estimation should not error when paused")
	s.Require().Equal(vault.PausedBalance, estimatedTVV, "estimated TVV should equal PausedBalance when vault is paused")
}

func (s *TestSuite) TestEstimateTotalVaultValue_SingleAsset() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1000),
	)), "funding principal should succeed")
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.GetAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
	)), "funding reserves should succeed")

	testKeeper := s.k
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

	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
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

	testKeeper := s.k
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

	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
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

	testKeeper := s.k
	estimatedTVV, err := testKeeper.EstimateTotalVaultValue(s.ctx, vault)

	s.Require().NoError(err, "estimation should not error for single asset")

	baseAmt := math.NewInt(1000)
	expectedTotalAmount, err := expectedWithSimpleAPY(baseAmt, interestRate, secondsToAccrue)
	s.Require().NoError(err, "calculating expected APY should not fail")

	expectedCoin := sdk.NewCoin(underlyingDenom, expectedTotalAmount)
	s.Require().Equal(expectedCoin, estimatedTVV, "estimated TVV should equal principal balance plus accrued interest")
}

// TestEstimateTotalVaultValue_MultiAsset_Table exercises EstimateTotalVaultValue
// for vaults that hold a mix of underlying + held denom in their principal
// balance, with the price of one heldDenom unit fixed at 1 underlying via
// Internal NAV (the previous uylds.fcc 1:1 peg is no longer special-cased).
//
// Each case funds the principal with 100 underlying + 50 held asset (= 150
// underlying after NAV conversion) and 10 underlying as reserves (excluded).
// When interestRate is non-empty, simple APY accrual over secondsToAccrue is
// applied on top of the base 150.
func (s *TestSuite) TestEstimateTotalVaultValue_MultiAsset_Table() {
	const secondsToAccrue = int64(60 * 60 * 24 * 30)

	cases := []struct {
		name            string
		underlyingDenom string
		heldDenom       string
		shareDenom      string
		interestRate    string
	}{
		{
			name:            "underlying is uylds.fcc, no interest, sums at 1:1",
			underlyingDenom: "uylds.fcc",
			heldDenom:       "usdc",
			shareDenom:      "vsharefcc",
		},
		{
			name:            "underlying is uylds.fcc, positive interest accrues",
			underlyingDenom: "uylds.fcc",
			heldDenom:       "usdc",
			shareDenom:      "vsharefcc",
			interestRate:    "0.1",
		},
		{
			name:            "underlying is uylds.fcc, negative interest accrues",
			underlyingDenom: "uylds.fcc",
			heldDenom:       "usdc",
			shareDenom:      "vsharefcc",
			interestRate:    "-0.1",
		},
		{
			name:            "held denom is uylds.fcc, no interest, sums at 1:1",
			underlyingDenom: "receipttoken",
			heldDenom:       "uylds.fcc",
			shareDenom:      "vsharercpt",
		},
		{
			name:            "held denom is uylds.fcc, positive interest accrues",
			underlyingDenom: "receipttoken",
			heldDenom:       "uylds.fcc",
			shareDenom:      "vsharercpt",
			interestRate:    "0.1",
		},
		{
			name:            "held denom is uylds.fcc, negative interest accrues",
			underlyingDenom: "receipttoken",
			heldDenom:       "uylds.fcc",
			shareDenom:      "vsharercpt",
			interestRate:    "-0.1",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()

			s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(tc.underlyingDenom, 1_000_000), s.adminAddr)
			s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(tc.heldDenom, 1_000_000), s.adminAddr)
			s.Require().NoError(
				s.k.MarkerKeeper.WithdrawCoins(
					s.ctx, s.adminAddr, s.adminAddr, tc.underlyingDenom,
					sdk.NewCoins(sdk.NewInt64Coin(tc.underlyingDenom, 110)),
				),
				"withdrawing underlying marker funds should succeed for case %q", tc.name,
			)
			s.Require().NoError(
				s.k.MarkerKeeper.WithdrawCoins(
					s.ctx, s.adminAddr, s.adminAddr, tc.heldDenom,
					sdk.NewCoins(sdk.NewInt64Coin(tc.heldDenom, 50)),
				),
				"withdrawing held-asset marker funds should succeed for case %q", tc.name,
			)

			vault, err := s.k.CreateVault(s.ctx, vaultAttrs{admin: s.adminAddr.String(), share: tc.shareDenom, underlying: tc.underlyingDenom})
			s.Require().NoError(err, "vault creation should succeed for case %q", tc.name)
			s.setVaultNAV(vault, tc.heldDenom, sdk.NewInt64Coin(tc.underlyingDenom, 1), 1)

			s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
				sdk.NewInt64Coin(tc.underlyingDenom, 100),
				sdk.NewInt64Coin(tc.heldDenom, 50),
			)), "funding principal should succeed for case %q", tc.name)
			s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.GetAddress(), sdk.NewCoins(
				sdk.NewInt64Coin(tc.underlyingDenom, 10),
			)), "funding reserves should succeed for case %q", tc.name)

			if tc.interestRate != "" {
				startTime := s.ctx.BlockTime()
				vault.PeriodStart = startTime.Unix()
				vault.CurrentInterestRate = tc.interestRate
				s.k.AuthKeeper.SetAccount(s.ctx, vault)
				s.ctx = s.ctx.WithBlockTime(startTime.Add(time.Second * time.Duration(secondsToAccrue)))
			}

			estimatedTVV, err := s.k.EstimateTotalVaultValue(s.ctx, vault)
			s.Require().NoError(err, "EstimateTotalVaultValue should not error for case %q", tc.name)

			baseAmt := math.NewInt(150)
			expectedAmount := baseAmt
			if tc.interestRate != "" {
				expectedAmount, err = expectedWithSimpleAPY(baseAmt, tc.interestRate, secondsToAccrue)
				s.Require().NoError(err, "expectedWithSimpleAPY should not fail for case %q", tc.name)
			}

			expectedCoin := sdk.NewCoin(tc.underlyingDenom, expectedAmount)
			s.Require().Equal(expectedCoin, estimatedTVV,
				"estimated TVV mismatch for case %q (expected %s)", tc.name, expectedCoin)
		})
	}
}

func (s *TestSuite) TestEstimateTotalVaultValue_MultiAsset_WithNAV() {
	underlyingDenom := "ylds"
	heldDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)

	const interestRate = "0.1"
	const secondsToAccrue = int64(60 * 60 * 24 * 30)

	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
		sdk.NewInt64Coin(heldDenom, 50),
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
	heldDenom := "usdc"
	shareDenom := "vshare"
	vault := s.setupHeldAssetVault(underlyingDenom, shareDenom, heldDenom, 1, 2)

	const interestRate = "-0.1"
	const secondsToAccrue = int64(60 * 60 * 24 * 30)

	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 100),
		sdk.NewInt64Coin(heldDenom, 50),
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

func (s *TestSuite) TestEstimateTotalVaultValue_FullScenario() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	// Setup: 1000 principal, 50 outstanding fee, 10% rate.
	s.Require().NoError(s.k.BankKeeper.SendCoins(markertypes.WithBypass(s.ctx), s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 1_000),
	)))

	startTime := s.ctx.BlockTime()
	vault.PeriodStart = startTime.Unix()
	vault.FeePeriodStart = startTime.Unix()
	vault.CurrentInterestRate = "0.1"
	vault.OutstandingAumFee = sdk.NewInt64Coin(underlyingDenom, 50)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	// Advance 1 year. Expected (Continuous Compounding):
	// Principal (1000) + Accrued Interest (105) - Accrued Fee (1) - Outstanding Fee (50) = 1054.
	s.ctx = s.ctx.WithBlockTime(startTime.Add(time.Second * 31_536_000))

	estimatedTVV, err := s.k.EstimateTotalVaultValue(s.ctx, vault)
	s.Require().NoError(err)
	s.Require().Equal(sdk.NewInt64Coin(underlyingDenom, 1_054), estimatedTVV)
}
