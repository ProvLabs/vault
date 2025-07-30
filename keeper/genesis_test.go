package keeper_test

import (
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/provlabs/vault/keeper"
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
		UnderlyingAssets:    []string{underlying},
		CurrentInterestRate: keeper.ZeroInterestRate,
		DesiredInterestRate: keeper.ZeroInterestRate,
	}

	genesis := &types.GenesisState{
		Params: types.DefaultParams(),
		Vaults: []types.VaultAccount{vault},
	}

	s.k.InitGenesis(s.ctx, genesis)

	exported := s.k.ExportGenesis(s.ctx)

	s.Require().Equal(genesis.Params, exported.Params)
	s.Require().Len(exported.Vaults, 1)

	exp := exported.Vaults[0]
	s.Require().Equal(vault.GetAddress().String(), exp.GetAddress().String())
	s.Require().Equal(vault.Admin, exp.Admin)
	s.Require().Equal(vault.ShareDenom, exp.ShareDenom)
	s.Require().Equal(vault.UnderlyingAssets, exp.UnderlyingAssets)
}
