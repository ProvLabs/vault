package keeper_test

import (
	"fmt"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	attrtypes "github.com/provenance-io/provenance/x/attribute/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
)

// TestKeeperTestSuite is handled in suite_test.go

func (s *TestSuite) TestCreateVault_Success() {
	share := "vaultshare"
	base := "undercoin"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(base, 1_000_000), s.adminAddr)

	attrs := vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      share,
		underlying: base,
	}

	vault, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().NoError(err, "CreateVault should succeed for valid attributes")
	s.Require().Equal(attrs.admin, vault.Admin, "vault admin should match request")
	s.Require().Equal(attrs.share, vault.TotalShares.Denom, "vault share denom should match request")
	s.Require().Equal(attrs.underlying, vault.UnderlyingAsset, "vault underlying asset should match request")

	addr := types.GetVaultAddress(share)
	stored, err := s.k.GetVault(s.ctx, addr)
	s.Require().NoError(err, "GetVault should succeed for existing vault")
	s.Require().Equal(vault.Address, stored.Address, "stored vault address should match created vault address")
}

func (s *TestSuite) TestCreateVault_AssetMarkerMissing() {
	share := "vaultshare"
	base := "missingasset"

	attrs := vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      share,
		underlying: base,
	}

	_, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().ErrorContains(err, "underlying asset marker")
}

func (s *TestSuite) TestCreateVault_DuplicateMarkerFails() {
	denom := "dupecoin"
	base := "basecoin"

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(base, 1000), s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(denom, 1), s.adminAddr)

	attrs := vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      denom,
		underlying: base,
	}

	_, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().Error(err, "CreateVault should fail for duplicate marker")
	s.Require().ErrorContains(err, "already exists", "error message should mention duplicate marker")
}

func (s *TestSuite) TestCreateVault_InvalidDenomFails() {
	attrs := vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      "!!bad!!",
		underlying: "under",
	}
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(attrs.underlying, 1000), s.adminAddr)

	_, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().Error(err, "CreateVault should fail for invalid share denom")
	s.Require().ErrorContains(err, "invalid denom", "error message should mention invalid denom")
}

func (s *TestSuite) TestCreateVault_InvalidAdminFails() {
	share := "vaultx"
	base := "basecoin"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(base, 500), s.adminAddr)

	attrs := vaultAttrs{
		admin:      "not-a-valid-bech32",
		share:      share,
		underlying: base,
	}

	_, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().Error(err, "CreateVault should fail for invalid admin address")
	s.Require().ErrorContains(err, "invalid admin address", "error message should mention invalid admin address")
}

// TestCreateVault_DistinctPaymentDenomSeedsNAV verifies that creating a vault
// with payment_denom != underlying_asset persists the bootstrap NAV entry in
// the same atomic operation, so the valuation engine can convert payment-denom
// balances without an out-of-band setup step.
func (s *TestSuite) TestCreateVault_DistinctPaymentDenomSeedsNAV() {
	share := "vshare.bootstrap"
	underlying := "ulying"
	payment := "upay"

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 1_000_000), s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(payment, 1_000_000), s.adminAddr)

	attrs := vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      share,
		underlying: underlying,
		payment:    payment,
		initialPaymentNav: &types.InitialVaultNAV{
			Price:  sdk.NewInt64Coin(underlying, 3),
			Volume: math.NewInt(2),
			Source: "create-bootstrap",
		},
	}

	vault, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().NoError(err, "CreateVault should succeed when initial payment NAV is supplied")

	stored, err := s.k.GetVaultNAV(s.ctx, vault.GetAddress(), payment)
	s.Require().NoError(err, "bootstrap NAV should be persisted for payment denom %q", payment)
	s.Require().Equal(payment, stored.Denom, "stored NAV denom should be payment denom")
	s.Require().Equal(sdk.NewInt64Coin(underlying, 3), stored.Price, "stored NAV price should match supplied price")
	s.Require().Equal(math.NewInt(2), stored.Volume, "stored NAV volume should match supplied volume")
	s.Require().Equal("create-bootstrap", stored.Source, "stored NAV source should match supplied attribution")
}

// TestCreateVault_MissingInitialPaymentNAVFails verifies that creating a vault
// with payment_denom != underlying_asset fails when the bootstrap NAV is
// omitted. The check is enforced at ValidateBasic so an operator cannot stand
// up a vault that would silently stall fee collection and payment-denom
// swap-outs.
func (s *TestSuite) TestCreateVault_MissingInitialPaymentNAVFails() {
	share := "vshare.missing.nav"
	underlying := "ulying"
	payment := "upay"

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 1_000_000), s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(payment, 1_000_000), s.adminAddr)

	_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           s.adminAddr.String(),
		ShareDenom:      share,
		UnderlyingAsset: underlying,
		PaymentDenom:    payment,
	})
	s.Require().Error(err, "CreateVault should reject a vault with mismatched denoms and no initial NAV")
	s.Require().ErrorContains(err, "initial_payment_nav is required", "error should call out missing bootstrap NAV")
}

// TestCreateVault_InitialPaymentNAVRolledBackOnFailure verifies that the
// bootstrap NAV is written under the same cache context as the rest of vault
// creation: a downstream failure (here, the share-denom marker already exists)
// rolls back the NAV alongside the vault account.
func (s *TestSuite) TestCreateVault_InitialPaymentNAVRolledBackOnFailure() {
	share := "vshare.rollback"
	underlying := "ulying"
	payment := "upay"

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 1_000_000), s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(payment, 1_000_000), s.adminAddr)
	// Pre-create a marker at the share denom so createVaultMarker fails.
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(share, 1), s.adminAddr)

	vaultAddr := types.GetVaultAddress(share)
	_, err := s.k.CreateVault(s.ctx, &types.MsgCreateVaultRequest{
		Admin:           s.adminAddr.String(),
		ShareDenom:      share,
		UnderlyingAsset: underlying,
		PaymentDenom:    payment,
		InitialPaymentNav: &types.InitialVaultNAV{
			Price:  sdk.NewInt64Coin(underlying, 1),
			Volume: math.OneInt(),
		},
	})
	s.Require().Error(err, "CreateVault should fail when the share marker already exists")

	_, navErr := s.k.GetVaultNAV(s.ctx, vaultAddr, payment)
	s.Require().Error(navErr, "bootstrap NAV must not be persisted when create fails")
}

func (s *TestSuite) TestSwapIn_MultiAsset() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	unacceptedDenom := "junk"
	shareDenom := "vshare"
	vault := s.setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom, 1, 2)
	vault.SwapInEnabled = true
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	depositorAddr := s.CreateAndFundAccount(sdk.NewInt64Coin(paymentDenom, 1000))
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, depositorAddr, sdk.NewCoins(sdk.NewInt64Coin(unacceptedDenom, 1000))), "should fund depositor with unaccepted denom")

	totalShares := sdk.NewCoin(shareDenom, utils.ShareScalar.MulRaw(1000))
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 1000))), "should fund vault principal with initial TVV")
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), totalShares), "should mint initial share supply")
	vault.TotalShares = totalShares
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	depositCoin := sdk.NewInt64Coin(paymentDenom, 10)
	mintedShares, err := s.k.SwapIn(s.ctx, vault.GetAddress(), depositorAddr, depositCoin)
	s.Require().NoError(err, "should successfully swap in an accepted payment denom")

	expectedShares := utils.ShareScalar.MulRaw(5)
	s.Require().Equal(expectedShares, mintedShares.Amount, "minted shares should be proportional to the payment denom's value in the underlying asset")
	s.assertBalance(depositorAddr, shareDenom, expectedShares)
	s.assertBalance(vault.PrincipalMarkerAddress(), paymentDenom, math.NewInt(10))

	unacceptedCoin := sdk.NewInt64Coin(unacceptedDenom, 50)
	_, err = s.k.SwapIn(s.ctx, vault.GetAddress(), depositorAddr, unacceptedCoin)
	s.Require().Error(err, "should fail to swap in an unaccepted asset")
	s.Require().ErrorContains(err, "denom not supported for vault", "error should indicate the denom is not accepted")
}

func (s *TestSuite) TestSwapIn_Failures() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)
	depositorAddr := s.CreateAndFundAccount(sdk.NewInt64Coin(underlyingDenom, 100))

	vault.SwapInEnabled = false
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	_, err := s.k.SwapIn(s.ctx, vault.GetAddress(), depositorAddr, sdk.NewInt64Coin(underlyingDenom, 10))
	s.Require().Error(err, "swap in should fail when disabled")
	s.Require().ErrorContains(err, "swaps are not enabled", "error should mention swaps are disabled")

	vault.SwapInEnabled = true
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	_, err = s.k.SwapIn(s.ctx, vault.GetAddress(), depositorAddr, sdk.NewInt64Coin(underlyingDenom, 101))
	s.Require().Error(err, "swap in should fail with insufficient funds")
	s.Require().ErrorContains(err, "insufficient funds", "error should mention insufficient funds")
}

func (s *TestSuite) TestSwapIn_ZeroShareDeposit() {
	tests := []struct {
		name                string
		totalShares         math.Int
		principalBacking    int64
		deposit             int64
		expectedShares      math.Int
		expectedErrContains string
	}{
		{
			name:                "deposit floors to zero shares, should be rejected with no funds moved",
			totalShares:         math.OneInt(),
			principalBacking:    1_500_000,
			deposit:             1,
			expectedErrContains: "too small to mint shares",
		},
		{
			name:             "deposit just above the zero-share boundary, should mint a share",
			totalShares:      math.OneInt(),
			principalBacking: 1_500_000,
			deposit:          2,
			expectedShares:   math.OneInt(),
		},
	}

	for i, tc := range tests {
		s.Run(tc.name, func() {
			underlyingDenom := fmt.Sprintf("ylds%d", i)
			shareDenom := fmt.Sprintf("vshare%d", i)
			depositorFunding := math.NewInt(100)

			vault := s.setupBaseVault(underlyingDenom, shareDenom)
			vault.SwapInEnabled = true
			vault.TotalShares = sdk.NewCoin(shareDenom, tc.totalShares)
			s.k.AuthKeeper.SetAccount(s.ctx, vault)

			s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewCoin(shareDenom, tc.totalShares)),
				"should mint initial share supply %s%s", tc.totalShares, shareDenom)
			s.Require().NoError(s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, s.adminAddr, underlyingDenom, sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, tc.principalBacking))),
				"should withdraw %d%s of backing to the admin", tc.principalBacking, underlyingDenom)
			s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, tc.principalBacking))),
				"should fund vault principal with %d%s of backing", tc.principalBacking, underlyingDenom)

			depositorAddr := s.CreateAndFundAccount(sdk.NewCoin(underlyingDenom, depositorFunding))
			depositCoin := sdk.NewInt64Coin(underlyingDenom, tc.deposit)

			mintedShares, err := s.k.SwapIn(s.ctx, vault.GetAddress(), depositorAddr, depositCoin)

			if tc.expectedErrContains != "" {
				s.Require().Error(err, "swap in of %s should be rejected when it converts to zero shares", depositCoin)
				s.Require().ErrorContains(err, tc.expectedErrContains, "swap in rejection should explain the deposit is too small for vault %s", vault.GetAddress())
				s.assertBalance(depositorAddr, underlyingDenom, depositorFunding)
				s.assertBalance(depositorAddr, shareDenom, math.ZeroInt())
				s.assertBalance(vault.PrincipalMarkerAddress(), underlyingDenom, math.NewInt(tc.principalBacking))

				updatedVault, getErr := s.k.GetVault(s.ctx, vault.GetAddress())
				s.Require().NoError(getErr, "should get vault %s after rejected swap in", vault.GetAddress())
				s.Assert().Equal(tc.totalShares.String(), updatedVault.TotalShares.Amount.String(), "vault total shares must be unchanged after rejected swap in for vault %s", vault.GetAddress())
				return
			}

			s.Require().NoError(err, "swap in of %s should succeed when it converts to at least one share", depositCoin)
			s.Require().Equal(tc.expectedShares.String(), mintedShares.Amount.String(), "minted share amount mismatch for deposit %s", depositCoin)
			s.assertBalance(depositorAddr, underlyingDenom, depositorFunding.SubRaw(tc.deposit))
			s.assertBalance(depositorAddr, shareDenom, tc.expectedShares)
			s.assertBalance(vault.PrincipalMarkerAddress(), underlyingDenom, math.NewInt(tc.principalBacking+tc.deposit))

			updatedVault, getErr := s.k.GetVault(s.ctx, vault.GetAddress())
			s.Require().NoError(getErr, "should get vault %s after successful swap in", vault.GetAddress())
			s.Assert().Equal(tc.totalShares.Add(tc.expectedShares).String(), updatedVault.TotalShares.Amount.String(), "vault total shares should grow by the minted amount for vault %s", vault.GetAddress())
		})
	}
}

func (s *TestSuite) TestSwapOut_MultiAsset() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	unacceptedDenom := "junk"
	shareDenom := "vshare"
	blockTime := time.Now().UTC()
	s.ctx = s.ctx.WithBlockTime(blockTime)

	initialShares := utils.ShareScalar.MulRaw(100)
	vault := s.setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom, 1, 2)
	vault.SwapOutEnabled = true
	vault.WithdrawalDelaySeconds = 0 // Set to zero for instant processing in the same block's endblocker
	vault.TotalShares = sdk.NewCoin(shareDenom, initialShares)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	redeemerAddr := s.CreateAndFundAccount(sdk.NewCoin(shareDenom, initialShares))

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 500),
		sdk.NewInt64Coin(paymentDenom, 500),
	)), "should fund vault principal with liquidity")

	vault, err := s.k.GetVault(s.ctx, vault.GetAddress())
	s.Require().NoError(err, "should get vault")
	s.Require().NotNil(vault, "vault should not be nil")
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewCoin(shareDenom, utils.ShareScalar.MulRaw(500))), "should mint initial share supply")
	vault.TotalShares = vault.TotalShares.Add(sdk.NewCoin(shareDenom, utils.ShareScalar.MulRaw(500)))
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	sharesToRedeemForPayment := sdk.NewCoin(shareDenom, utils.ShareScalar.MulRaw(10))
	reqID1, err := s.k.SwapOut(s.ctx, vault.GetAddress(), redeemerAddr, sharesToRedeemForPayment, paymentDenom)
	s.Require().NoError(err, "should successfully queue swap out for an accepted payment denom")
	s.Require().Equal(uint64(0), reqID1, "first request id should be 0")

	s.assertBalance(redeemerAddr, shareDenom, initialShares.Sub(sharesToRedeemForPayment.Amount))
	s.assertBalance(vault.GetAddress(), shareDenom, sharesToRedeemForPayment.Amount)

	sharesToRedeemForDefaultPayment := sdk.NewCoin(shareDenom, utils.ShareScalar.MulRaw(5))
	reqID2, err := s.k.SwapOut(s.ctx, vault.GetAddress(), redeemerAddr, sharesToRedeemForDefaultPayment, "")
	s.Require().NoError(err, "should successfully queue swap out for the default underlying asset")
	s.Require().Equal(uint64(1), reqID2, "second request id should be 1")

	sharesToRedeemForUnderlying := sdk.NewCoin(shareDenom, utils.ShareScalar.MulRaw(8))
	reqID3, err := s.k.SwapOut(s.ctx, vault.GetAddress(), redeemerAddr, sharesToRedeemForUnderlying, underlyingDenom)
	s.Require().NoError(err, "should successfully queue swap out for the default underlying asset")
	s.Require().Equal(uint64(2), reqID3, "third request id should be 2")

	err = s.k.TestAccessor_processPendingSwapOuts(s.T(), s.ctx, keeper.MaxSwapOutBatchSize)
	s.Require().NoError(err, "processing pending withdrawals should not fail")

	// --- Assert Final Balances ---
	s.assertBalance(redeemerAddr, paymentDenom, math.NewInt(36))
	s.assertBalance(redeemerAddr, underlyingDenom, math.NewInt(10))

	// --- Test 3: Unaccepted Denom ---
	_, err = s.k.SwapOut(s.ctx, vault.GetAddress(), redeemerAddr, sharesToRedeemForDefaultPayment, unacceptedDenom)
	s.Require().Error(err, "should fail to swap out for an unaccepted asset")
	s.Require().ErrorContains(err, "denom not supported for vault", "error should indicate the denom is not accepted")
}

func (s *TestSuite) TestSwapOut_FailsWhenDisabled() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)
	redeemerAddr := s.CreateAndFundAccount(sdk.NewCoin(shareDenom, math.NewInt(100)))

	vault.SwapOutEnabled = false
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	_, err := s.k.SwapOut(s.ctx, vault.GetAddress(), redeemerAddr, sdk.NewInt64Coin(shareDenom, 10), "")
	s.Require().Error(err, "SwapOut should fail when swaps are disabled")
	s.Require().ErrorContains(err, "swaps are not enabled", "error message should mention swaps are disabled")
}

func (s *TestSuite) TestSwapOut_FailsWithInsufficientShares() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)

	initialTVV := int64(1000)
	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, initialTVV))), "should fund vault principal to give shares value")
	initialShares := utils.ShareScalar.MulRaw(initialTVV)
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewCoin(shareDenom, initialShares)), "should mint initial share supply")

	sharesForRedeemer := utils.ShareScalar.MulRaw(100)
	redeemerAddr := s.CreateAndFundAccount(sdk.Coin{})
	s.Require().NoError(s.k.MarkerKeeper.WithdrawCoins(s.ctx, vault.GetAddress(), redeemerAddr, shareDenom, sdk.NewCoins(sdk.NewCoin(shareDenom, sharesForRedeemer))), "should fund redeemer with shares")

	vault.SwapOutEnabled = true
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	sharesToRedeem := utils.ShareScalar.MulRaw(101)
	_, err := s.k.SwapOut(s.ctx, vault.GetAddress(), redeemerAddr, sdk.NewCoin(shareDenom, sharesToRedeem), "")
	s.Require().Error(err, "swap out should fail with insufficient shares")
	s.Require().ErrorContains(err, "insufficient funds", "error should mention insufficient funds for shares")
}

// TODO: https://github.com/ProvLabs/vault/issues/49
// We will probably need to review the implementation of this if it isn't right.
// This test confirms that a SwapOut from a marker account fails if the underlying restricted asset has no required attributes
// and the vault lacks TRANSFER permission on it. This behavior is distinct from the case where the asset has required
// attributes, in which the attribute check is correctly enforced. We need to confirm if this "fail-on-no-attributes" path is
// the intended design. A proposed solution would be to add bypass but send it to the vault account, then from the vault
// account, send it to the claimer without bypass in order to apply all the send restrictions correctly.
func (s *TestSuite) TestSwapOut_FailsWithRestrictedUnderlyingAssetNoAttributes() {
	shareDenom := "vshare"
	restrictedUnderlyingDenom := "restrictedasset"

	s.SetupTechFeeAccount(restrictedUnderlyingDenom)

	restrictedMarkerAddr := markertypes.MustGetMarkerAddress(restrictedUnderlyingDenom)
	restrictedMarker := markertypes.NewMarkerAccount(
		authtypes.NewBaseAccountWithAddress(restrictedMarkerAddr),
		sdk.NewInt64Coin(restrictedUnderlyingDenom, 1_000_000),
		s.adminAddr,
		[]markertypes.AccessGrant{
			{Address: s.adminAddr.String(), Permissions: markertypes.AccessList{markertypes.Access_Mint, markertypes.Access_Admin, markertypes.Access_Withdraw, markertypes.Access_Burn, markertypes.Access_Transfer}},
			{Address: s.EnsureTechFeeAccount().String(), Permissions: markertypes.AccessList{markertypes.Access_Transfer, markertypes.Access_Deposit}},
			{Address: types.GetVaultAddress(shareDenom).String(), Permissions: markertypes.AccessList{markertypes.Access_Withdraw, markertypes.Access_Transfer}},
		},
		markertypes.StatusProposed,
		markertypes.MarkerType_RestrictedCoin,
		false, true, false, []string{},
	)
	s.Require().NoError(s.simApp.MarkerKeeper.AddFinalizeAndActivateMarker(s.ctx, restrictedMarker), "should successfully create the restricted underlying asset marker")

	vaultCfg := vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      shareDenom,
		underlying: restrictedUnderlyingDenom,
	}
	vault, err := s.k.CreateVault(s.ctx, vaultCfg)
	s.Require().NoError(err, "vault creation with restricted underlying should succeed when vault has transfer permission")
	vault.SwapOutEnabled = true
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	marker, err := s.simApp.MarkerKeeper.GetMarker(s.ctx, restrictedMarkerAddr)
	s.Require().NoError(err, "should fetch active marker")
	activeMarker := marker.(*markertypes.MarkerAccount)

	// Now REMOVE the transfer permission from the vault to test the "fail-on-no-attributes" path
	activeMarker.AccessControl = []markertypes.AccessGrant{
		{Address: s.adminAddr.String(), Permissions: markertypes.AccessList{markertypes.Access_Mint, markertypes.Access_Admin, markertypes.Access_Withdraw, markertypes.Access_Burn, markertypes.Access_Transfer}},
		{Address: s.EnsureTechFeeAccount().String(), Permissions: markertypes.AccessList{markertypes.Access_Transfer, markertypes.Access_Deposit}},
		{Address: types.GetVaultAddress(shareDenom).String(), Permissions: markertypes.AccessList{markertypes.Access_Withdraw}},
	}
	s.simApp.MarkerKeeper.SetMarker(s.ctx, activeMarker)

	initialTVV := int64(500)
	s.Require().NoError(s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), restrictedUnderlyingDenom, sdk.NewCoins(sdk.NewInt64Coin(restrictedUnderlyingDenom, initialTVV))))
	initialShares := utils.ShareScalar.MulRaw(initialTVV)
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewCoin(shareDenom, initialShares)), "should mint initial share supply")

	redeemerAddr := s.CreateAndFundAccount(sdk.Coin{})
	sharesForRedeemer := utils.ShareScalar.MulRaw(100)
	s.Require().NoError(s.k.MarkerKeeper.WithdrawCoins(s.ctx, vault.GetAddress(), redeemerAddr, shareDenom, sdk.NewCoins(sdk.NewCoin(shareDenom, sharesForRedeemer))), "should fund redeemer from the vault's existing shares")

	sharesToRedeem := sdk.NewCoin(shareDenom, utils.ShareScalar.MulRaw(50))
	_, err = s.k.SwapOut(s.ctx, vault.GetAddress(), redeemerAddr, sharesToRedeem, "")

	s.Require().Error(err, "swap-out should fail because the vault lacks transfer permission and the asset is restricted (even with no attributes)")
	s.Require().ErrorContains(err, "does not have transfer permissions", "error should indicate missing transfer permissions")
}

func (s *TestSuite) TestSwapOut_FailsWithRestrictedUnderlyingAssetRequiredAttributes() {
	shareDenom := "vshare"
	restrictedUnderlyingDenom := "restrictedasset"

	requiredAttr := "you.dont.have.me"
	s.SetupTechFeeAccount(requiredAttr)

	restrictedMarkerAddr := markertypes.MustGetMarkerAddress(restrictedUnderlyingDenom)
	restrictedMarker := markertypes.NewMarkerAccount(
		authtypes.NewBaseAccountWithAddress(restrictedMarkerAddr),
		sdk.NewInt64Coin(restrictedUnderlyingDenom, 1_000_000),
		s.adminAddr,
		[]markertypes.AccessGrant{
			{Address: s.adminAddr.String(), Permissions: markertypes.AccessList{markertypes.Access_Mint, markertypes.Access_Admin, markertypes.Access_Withdraw, markertypes.Access_Burn, markertypes.Access_Transfer}},
			{Address: s.EnsureTechFeeAccount().String(), Permissions: markertypes.AccessList{markertypes.Access_Transfer, markertypes.Access_Deposit}},
			{Address: types.GetVaultAddress(shareDenom).String(), Permissions: markertypes.AccessList{markertypes.Access_Withdraw}},
		},
		markertypes.StatusProposed,
		markertypes.MarkerType_RestrictedCoin,
		false, true, false, []string{"you.dont.have.me"},
	)
	s.Require().NoError(s.simApp.MarkerKeeper.AddFinalizeAndActivateMarker(s.ctx, restrictedMarker), "should successfully create the restricted underlying asset marker")

	vaultCfg := vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      shareDenom,
		underlying: restrictedUnderlyingDenom,
	}
	vault, err := s.k.CreateVault(s.ctx, vaultCfg)
	s.Require().NoError(err, "vault creation with restricted underlying should succeed")
	vault.SwapOutEnabled = true
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	initialTVV := int64(500)
	s.Require().NoError(s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), restrictedUnderlyingDenom, sdk.NewCoins(sdk.NewInt64Coin(restrictedUnderlyingDenom, initialTVV))))
	initialShares := utils.ShareScalar.MulRaw(initialTVV)
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewCoin(shareDenom, initialShares)), "should mint initial share supply")

	sharesForRedeemer := utils.ShareScalar.MulRaw(100)
	redeemerAddr := s.CreateAndFundAccount(sdk.NewCoin(shareDenom, sharesForRedeemer))

	sharesToRedeem := sdk.NewCoin(shareDenom, utils.ShareScalar.MulRaw(50))
	_, err = s.k.SwapOut(s.ctx, vault.GetAddress(), redeemerAddr, sharesToRedeem, "")

	s.Require().Error(err, "swap-out should fail because the redeemer is missing a required attribute")
	s.Require().ErrorContains(err, "required attribute: \"you.dont.have.me\"", "error should indicate a missing attribute failure")
}

func (s *TestSuite) TestSwapOut_SucceedsWithRestrictedUnderlyingAssetRequiredAttributes() {
	shareDenom := "vshare"
	restrictedUnderlyingDenom := "restrictedasset"
	requiredAttribute := "iamrequired"

	s.SetupTechFeeAccount(requiredAttribute)

	s.ctx = s.ctx.WithBlockTime(time.Now().UTC())

	restrictedMarkerAddr := markertypes.MustGetMarkerAddress(restrictedUnderlyingDenom)
	restrictedMarker := markertypes.NewMarkerAccount(
		authtypes.NewBaseAccountWithAddress(restrictedMarkerAddr),
		sdk.NewInt64Coin(restrictedUnderlyingDenom, 1_000_000),
		s.adminAddr,
		[]markertypes.AccessGrant{
			{Address: s.adminAddr.String(), Permissions: markertypes.AccessList{markertypes.Access_Mint, markertypes.Access_Admin, markertypes.Access_Withdraw, markertypes.Access_Burn, markertypes.Access_Transfer}},
			{Address: s.EnsureTechFeeAccount().String(), Permissions: markertypes.AccessList{markertypes.Access_Transfer, markertypes.Access_Deposit}},
			{Address: types.GetVaultAddress(shareDenom).String(), Permissions: markertypes.AccessList{markertypes.Access_Withdraw}},
		},
		markertypes.StatusProposed,
		markertypes.MarkerType_RestrictedCoin,
		false, true, false, []string{requiredAttribute},
	)
	s.Require().NoError(s.simApp.MarkerKeeper.AddFinalizeAndActivateMarker(s.ctx, restrictedMarker), "should successfully create the restricted underlying asset marker")

	vaultCfg := vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      shareDenom,
		underlying: restrictedUnderlyingDenom,
	}
	vault, err := s.k.CreateVault(s.ctx, vaultCfg)
	s.Require().NoError(err, "vault creation with restricted underlying should succeed")
	vault.SwapOutEnabled = true
	vault.WithdrawalDelaySeconds = 0 // Set to zero for same-block processing
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	initialTVV := int64(500)
	s.Require().NoError(s.k.MarkerKeeper.WithdrawCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), restrictedUnderlyingDenom, sdk.NewCoins(sdk.NewInt64Coin(restrictedUnderlyingDenom, initialTVV))))
	initialShares := utils.ShareScalar.MulRaw(initialTVV)
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewCoin(shareDenom, initialShares)), "should mint initial share supply")
	vault, err = s.k.GetVault(s.ctx, vault.GetAddress())
	s.Require().NoError(err, "should get vault")
	s.Require().NotNil(vault, "vault should not be nil")
	vault.TotalShares = sdk.NewCoin(shareDenom, initialShares)
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	redeemerAddr := s.CreateAndFundAccount(sdk.Coin{})
	sharesForRedeemer := utils.ShareScalar.MulRaw(100)
	s.Require().NoError(s.k.MarkerKeeper.WithdrawCoins(s.ctx, vault.GetAddress(), redeemerAddr, shareDenom, sdk.NewCoins(sdk.NewCoin(shareDenom, sharesForRedeemer))), "should fund redeemer from the vault's existing shares")

	s.simApp.AccountKeeper.SetAccount(s.ctx, s.simApp.AccountKeeper.NewAccountWithAddress(s.ctx, s.adminAddr))

	expireTime := time.Now().Add(24 * time.Hour)
	attribute := attrtypes.NewAttribute(requiredAttribute, redeemerAddr.String(), attrtypes.AttributeType_String, []byte("true"), &expireTime, "")
	s.Require().NoError(s.simApp.AttributeKeeper.SetAttribute(s.ctx, attribute, s.adminAddr), "should successfully set the required attribute on the redeemer")

	sharesToRedeem := sdk.NewCoin(shareDenom, utils.ShareScalar.MulRaw(50))
	_, err = s.k.SwapOut(s.ctx, vault.GetAddress(), redeemerAddr, sharesToRedeem, "")
	s.Require().NoError(err, "swap-out request should succeed because the redeemer has the required attribute")

	s.assertBalance(redeemerAddr, shareDenom, sharesForRedeemer.Sub(sharesToRedeem.Amount))
	s.assertBalance(vault.GetAddress(), shareDenom, sharesToRedeem.Amount)
	err = s.k.TestAccessor_processPendingSwapOuts(s.T(), s.ctx, keeper.MaxSwapOutBatchSize)
	s.Require().NoError(err, "processing pending withdrawals should not fail")

	s.assertBalance(redeemerAddr, restrictedUnderlyingDenom, math.NewInt(50))
}

func (s *TestSuite) TestSetMinMaxInterestRate_NoOp_NoEvent() {
	share := "vaultshare-np"
	base := "undercoin-np"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(base, 1_000), s.adminAddr)

	attrs := vaultAttrs{admin: s.adminAddr.String(), share: share, underlying: base}
	v, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().NoError(err, "vault creation should succeed")

	s.Require().NoError(s.k.UpdateInterestRates(s.ctx, v, "0.10", "0.10"), "should set initial min/max rates without error")
	s.k.AuthKeeper.SetAccount(s.ctx, v)

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	err = s.k.SetMinInterestRate(s.ctx, v, "0.10")
	s.Require().NoError(err, "setting min interest rate should not error")

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	err = s.k.SetMinInterestRate(s.ctx, v, "0.10")
	s.Require().NoError(err, "setting min interest rate should not error")
	s.Require().Len(s.ctx.EventManager().Events(), 0, "no events should be emitted when setting the same min interest rate")

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	err = s.k.SetMaxInterestRate(s.ctx, v, "0.25")
	s.Require().NoError(err, "setting max interest rate should not error")

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	err = s.k.SetMaxInterestRate(s.ctx, v, "0.25")
	s.Require().NoError(err, "setting max interest rate should not error")
	s.Require().Len(s.ctx.EventManager().Events(), 0, "no events should be emitted when setting the same max interest rate")
}

func (s *TestSuite) TestSetMinInterestRate_ValidationBlocksWhenAboveExistingMax() {
	share := "vaultshare-min-gt-max"
	base := "undercoin-min-gt-max"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(base, 1_000), s.adminAddr)

	attrs := vaultAttrs{admin: s.adminAddr.String(), share: share, underlying: base}
	v, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().NoError(err)

	s.Require().NoError(s.k.SetMaxInterestRate(s.ctx, v, "0.40"))

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	err = s.k.SetMinInterestRate(s.ctx, v, "0.50")
	s.Require().Error(err, "setting min interest rate above max should fail")
	s.Require().Contains(err.Error(), "minimum interest rate", "error message mismatch when setting min > max")
	s.Require().Equal("0.40", v.MaxInterestRate, "max interest rate should remain unchanged")
	s.Require().Equal("", v.MinInterestRate, "min interest rate should remain unchanged")
	s.Require().Empty(s.ctx.EventManager().Events(), "no events should be emitted on validation failure")
}

func (s *TestSuite) TestSetMaxInterestRate_ValidationBlocksWhenBelowExistingMin() {
	share := "vaultshare-max-lt-min"
	base := "undercoin-max-lt-min"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(base, 1_000), s.adminAddr)

	attrs := vaultAttrs{admin: s.adminAddr.String(), share: share, underlying: base}
	v, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().NoError(err, "CreateVault should succeed")

	s.Require().NoError(s.k.SetMinInterestRate(s.ctx, v, "-0.50"), "setting min interest rate should succeed")

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	err = s.k.SetMaxInterestRate(s.ctx, v, "-0.60")
	s.Require().Error(err, "setting max interest rate below min should fail")
	s.Require().Contains(err.Error(), "minimum interest rate", "error message mismatch when setting max < min")
	s.Require().Equal("-0.50", v.MinInterestRate, "min interest rate should remain unchanged")
	s.Require().Equal("", v.MaxInterestRate, "max interest rate should remain unchanged")
	s.Require().Empty(s.ctx.EventManager().Events(), "no events should be emitted on validation failure")
}

func (s *TestSuite) TestValidateInterestRateLimits() {
	tests := []struct {
		name      string
		min       string
		max       string
		expectErr string
	}{
		{
			name: "both empty => ok",
			min:  "",
			max:  "",
		},
		{
			name: "min empty => ok",
			min:  "",
			max:  "0.25",
		},
		{
			name: "max empty => ok",
			min:  "0.10",
			max:  "",
		},
		{
			name: "equal => ok",
			min:  "0.15",
			max:  "0.15",
		},
		{
			name: "min < max => ok",
			min:  "0.049",
			max:  "0.051",
		},
		{
			name:      "min > max => error",
			min:       "0.60",
			max:       "0.50",
			expectErr: "minimum interest rate",
		},
		{
			name:      "bad min => error",
			min:       "nope",
			max:       "0.10",
			expectErr: "invalid min interest rate",
		},
		{
			name:      "bad max => error",
			min:       "0.10",
			max:       "wat",
			expectErr: "invalid max interest rate",
		},
		{
			name:      "bad min, empty max => error",
			min:       "not-a-number",
			max:       "",
			expectErr: "invalid min interest rate",
		},
		{
			name:      "empty min, bad max => error",
			min:       "",
			max:       "invalid-decimal",
			expectErr: "invalid max interest rate",
		},
		{
			name:      "both bad => error",
			min:       "bad",
			max:       "worse",
			expectErr: "invalid min interest rate",
		},
		{
			name: "zero min, zero max => ok",
			min:  "0",
			max:  "0",
		},
		{
			name: "zero min, positive max => ok",
			min:  "0",
			max:  "0.25",
		},
		{
			name: "high precision => ok",
			min:  "0.123456789012345678",
			max:  "0.223456789012345678",
		},
		{
			name:      "negative min > negative max => error",
			min:       "-0.05",
			max:       "-0.10",
			expectErr: "cannot be greater than",
		},
		{
			name: "negative min < negative max => ok",
			min:  "-0.10",
			max:  "-0.05",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			err := s.k.ValidateInterestRateLimits(tc.min, tc.max)
			if tc.expectErr == "" {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.expectErr)
			}
		})
	}
}

func (s *TestSuite) TestUpdateInterestRate_BoundsEnforced() {
	share := "vaultshare-rate"
	base := "undercoin-rate"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(base, 1_000_000), s.adminAddr)

	attrs := vaultAttrs{admin: s.adminAddr.String(), share: share, underlying: base}
	_, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().NoError(err)
	addr := types.GetVaultAddress(share)

	v, err := s.k.GetVault(s.ctx, addr)
	s.Require().NoError(err, "GetVault should succeed")

	s.Require().NoError(s.k.UpdateInterestRates(s.ctx, v, "0.10", "0.10"), "should set initial min/max rates without error")
	s.k.AuthKeeper.SetAccount(s.ctx, v)

	s.Require().NoError(s.k.SetMinInterestRate(s.ctx, v, "0.10"), "setting min interest rate should not error")
	s.Require().NoError(s.k.SetMaxInterestRate(s.ctx, v, "0.50"), "setting max interest rate should not error")

	srv := keeper.NewMsgServer(s.simApp.VaultKeeper)

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	_, err = srv.UpdateInterestRate(s.ctx, &types.MsgUpdateInterestRateRequest{
		Authority:    s.adminAddr.String(),
		VaultAddress: addr.String(),
		NewRate:      "0.25",
	})
	s.Require().NoError(err, "updating interest rate to 0.25 should not error")
	v2, err := s.k.GetVault(s.ctx, addr)
	s.Require().NoError(err, "getting vault after interest rate update should not error")
	s.Require().Equal("0.25", v2.CurrentInterestRate, "current interest rate mismatch after update")
	s.Require().Equal("0.25", v2.DesiredInterestRate, "desired interest rate mismatch after update")

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	_, err = srv.UpdateInterestRate(s.ctx, &types.MsgUpdateInterestRateRequest{
		Authority:    s.adminAddr.String(),
		VaultAddress: addr.String(),
		NewRate:      "0.05",
	})
	s.Require().Error(err, "updating interest rate to 0.05 (below min 0.10) should error")

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	_, err = srv.UpdateInterestRate(s.ctx, &types.MsgUpdateInterestRateRequest{
		Authority:    s.adminAddr.String(),
		VaultAddress: addr.String(),
		NewRate:      "0.60",
	})
	s.Require().Error(err, "updating interest rate to 0.60 (above max 0.50) should error")
}

func (s *TestSuite) TestAutoPauseVault_SetsPausedAndEmitsEvent() {
	under := "under-ap"
	share := "share-ap"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(under, 1_000), s.adminAddr)

	v, err := s.k.CreateVault(s.ctx, vaultAttrs{admin: s.adminAddr.String(), share: share, underlying: under})
	s.Require().NoError(err, "CreateVault should succeed")

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())

	reason := "critical failure"
	s.k.TestAccessor_autoPauseVault(s.T(), s.ctx, v, reason)

	got, err := s.k.GetVault(s.ctx, types.GetVaultAddress(share))
	s.Require().NoError(err, "GetVault should succeed after autoPause")
	s.Require().True(got.Paused, "vault should be marked paused")
	s.Require().Equal(reason, got.PausedReason, "paused reason should match provided error")
	s.Require().Equal(types.ZeroInterestRate, got.CurrentInterestRate, "auto pause should zero the current interest rate to stop accrual, mirroring the operator pause")

	evs := s.ctx.EventManager().Events()
	s.Require().NotEmpty(evs, "an event should be emitted")

	hasInterestChange := false
	for _, e := range evs {
		if e.Type == "provlabs.vault.v1.EventVaultInterestChange" {
			hasInterestChange = true
		}
	}
	s.Require().True(hasInterestChange, "auto pause should emit an EventVaultInterestChange when zeroing the rate, mirroring the operator pause")

	last := evs[len(evs)-1]
	s.Require().Equal("provlabs.vault.v1.EventVaultPaused", last.Type, "event type should be EventVaultAutoPaused")

	hasAddr := false
	hasReason := false
	for _, a := range last.Attributes {
		if a.Key == "vault_address" && a.Value == fmt.Sprintf("\"%s\"", v.GetAddress().String()) {
			hasAddr = true
		}
		if a.Key == "reason" && a.Value == fmt.Sprintf("\"%s\"", reason) {
			hasReason = true
		}
	}
	s.Require().True(hasAddr, "event should include vault_address attribute")
	s.Require().True(hasReason, "event should include reason attribute")
}

func (s *TestSuite) TestSetWithdrawalDelay() {
	share := "jackthecatshare"
	under := "georgethedogunder"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(under, 1_000_000), s.adminAddr)

	attrs := vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      share,
		underlying: under,
	}
	vault, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().NoError(err, "CreateVault should succeed")

	vaultAddr := types.GetVaultAddress(share)
	authorityAddr := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
	authority := authorityAddr.String()
	delay := uint64(3600)

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	err = s.k.SetWithdrawalDelay(s.ctx, vault, delay, authority)
	s.Require().NoError(err, "SetWithdrawalDelay should succeed")

	updated, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "GetVault should succeed after SetWithdrawalDelay")
	s.Require().NotNil(updated, "vault should exist after SetWithdrawalDelay")
	s.Require().Equal(delay, updated.WithdrawalDelaySeconds, "WithdrawalDelaySeconds should be updated")

	expectedEvents := sdk.Events{
		sdk.NewEvent("provlabs.vault.v1.EventWithdrawalDelayUpdated",
			sdk.NewAttribute("authority", authority),
			sdk.NewAttribute("vault_address", vaultAddr.String()),
			sdk.NewAttribute("withdrawal_delay_seconds", fmt.Sprintf("%d", delay)),
		),
	}

	evs := s.ctx.EventManager().Events()
	s.Require().Equal(normalizeEvents(expectedEvents), normalizeEvents(evs), "events should match expected EventWithdrawalDelayUpdated")
}
