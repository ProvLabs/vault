package simulation

import (
	"encoding/json"
	"fmt"

	"github.com/provlabs/vault/types"

	"github.com/cosmos/cosmos-sdk/types/module"
)

// RandomizedGenState generates a random GenesisState for the vault module
func RandomizedGenState(simState *module.SimulationState) {
	techFeeAddr := simState.Accounts[simState.Rand.Intn(len(simState.Accounts))].Address

	vaultGenesis := types.GenesisState{
		Vaults:              []types.VaultAccount{},
		PayoutTimeoutQueue:  []types.QueueEntry{},
		PendingSwapOutQueue: types.PendingSwapOutQueue{},
		Params: types.Params{
			TechFeeAddress:    techFeeAddr.String(),
			DefaultAumFeeBips: uint32(simState.Rand.Intn(1001)), // 0 to 1000 bips
		},
	}

	bz, err := json.MarshalIndent(&vaultGenesis, "", " ")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected randomly generated vault parameters: %s\n", bz)

	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&vaultGenesis)
}
