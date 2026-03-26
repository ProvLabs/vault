package simulation

import (
	"encoding/json"
	"fmt"
	"math/rand"

	"github.com/provlabs/vault/types"

	"github.com/cosmos/cosmos-sdk/types/module"
)

// RandomizedGenState generates a random GenesisState for the vault module
func RandomizedGenState(simState *module.SimulationState) {
	var aumFeeBips uint32
	simState.AppParams.GetOrGenerate(
		"aum_fee_bips", &aumFeeBips, simState.Rand,
		func(r *rand.Rand) { aumFeeBips = uint32(r.Intn(100)) },
	)

	vaultGenesis := types.GenesisState{
		Params: types.Params{
			DefaultAumFeeBips: aumFeeBips,
			TechFeeAddress:    "", // Let InitGenesis handle default TechFeeAddress based on chain prefix
		},
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
