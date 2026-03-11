package keeper_test

import (
	"time"

	"cosmossdk.io/math"
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
	genesis := &types.GenesisState{
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

func (s *TestSuite) TestVaultGenesis_RoundTrip_PastAndFutureTimeouts() {
	shareDenom := "vaultshare2"
	underlying := "under2"
	vaultAddr := types.GetVaultAddress(shareDenom)
	admin := s.adminAddr.String()

	vault := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               admin,
		TotalShares:         sdk.NewInt64Coin(shareDenom, 0),
		UnderlyingAsset:     underlying,
		PaymentDenom:        underlying,
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}

	now := time.Now()
	past := now.Add(-10 * time.Minute).Unix()
	future := now.Add(10 * time.Minute).Unix()

	genesis := &types.GenesisState{
		Vaults: []types.VaultAccount{vault},
		PayoutTimeoutQueue: []types.QueueEntry{
			{Time: uint64(past), Addr: vaultAddr.String()},
			{Time: uint64(future), Addr: vaultAddr.String()},
		},
		PendingSwapOutQueue: types.PendingSwapOutQueue{},
	}

	s.k.InitGenesis(s.ctx, genesis)
	s.ctx = s.ctx.WithBlockTime(now)

	exported := s.k.ExportGenesis(s.ctx)
	s.Require().Len(exported.PayoutTimeoutQueue, 2, "exported genesis should contain exactly two payout timeout entries")
	s.Require().Equal(vaultAddr.String(), exported.PayoutTimeoutQueue[0].Addr, "first payout timeout entry address mismatch")
	s.Require().Equal(uint64(past), exported.PayoutTimeoutQueue[0].Time, "first payout timeout entry time mismatch")
	s.Require().Equal(vaultAddr.String(), exported.PayoutTimeoutQueue[1].Addr, "second payout timeout entry address mismatch")
	s.Require().Equal(uint64(future), exported.PayoutTimeoutQueue[1].Time, "second payout timeout entry time mismatch")
}

func (s *TestSuite) TestVaultGenesis_InvalidTimeoutAddressPanics() {
	shareDenom := "vaultshare3"
	underlying := "under3"
	vaultAddr := types.GetVaultAddress(shareDenom)
	admin := s.adminAddr.String()

	vault := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               admin,
		TotalShares:         sdk.NewInt64Coin(shareDenom, 0),
		UnderlyingAsset:     underlying,
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}

	genesis := &types.GenesisState{
		Vaults: []types.VaultAccount{vault},
		PayoutTimeoutQueue: []types.QueueEntry{
			{Time: uint64(time.Now().Unix()), Addr: "not-bech32"},
		},
		PendingSwapOutQueue: types.PendingSwapOutQueue{},
	}

	s.Require().Panics(func() { s.k.InitGenesis(s.ctx, genesis) }, "InitGenesis should panic on invalid timeout address")
}

func (s *TestSuite) TestVaultGenesis_ExistingAccountNumberCopied() {
	shareDenom := "vaultshare4"
	underlying := "under4"
	admin := s.adminAddr.String()
	vaultAddr := types.GetVaultAddress(shareDenom)

	existing := authtypes.NewBaseAccountWithAddress(vaultAddr)
	s.Require().NoError(existing.SetAccountNumber(7), "failed to set account number for existing account")
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
		Vaults:              []types.VaultAccount{vault},
		PendingSwapOutQueue: types.PendingSwapOutQueue{},
	}

	s.k.InitGenesis(s.ctx, genesis)
	exported := s.k.ExportGenesis(s.ctx)
	s.Require().Len(exported.Vaults, 1, "exported genesis should contain exactly one vault")
	s.Require().Equal(uint64(7), exported.Vaults[0].GetAccountNumber(), "exported vault account number mismatch")
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
		TotalShares:         sdk.Coin{Denom: "invalid denom!", Amount: math.NewInt(0)},
		UnderlyingAsset:     "underX",
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}
	genesis := &types.GenesisState{Vaults: []types.VaultAccount{vault}}
	s.Require().Panics(func() { s.k.InitGenesis(s.ctx, genesis) }, "InitGenesis should panic on invalid genesis state")
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
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}

	genesis := &types.GenesisState{
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
	s.Require().Panics(func() { s.k.InitGenesis(s.ctx, genesis) }, "InitGenesis should panic on invalid genesis state")
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
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}

	genesis := &types.GenesisState{
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
	s.Require().Panics(func() { s.k.InitGenesis(s.ctx, genesis) }, "InitGenesis should panic on invalid genesis state")
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
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}

	genesis := &types.GenesisState{
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
	s.Require().Panics(func() { s.k.InitGenesis(s.ctx, genesis) }, "InitGenesis should panic on invalid genesis state")
}
