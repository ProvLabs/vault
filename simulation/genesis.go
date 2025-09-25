package simulation

import (
	"encoding/json"
	"fmt"

	"github.com/provlabs/vault/types"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// RandomizedGenState generates a random GenesisState for the vault module
func RandomizedGenState(simState *module.SimulationState) {
	admin := simState.Accounts[0].Address.String()
	underlying := "underlying"

	vaults := []types.VaultAccount{}

	for i := 0; i < simState.Rand.Intn(5)+1; i++ {
		denom := fmt.Sprintf("vaultshare%d", i)
		addr := types.GetVaultAddress(denom)

		vaults = append(vaults, types.VaultAccount{
			BaseAccount:     authtypes.NewBaseAccountWithAddress(addr),
			Admin:           admin,
			TotalShares:     sdk.NewCoin(denom, sdkmath.ZeroInt()),
			UnderlyingAsset: underlying,
		})
	}

	vaultGenesis := types.GenesisState{
		Vaults: vaults,
	}

	bz, err := json.MarshalIndent(&vaultGenesis, "", " ")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected randomly generated vault parameters:\n%s\n", bz)

	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&vaultGenesis)
}
