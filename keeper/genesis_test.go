package keeper_test

import (
	"fmt"
	"math"
	"time"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/provlabs/vault/types"
)

func (s *TestSuite) TestVaultGenesis_InitAndExport() {
	shareDenom := "vaultshare"
	underlying := "undercoin"
	admin := s.adminAddr.String()
	vaultAddr := types.GetVaultAddress(shareDenom)

	vault := makeGenesisVaultAccount(shareDenom, underlying, admin)

	past := time.Now().Add(-5 * time.Minute).Unix()
	vault.PeriodTimeout = past

	genesis := &types.GenesisState{
		Params: types.DefaultParams(),
		Vaults: []types.VaultAccount{vault},
		PayoutTimeoutQueue: []types.QueueEntry{
			{Time: uint64(past), Addr: vaultAddr.String()},
		},
		PendingSwapOutQueue: types.PendingSwapOutQueue{
			LatestSequenceNumber: 55,
			Entries: []types.PendingSwapOutQueueEntry{
				{
					Time: 10000,
					Id:   1,
					SwapOut: types.PendingSwapOut{
						Owner:        admin,
						VaultAddress: vault.Address,
						RedeemDenom:  "ylds",
						Shares:       sdk.NewInt64Coin("vshares", 100),
					},
				},
			},
		},
	}

	s.k.InitGenesis(s.ctx, genesis)
	s.ctx = s.ctx.WithBlockTime(time.Unix(past+60, 0))

	exported := s.k.ExportGenesis(s.ctx)
	s.Require().Len(exported.Vaults, 1, "exported genesis should contain exactly one vault")
	exp := exported.Vaults[0]
	s.Require().Equal(vault.GetAddress().String(), exp.GetAddress().String(), "exported vault address mismatch")
	s.Require().Equal(vault.Admin, exp.Admin, "exported vault admin mismatch")
	s.Require().Equal(vault.TotalShares, exp.TotalShares, "exported vault total shares mismatch")
	s.Require().Equal(vault.UnderlyingAsset, exp.UnderlyingAsset, "exported vault underlying asset mismatch")

	s.Require().Len(exported.PayoutTimeoutQueue, 1, "exported genesis should contain exactly one payout timeout entry")
	s.Require().Equal(vaultAddr.String(), exported.PayoutTimeoutQueue[0].Addr, "payout timeout entry address mismatch")
	s.Require().Equal(uint64(past), exported.PayoutTimeoutQueue[0].Time, "payout timeout entry time mismatch")

	got, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err, "failed to get vault after InitGenesis")
	s.Require().NotNil(got, "vault should not be nil after InitGenesis")
	s.Require().Equal(vaultAddr.String(), got.GetAddress().String(), "retrieved vault address mismatch")

	s.Require().Len(exported.PendingSwapOutQueue.Entries, 1, "exported genesis should contain exactly one pending swap out entry")
	s.Require().Equal(uint64(55), exported.PendingSwapOutQueue.LatestSequenceNumber, "pending swap out queue latest sequence number mismatch")
	s.Require().Equal(uint64(1), exported.PendingSwapOutQueue.Entries[0].Id, "pending swap out entry ID mismatch")
	s.Require().Equal(int64(10000), exported.PendingSwapOutQueue.Entries[0].Time, "pending swap out entry time mismatch")
	s.Require().Equal(admin, exported.PendingSwapOutQueue.Entries[0].SwapOut.Owner, "pending swap out entry owner mismatch")
	s.Require().Equal(vault.Address, exported.PendingSwapOutQueue.Entries[0].SwapOut.VaultAddress, "pending swap out entry vault address mismatch")
	s.Require().Equal("ylds", exported.PendingSwapOutQueue.Entries[0].SwapOut.RedeemDenom, "pending swap out entry redeem denom mismatch")
}

func (s *TestSuite) TestInitGenesis_PanicOnInvalidTimeout() {
	vaultAddr := types.GetVaultAddress("panic-vault").String()
	vault := makeGenesisVaultAccount("panic-vault", "undercoin", s.adminAddr.String())

	tests := []struct {
		name     string
		genState *types.GenesisState
		panicMsg string
	}{
		{
			name: "payout timeout exceeds max int64",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{
					func() types.VaultAccount {
						v := vault
						v.PeriodTimeout = math.MinInt64
						return v
					}(),
				},
				PayoutTimeoutQueue: []types.QueueEntry{
					{Time: uint64(math.MaxInt64) + 1, Addr: vaultAddr},
				},
			},
			panicMsg: fmt.Sprintf("invalid vault genesis state: payout timeout queue entry at index 0 has time %d which exceeds max int64", uint64(math.MaxInt64)+1),
		},
		{
			name: "fee timeout exceeds max int64",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				Vaults: []types.VaultAccount{
					func() types.VaultAccount {
						v := vault
						v.FeePeriodTimeout = math.MinInt64
						return v
					}(),
				},
				FeeTimeoutQueue: []types.QueueEntry{
					{Time: uint64(math.MaxInt64) + 1, Addr: vaultAddr},
				},
			},
			panicMsg: fmt.Sprintf("invalid vault genesis state: fee timeout queue entry at index 0 has time %d which exceeds max int64", uint64(math.MaxInt64)+1),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()
			s.Require().PanicsWithError(tt.panicMsg, func() {
				s.k.InitGenesis(s.ctx, tt.genState)
			})
		})
	}
}

func (s *TestSuite) TestVaultGenesis_RoundTrip_FeeTimeoutAndParams() {
	shareDenom := "vaultshare_fee"
	underlying := "under_fee"
	vaultAddr := types.GetVaultAddress(shareDenom)
	admin := s.adminAddr.String()
	aumFeeAddress := sdk.AccAddress{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}

	now := time.Now().Unix()
	future := now + 1000

	vault := makeGenesisVaultAccount(shareDenom, underlying, admin)
	vault.FeePeriodTimeout = future

	params := types.Params{
		TechFeeAddress:    aumFeeAddress.String(),
		DefaultAumFeeBips: 100,
	}

	genesis := &types.GenesisState{
		Vaults: []types.VaultAccount{vault},
		FeeTimeoutQueue: []types.QueueEntry{
			{Time: uint64(future), Addr: vaultAddr.String()},
		},
		Params: params,
	}

	s.k.InitGenesis(s.ctx, genesis)

	exported := s.k.ExportGenesis(s.ctx)
	s.Require().Len(exported.FeeTimeoutQueue, 1, "exported genesis should contain exactly one fee timeout entry")
	s.Require().Equal(vaultAddr.String(), exported.FeeTimeoutQueue[0].Addr, "fee timeout entry address mismatch")
	s.Require().Equal(uint64(future), exported.FeeTimeoutQueue[0].Time, "fee timeout entry time mismatch")
	s.Require().Equal(params, exported.Params, "exported Params mismatch")

	// Verify the AUM fee address was actually set in the keeper
	storedAddr, err := s.k.GetAUMFeeAddress(s.ctx)
	s.Require().NoError(err, "failed to get AUM fee address")
	s.Require().Equal(aumFeeAddress, storedAddr, "stored AUM fee address mismatch in keeper")
}

func (s *TestSuite) TestVaultGenesis_RoundTrip_PastAndFutureTimeouts() {
	shareDenom1 := "vaultshare2"
	underlying1 := "under2"
	vaultAddr1 := types.GetVaultAddress(shareDenom1)

	shareDenom2 := "vaultshare3"
	underlying2 := "under3"
	vaultAddr2 := types.GetVaultAddress(shareDenom2)

	admin := s.adminAddr.String()

	now := time.Now()
	past := now.Add(-10 * time.Minute).Unix()
	future := now.Add(10 * time.Minute).Unix()

	vault1 := makeGenesisVaultAccount(shareDenom1, underlying1, admin)
	vault1.PeriodTimeout = past

	vault2 := makeGenesisVaultAccount(shareDenom2, underlying2, admin)
	vault2.PeriodTimeout = future

	genesis := &types.GenesisState{
		Params: types.DefaultParams(),
		Vaults: []types.VaultAccount{vault1, vault2},
		PayoutTimeoutQueue: []types.QueueEntry{
			{Time: uint64(past), Addr: vaultAddr1.String()},
			{Time: uint64(future), Addr: vaultAddr2.String()},
		},
		PendingSwapOutQueue: types.PendingSwapOutQueue{},
	}

	s.k.InitGenesis(s.ctx, genesis)
	s.ctx = s.ctx.WithBlockTime(now)

	exported := s.k.ExportGenesis(s.ctx)
	s.Require().Len(exported.PayoutTimeoutQueue, 2, "exported genesis should contain exactly two payout timeout entries")
}

func (s *TestSuite) TestVaultGenesis_InvalidTimeoutAddressPanics() {
	shareDenom := "vaultshare3"
	underlying := "under3"
	admin := s.adminAddr.String()

	now := uint64(time.Now().Unix())

	vault := makeGenesisVaultAccount(shareDenom, underlying, admin)
	vault.PeriodTimeout = int64(now)

	genesis := &types.GenesisState{
		Params: types.DefaultParams(),
		Vaults: []types.VaultAccount{vault},
		PayoutTimeoutQueue: []types.QueueEntry{
			{Time: now, Addr: "invalid-bech32"},
		},
		PendingSwapOutQueue: types.PendingSwapOutQueue{},
	}

	expectedPanic := "invalid vault genesis state: invalid payout timeout queue address at index 0: decoding bech32 failed: invalid separator index -1"
	s.Require().PanicsWithError(expectedPanic, func() { s.k.InitGenesis(s.ctx, genesis) }, "InitGenesis should panic on invalid timeout address")
}

func (s *TestSuite) TestVaultGenesis_ExistingAccountNumberCopied() {
	shareDenom := "vaultshare4"
	underlying := "under4"
	admin := s.adminAddr.String()
	vaultAddr := types.GetVaultAddress(shareDenom)

	existing := authtypes.NewBaseAccountWithAddress(vaultAddr)
	s.Require().NoError(existing.SetAccountNumber(999), "failed to set account number for existing account")
	s.k.AuthKeeper.SetAccount(s.ctx, existing)

	vault := makeGenesisVaultAccount(shareDenom, underlying, admin)

	genesis := &types.GenesisState{
		Params:              types.DefaultParams(),
		Vaults:              []types.VaultAccount{vault},
		PendingSwapOutQueue: types.PendingSwapOutQueue{},
	}

	s.k.InitGenesis(s.ctx, genesis)
	exported := s.k.ExportGenesis(s.ctx)
	s.Require().Len(exported.Vaults, 1, "exported genesis should contain exactly one vault")
	s.Require().Equal(uint64(999), exported.Vaults[0].GetAccountNumber(), "exported vault account number mismatch")
}

func (s *TestSuite) TestVaultGenesis_InitNilDoesNothing() {
	s.k.InitGenesis(s.ctx, nil)
	exported := s.k.ExportGenesis(s.ctx)
	s.Require().Len(exported.Vaults, 0, "exported genesis should contain zero vaults after nil InitGenesis")
	s.Require().Len(exported.PayoutTimeoutQueue, 0, "exported genesis should contain zero payout timeout entries after nil InitGenesis")
}

func (s *TestSuite) TestVaultGenesis_InitPanicsOnInvalidVault() {
	vaultAddr := sdk.AccAddress("badaddr")
	vault := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               "",
		TotalShares:         sdk.Coin{Denom: "invalid denom!", Amount: sdkmath.NewInt(0)},
		UnderlyingAsset:     "underX",
		PaymentDenom:        "underX",
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}
	genesis := &types.GenesisState{
		Params: types.DefaultParams(),
		Vaults: []types.VaultAccount{vault},
	}
	expectedPanic := "invalid vault genesis state: invalid vault at index 0: invalid admin address: empty address string is not allowed"
	s.Require().PanicsWithError(expectedPanic, func() { s.k.InitGenesis(s.ctx, genesis) }, "InitGenesis should panic on invalid vault")
}

func (s *TestSuite) TestVaultGenesis_InitPanicsOnInvalidPendingSwapOut() {
	shareDenom := "vaultshare"
	underlying := "undercoin"
	admin := s.adminAddr.String()

	vault := makeGenesisVaultAccount(shareDenom, underlying, admin)

	genesis := &types.GenesisState{
		Params: types.DefaultParams(),
		Vaults: []types.VaultAccount{vault},
		PendingSwapOutQueue: types.PendingSwapOutQueue{
			LatestSequenceNumber: 55,
			Entries: []types.PendingSwapOutQueueEntry{
				{
					Time: 10000,
					Id:   1,
					SwapOut: types.PendingSwapOut{
						Owner:        "badaddress",
						VaultAddress: vault.Address,
						RedeemDenom:  "ylds",
						Shares:       sdk.NewInt64Coin("vshares", 100),
					},
				},
			},
		},
	}
	expectedPanic := "failed to import pending swap out queue: invalid owner address in pending swap out queue: decoding bech32 failed: invalid separator index -1"
	s.Require().PanicsWithError(expectedPanic, func() { s.k.InitGenesis(s.ctx, genesis) }, "InitGenesis should panic on invalid pending swap out")
}

func (s *TestSuite) TestVaultGenesis_InitPanicsWhenPendingSwapOutHasUnknownVault() {
	shareDenom := "vaultshare"
	underlying := "undercoin"
	admin := s.adminAddr.String()
	badVaultAddr := types.GetVaultAddress("baddenom")

	vault := makeGenesisVaultAccount(shareDenom, underlying, admin)

	genesis := &types.GenesisState{
		Params: types.DefaultParams(),
		Vaults: []types.VaultAccount{vault},
		PendingSwapOutQueue: types.PendingSwapOutQueue{
			LatestSequenceNumber: 55,
			Entries: []types.PendingSwapOutQueueEntry{
				{
					Time: 10000,
					Id:   1,
					SwapOut: types.PendingSwapOut{
						Owner:        admin,
						VaultAddress: badVaultAddr.String(),
						RedeemDenom:  "ylds",
						Shares:       sdk.NewInt64Coin("vshares", 100),
					},
				},
			},
		},
	}
	expectedPanic := fmt.Sprintf("invalid vault genesis state: pending swap out queue vault address at index 0 is not an imported vault: %s", badVaultAddr.String())
	s.Require().PanicsWithError(expectedPanic, func() { s.k.InitGenesis(s.ctx, genesis) }, "InitGenesis should panic on unknown vault")
}

func (s *TestSuite) TestVaultGenesis_InitPanicsWhenPendingSwapOutHasBadVaultAddress() {
	shareDenom := "vaultshare"
	underlying := "undercoin"
	admin := s.adminAddr.String()

	vault := makeGenesisVaultAccount(shareDenom, underlying, admin)

	genesis := &types.GenesisState{
		Params: types.DefaultParams(),
		Vaults: []types.VaultAccount{vault},
		PendingSwapOutQueue: types.PendingSwapOutQueue{
			LatestSequenceNumber: 55,
			Entries: []types.PendingSwapOutQueueEntry{
				{
					Time: 10000,
					Id:   1,
					SwapOut: types.PendingSwapOut{
						Owner:        admin,
						VaultAddress: "badaddress",
						RedeemDenom:  "ylds",
						Shares:       sdk.NewInt64Coin("vshares", 100),
					},
				},
			},
		},
	}
	expectedPanic := "invalid vault genesis state: invalid vault address in pending swap out queue at index 0: decoding bech32 failed: invalid separator index -1"
	s.Require().PanicsWithError(expectedPanic, func() { s.k.InitGenesis(s.ctx, genesis) }, "InitGenesis should panic on bad vault address")
}

func (s *TestSuite) TestVaultGenesis_InitPanicsWhenPayoutTimeoutHasUnknownVault() {
	badVaultAddr := types.GetVaultAddress("baddenom")

	genesis := &types.GenesisState{
		Params: types.DefaultParams(),
		PayoutTimeoutQueue: []types.QueueEntry{
			{Time: 1000, Addr: badVaultAddr.String()},
		},
	}
	expectedPanic := fmt.Sprintf("invalid vault genesis state: payout timeout queue address at index 0 is not an imported vault: %s", badVaultAddr.String())
	s.Require().PanicsWithError(expectedPanic, func() { s.k.InitGenesis(s.ctx, genesis) }, "InitGenesis should panic on unknown vault in payout timeout queue")
}

func (s *TestSuite) TestVaultGenesis_InitPanicsWhenFeeTimeoutHasUnknownVault() {
	badVaultAddr := types.GetVaultAddress("baddenom")

	genesis := &types.GenesisState{
		Params: types.DefaultParams(),
		FeeTimeoutQueue: []types.QueueEntry{
			{Time: 1000, Addr: badVaultAddr.String()},
		},
	}
	expectedPanic := fmt.Sprintf("invalid vault genesis state: fee timeout queue address at index 0 is not an imported vault: %s", badVaultAddr.String())
	s.Require().PanicsWithError(expectedPanic, func() { s.k.InitGenesis(s.ctx, genesis) }, "InitGenesis should panic on unknown vault in fee timeout queue")
}

// TestVaultGenesis_RoundTrip_NAVs verifies the internal NAV table survives a
// genesis export/import round trip.
func (s *TestSuite) TestVaultGenesis_RoundTrip_NAVs() {
	shareDenom := "navshare"
	underlying := "navunder"
	vaultAddr := types.GetVaultAddress(shareDenom)

	updatedTime := time.Unix(1700000000, 0).UTC()
	navs := []types.VaultNAVEntry{
		{
			VaultAddress: vaultAddr.String(),
			Nav: types.VaultNAV{
				Denom:              "rwaone",
				Price:              sdk.NewInt64Coin(underlying, 150),
				Volume:             sdkmath.NewInt(3),
				Source:             "oracle-one",
				UpdatedBlockHeight: 7,
				UpdatedTime:        updatedTime,
			},
		},
		{
			VaultAddress: vaultAddr.String(),
			Nav: types.VaultNAV{
				Denom:              "rwatwo",
				Price:              sdk.Coin{Denom: underlying, Amount: sdkmath.NewInt(2)},
				Volume:             sdkmath.NewInt(1),
				UpdatedBlockHeight: 9,
				UpdatedTime:        updatedTime,
			},
		},
	}

	genesis := s.setupVaultWithNavs(shareDenom, underlying, s.adminAddr.String(), navs)

	storedNav, err := s.k.GetVaultNAV(s.ctx, vaultAddr, "rwaone")
	s.Require().NoError(err, "NAV should exist after InitGenesis")
	s.Assert().Equal(sdk.NewInt64Coin(underlying, 150), storedNav.Price, "imported NAV price mismatch")
	s.Assert().Equal(sdkmath.NewInt(3), storedNav.Volume, "imported NAV volume mismatch")
	s.Assert().Equal("oracle-one", storedNav.Source, "imported NAV source mismatch")

	exported := s.k.ExportGenesis(s.ctx)
	s.Require().Len(exported.Navs, len(genesis.Navs), "exported genesis should contain every NAV entry")
	s.Assert().ElementsMatch(genesis.Navs, exported.Navs, "exported NAV table should match the imported table")
}

// TestVaultGenesis_InitPanicsOnInvalidNAV verifies genesis validation rejects
// NAV entries that price a self-priced denom or carry a non-positive price or volume.
func (s *TestSuite) TestVaultGenesis_InitPanicsOnInvalidNAV() {
	shareDenom := "navshare"
	underlying := "navunder"
	vaultAddr := types.GetVaultAddress(shareDenom)

	tests := []struct {
		name        string
		nav         types.VaultNAV
		expectPanic string
	}{
		{
			name: "prices the vault share denom",
			nav: types.VaultNAV{
				Denom:       shareDenom,
				Price:       sdk.NewInt64Coin(underlying, 1),
				Volume:      sdkmath.NewInt(1),
				UpdatedTime: time.Unix(1700000000, 0).UTC(),
			},
			expectPanic: "invalid vault genesis state: nav entry at index 0 prices the vault share denom navshare",
		},
		{
			name: "self-referential price denom",
			nav: types.VaultNAV{
				Denom:       "rwa",
				Price:       sdk.NewInt64Coin("rwa", 100),
				Volume:      sdkmath.NewInt(1),
				UpdatedTime: time.Unix(1700000000, 0).UTC(),
			},
			expectPanic: "invalid vault genesis state: nav entry at index 0 has matching denom and price denom \"rwa\"",
		},
		{
			name: "negative price",
			nav: types.VaultNAV{
				Denom:       "rwa",
				Price:       sdk.Coin{Denom: underlying, Amount: sdkmath.NewInt(-1)},
				Volume:      sdkmath.NewInt(1),
				UpdatedTime: time.Unix(1700000000, 0).UTC(),
			},
			expectPanic: "invalid vault genesis state: invalid nav price at index 0: negative coin amount: -1",
		},
		{
			name: "zero volume",
			nav: types.VaultNAV{
				Denom:       "rwa",
				Price:       sdk.NewInt64Coin(underlying, 1),
				Volume:      sdkmath.ZeroInt(),
				UpdatedTime: time.Unix(1700000000, 0).UTC(),
			},
			expectPanic: "invalid vault genesis state: nav volume at index 0 must be positive",
		},
		{
			name: "negative volume",
			nav: types.VaultNAV{
				Denom:       "rwa",
				Price:       sdk.NewInt64Coin(underlying, 1),
				Volume:      sdkmath.NewInt(-1),
				UpdatedTime: time.Unix(1700000000, 0).UTC(),
			},
			expectPanic: "invalid vault genesis state: nav volume at index 0 must be positive",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			genesis := buildSingleVaultGenesisState(shareDenom, underlying, s.adminAddr.String(),
				[]types.VaultNAVEntry{{VaultAddress: vaultAddr.String(), Nav: tt.nav}})
			s.Require().PanicsWithError(tt.expectPanic, func() { s.k.InitGenesis(s.ctx, genesis) }, "InitGenesis should panic on an invalid NAV entry")
		})
	}
}

// TestVaultGenesis_InitPanicsOnUnregisteredNAVMarker verifies InitGenesis rejects
// a NAV entry whose denom passes stateless genesis validation but is not a
// registered marker on-chain, matching the invariant enforced by the runtime
// SetVaultNAV path.
func (s *TestSuite) TestVaultGenesis_InitPanicsOnUnregisteredNAVMarker() {
	shareDenom := "navmkrshare"
	underlying := "navmkrunder"
	vaultAddr := types.GetVaultAddress(shareDenom)

	genesis := buildSingleVaultGenesisState(shareDenom, underlying, s.adminAddr.String(),
		[]types.VaultNAVEntry{{
			VaultAddress: vaultAddr.String(),
			Nav: types.VaultNAV{
				Denom:       "unregisteredrwa",
				Price:       sdk.NewInt64Coin(underlying, 100),
				Volume:      sdkmath.NewInt(1),
				UpdatedTime: time.Unix(1700000000, 0).UTC(),
			},
		}})

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		s.k.InitGenesis(s.ctx, genesis)
	}()

	s.Require().NotNil(recovered, "InitGenesis should panic when a NAV denom is not a registered marker")
	err, ok := recovered.(error)
	s.Require().True(ok, "panic value should be an error, got %T", recovered)
	s.Require().ErrorContains(err, `nav denom "unregisteredrwa"`)
	s.Require().ErrorContains(err, "is not a registered marker")
}

// TestVaultGenesis_ExportNAVs_MultipleVaults verifies ExportGenesis includes NAV
// entries from all vaults, not just the first one.
func (s *TestSuite) TestVaultGenesis_ExportNAVs_MultipleVaults() {
	shareDenomA := "shareA"
	shareDenomB := "shareB"
	underlyingA := "underA"
	underlyingB := "underB"
	admin := s.adminAddr.String()
	vaultAddrA := types.GetVaultAddress(shareDenomA)
	vaultAddrB := types.GetVaultAddress(shareDenomB)

	vaultA := makeGenesisVaultAccount(shareDenomA, underlyingA, admin)
	vaultB := makeGenesisVaultAccount(shareDenomB, underlyingB, admin)

	navEntriesA := []types.VaultNAVEntry{
		{
			VaultAddress: vaultAddrA.String(),
			Nav: types.VaultNAV{
				Denom:              "rwa1",
				Price:              sdk.NewInt64Coin(underlyingA, 100),
				Volume:             sdkmath.NewInt(3),
				UpdatedBlockHeight: 5,
			},
		},
		{
			VaultAddress: vaultAddrA.String(),
			Nav: types.VaultNAV{
				Denom:              "rwa2",
				Price:              sdk.NewInt64Coin(underlyingA, 200),
				Volume:             sdkmath.NewInt(7),
				UpdatedBlockHeight: 5,
			},
		},
	}
	navEntriesB := []types.VaultNAVEntry{
		{
			VaultAddress: vaultAddrB.String(),
			Nav: types.VaultNAV{
				Denom:              "bond",
				Price:              sdk.NewInt64Coin(underlyingB, 999),
				Volume:             sdkmath.NewInt(1),
				UpdatedBlockHeight: 5,
			},
		},
	}

	allNavs := append(navEntriesA, navEntriesB...)
	genesis := &types.GenesisState{
		Params: types.DefaultParams(),
		Vaults: []types.VaultAccount{vaultA, vaultB},
		Navs:   allNavs,
	}

	for _, entry := range allNavs {
		s.requireSimpleMarker(entry.Nav.Denom)
	}

	s.k.InitGenesis(s.ctx, genesis)

	exported := s.k.ExportGenesis(s.ctx)
	s.Require().Len(exported.Navs, len(allNavs), "ExportGenesis should include NAVs from all vaults")
	s.Assert().ElementsMatch(allNavs, exported.Navs, "exported NAV table should match the imported table across both vaults")
}

// TestVaultGenesis_ExportPanicsOnNAVKeyValueDenomMismatch verifies that ExportGenesis
// surfaces a key/value denom mismatch in the NAVs collection instead of silently
// dropping the key denom. The SetVaultNAV write path enforces equality by construction,
// so this guard exists to catch any future write path that violates the invariant.
func (s *TestSuite) TestVaultGenesis_ExportPanicsOnNAVKeyValueDenomMismatch() {
	shareDenom := "navmismatch"
	underlying := "navmismatchunder"
	vaultAddr := types.GetVaultAddress(shareDenom)

	genesis := &types.GenesisState{
		Params: types.DefaultParams(),
		Vaults: []types.VaultAccount{makeGenesisVaultAccount(shareDenom, underlying, s.adminAddr.String())},
	}
	s.k.InitGenesis(s.ctx, genesis)

	nav := types.VaultNAV{
		Denom:       "valuedenom",
		Price:       sdk.NewInt64Coin(underlying, 1),
		Volume:      sdkmath.NewInt(1),
		UpdatedTime: time.Unix(1700000000, 0).UTC(),
	}
	s.Require().NoError(
		s.k.NAVs.Set(s.ctx, collections.Join(vaultAddr, "keydenom"), nav),
		"writing a mismatched NAV row should succeed at the collection level",
	)

	expectedPanic := fmt.Sprintf(`nav key/value denom mismatch for vault %s: key="keydenom" value="valuedenom"`, vaultAddr)
	s.Require().PanicsWithError(
		"failed to walk vault navs: "+expectedPanic,
		func() { s.k.ExportGenesis(s.ctx) },
		"ExportGenesis should panic when a NAV row's key denom does not match value.Denom",
	)
}
