package keeper_test

import (
	"fmt"
	"math"
	"time"

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

	vault := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               admin,
		TotalShares:         sdk.NewInt64Coin(shareDenom, 0),
		UnderlyingAsset:     underlying,
		PaymentDenom:        underlying,
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}

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
	vault := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(types.GetVaultAddress("panic-vault")),
		Admin:               s.adminAddr.String(),
		TotalShares:         sdk.NewInt64Coin("panic-vault", 0),
		UnderlyingAsset:     "undercoin",
		PaymentDenom:        "undercoin",
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}

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

	vault := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               admin,
		TotalShares:         sdk.NewInt64Coin(shareDenom, 0),
		UnderlyingAsset:     underlying,
		PaymentDenom:        underlying,
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
		FeePeriodTimeout:    future,
	}

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

	vault1 := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr1),
		Admin:               admin,
		TotalShares:         sdk.NewInt64Coin(shareDenom1, 0),
		UnderlyingAsset:     underlying1,
		PaymentDenom:        underlying1,
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
		PeriodTimeout:       past,
	}

	vault2 := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr2),
		Admin:               admin,
		TotalShares:         sdk.NewInt64Coin(shareDenom2, 0),
		UnderlyingAsset:     underlying2,
		PaymentDenom:        underlying2,
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
		PeriodTimeout:       future,
	}

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
	vaultAddr := types.GetVaultAddress(shareDenom)
	admin := s.adminAddr.String()

	now := uint64(time.Now().Unix())

	vault := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               admin,
		TotalShares:         sdk.NewInt64Coin(shareDenom, 0),
		UnderlyingAsset:     underlying,
		PaymentDenom:        underlying,
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
		PeriodTimeout:       int64(now),
	}

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

	vault := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               admin,
		TotalShares:         sdk.NewInt64Coin(shareDenom, 0),
		UnderlyingAsset:     underlying,
		PaymentDenom:        underlying,
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}

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
	vaultAddr := types.GetVaultAddress(shareDenom)

	vault := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               admin,
		TotalShares:         sdk.NewInt64Coin(shareDenom, 0),
		UnderlyingAsset:     underlying,
		PaymentDenom:        underlying,
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}

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
	vaultAddr := types.GetVaultAddress(shareDenom)
	badVaultAddr := types.GetVaultAddress("baddenom")

	vault := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               admin,
		TotalShares:         sdk.NewInt64Coin(shareDenom, 0),
		UnderlyingAsset:     underlying,
		PaymentDenom:        underlying,
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}

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
	vaultAddr := types.GetVaultAddress(shareDenom)

	vault := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               admin,
		TotalShares:         sdk.NewInt64Coin(shareDenom, 0),
		UnderlyingAsset:     underlying,
		PaymentDenom:        underlying,
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}

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
