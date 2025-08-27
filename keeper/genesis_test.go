package keeper_test

import (
	"time"

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
		ShareDenom:          shareDenom,
		UnderlyingAsset:     underlying,
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}

	past := time.Now().Add(-5 * time.Minute).Unix()
	genesis := &types.GenesisState{
		Vaults: []types.VaultAccount{vault},
		PayoutTimeoutQueue: []types.QueueEntry{
			{Time: uint64(past), Addr: vaultAddr.String()},
		},
	}

	s.k.InitGenesis(s.ctx, genesis)
	s.ctx = s.ctx.WithBlockTime(time.Unix(past+60, 0))

	exported := s.k.ExportGenesis(s.ctx)
	s.Require().Len(exported.Vaults, 1)
	exp := exported.Vaults[0]
	s.Require().Equal(vault.GetAddress().String(), exp.GetAddress().String())
	s.Require().Equal(vault.Admin, exp.Admin)
	s.Require().Equal(vault.ShareDenom, exp.ShareDenom)
	s.Require().Equal(vault.UnderlyingAsset, exp.UnderlyingAsset)

	s.Require().Len(exported.PayoutTimeoutQueue, 1)
	s.Require().Equal(vaultAddr.String(), exported.PayoutTimeoutQueue[0].Addr)
	s.Require().Equal(uint64(past), exported.PayoutTimeoutQueue[0].Time)

	got, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Require().Equal(vaultAddr.String(), got.GetAddress().String())
}

func (s *TestSuite) TestVaultGenesis_RoundTrip_PastAndFutureTimeouts() {
	shareDenom := "vaultshare2"
	underlying := "under2"
	vaultAddr := types.GetVaultAddress(shareDenom)
	admin := s.adminAddr.String()

	vault := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               admin,
		ShareDenom:          shareDenom,
		UnderlyingAsset:     underlying,
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
	}

	s.k.InitGenesis(s.ctx, genesis)
	s.ctx = s.ctx.WithBlockTime(now)

	exported := s.k.ExportGenesis(s.ctx)
	s.Require().Len(exported.PayoutTimeoutQueue, 2)
	s.Require().Equal(vaultAddr.String(), exported.PayoutTimeoutQueue[0].Addr)
	s.Require().Equal(uint64(past), exported.PayoutTimeoutQueue[0].Time)
	s.Require().Equal(vaultAddr.String(), exported.PayoutTimeoutQueue[1].Addr)
	s.Require().Equal(uint64(future), exported.PayoutTimeoutQueue[1].Time)
}

func (s *TestSuite) TestVaultGenesis_InvalidTimeoutAddressPanics() {
	shareDenom := "vaultshare3"
	underlying := "under3"
	vaultAddr := types.GetVaultAddress(shareDenom)
	admin := s.adminAddr.String()

	vault := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               admin,
		ShareDenom:          shareDenom,
		UnderlyingAsset:     underlying,
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}

	genesis := &types.GenesisState{
		Vaults: []types.VaultAccount{vault},
		PayoutTimeoutQueue: []types.QueueEntry{
			{Time: uint64(time.Now().Unix()), Addr: "not-bech32"},
		},
	}

	s.Require().Panics(func() { s.k.InitGenesis(s.ctx, genesis) })
}

func (s *TestSuite) TestVaultGenesis_ExistingAccountNumberCopied() {
	shareDenom := "vaultshare4"
	underlying := "under4"
	admin := s.adminAddr.String()
	vaultAddr := types.GetVaultAddress(shareDenom)

	existing := authtypes.NewBaseAccountWithAddress(vaultAddr)
	s.Require().NoError(existing.SetAccountNumber(7))
	s.k.AuthKeeper.SetAccount(s.ctx, existing)

	vault := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               admin,
		ShareDenom:          shareDenom,
		UnderlyingAsset:     underlying,
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}

	genesis := &types.GenesisState{
		Vaults: []types.VaultAccount{vault},
	}

	s.k.InitGenesis(s.ctx, genesis)
	exported := s.k.ExportGenesis(s.ctx)
	s.Require().Len(exported.Vaults, 1)
	s.Require().Equal(uint64(7), exported.Vaults[0].GetAccountNumber())
}

func (s *TestSuite) TestVaultGenesis_InitNilDoesNothing() {
	s.k.InitGenesis(s.ctx, nil)
	exported := s.k.ExportGenesis(s.ctx)
	s.Require().Len(exported.Vaults, 0)
	s.Require().Len(exported.PayoutTimeoutQueue, 0)
}

func (s *TestSuite) TestVaultGenesis_InitPanicsOnInvalidVault() {
	vaultAddr := sdk.AccAddress("badaddr")
	vault := types.VaultAccount{
		BaseAccount:         authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:               "",
		ShareDenom:          "invalid denom!",
		UnderlyingAsset:     "underX",
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}
	genesis := &types.GenesisState{Vaults: []types.VaultAccount{vault}}
	s.Require().Panics(func() { s.k.InitGenesis(s.ctx, genesis) })
}
