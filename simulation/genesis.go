package simulation

import (
	"encoding/json"
	"fmt"

	"github.com/provlabs/vault/types"

	"github.com/cosmos/cosmos-sdk/types/module"
)

// RandomizedGenState generates a random GenesisState for the vault module
func RandomizedGenState(simState *module.SimulationState) {
	vaultGenesis := types.GenesisState{
		Vaults:              []types.VaultAccount{},
		PayoutTimeoutQueue:  []types.QueueEntry{},
		PendingSwapOutQueue: types.PendingSwapOutQueue{},
	}

	bz, err := json.MarshalIndent(&vaultGenesis, "", " ")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected randomly generated vault parameters: %s\n", bz)

	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&vaultGenesis)
}
