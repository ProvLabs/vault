package keeper_test

import (
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
		UnderlyingAssets:    []string{underlying},
		CurrentInterestRate: types.ZeroInterestRate,
		DesiredInterestRate: types.ZeroInterestRate,
	}

	genesis := &types.GenesisState{
		Vaults: []types.VaultAccount{vault},
	}

	s.k.InitGenesis(s.ctx, genesis)

	exported := s.k.ExportGenesis(s.ctx)

	exp := exported.Vaults[0]
	s.Require().Equal(vault.GetAddress().String(), exp.GetAddress().String())
	s.Require().Equal(vault.Admin, exp.Admin)
	s.Require().Equal(vault.ShareDenom, exp.ShareDenom)
	s.Require().Equal(vault.UnderlyingAssets, exp.UnderlyingAssets)
}
