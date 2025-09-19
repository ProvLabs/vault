package simulation_test

import (
	"encoding/json"
	"math/rand"
	"slices"
	"testing"

	sdkmath "cosmossdk.io/math"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/provlabs/vault/simulation"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
)

func TestRandomizedGenState(t *testing.T) {
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(interfaceRegistry)

	s := rand.NewSource(1)
	r := rand.New(s)

	simState := module.SimulationState{
		AppParams:    make(simtypes.AppParams),
		Cdc:          cdc,
		Rand:         r,
		NumBonded:    3,
		Accounts:     simtypes.RandomAccounts(r, 3),
		InitialStake: sdkmath.NewInt(1000),
		GenState:     make(map[string]json.RawMessage),
	}

	simulation.RandomizedGenState(&simState)

	var vaultGenesis types.GenesisState
	simState.Cdc.MustUnmarshalJSON(simState.GenState[types.ModuleName], &vaultGenesis)

	require.NotEmpty(t, vaultGenesis.Vaults)
	for _, v := range vaultGenesis.Vaults {
		require.NotEmpty(t, v.Admin)
		require.NotEmpty(t, v.ShareDenom)
		require.NotEmpty(t, v.GetAddress())
		require.NotEmpty(t, v.UnderlyingAsset)
	}

	for _, v := range vaultGenesis.PayoutTimeoutQueue {
		require.NotEmpty(t, v.Time)
		require.NotEmpty(t, v.Addr)
		seq := utils.Filter(vaultGenesis.Vaults, func(vault types.VaultAccount) bool {
			return vault.GetAddress().String() == v.Addr
		})
		require.NotEmpty(t, slices.Collect(seq), "must be valid vault address")
	}

	var maxID uint64 = 0
	ids := make(map[uint64]bool)
	for _, entry := range vaultGenesis.PendingSwapOutQueue.Entries {
		if entry.Id > maxID {
			maxID = entry.Id
		}
		// Check for unique IDs
		require.False(t, ids[entry.Id], "duplicate pending swap out queue ID")
		ids[entry.Id] = true

		require.NotEmpty(t, entry.Time, "pending swap out queue entry time is empty")
		swapOut := entry.SwapOut
		require.NotEmpty(t, swapOut.Owner, "pending swap out owner is empty")
		require.NotEmpty(t, swapOut.VaultAddress, "pending swap out vault address is empty")
		require.NotEmpty(t, swapOut, "pending swap out shares is zero")
	}
	require.GreaterOrEqual(t, vaultGenesis.PendingSwapOutQueue.LatestSequenceNumber, maxID)
}

func TestRandomizedGenState_Panics(t *testing.T) {
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(interfaceRegistry)

	s := rand.NewSource(1)
	r := rand.New(s)

	tests := []struct {
		name     string
		simState module.SimulationState
	}{
		{
			name:     "nil simState",
			simState: module.SimulationState{},
		},
		{
			name: "missing GenState map",
			simState: module.SimulationState{
				AppParams: make(simtypes.AppParams),
				Cdc:       cdc,
				Rand:      r,
				Accounts:  simtypes.RandomAccounts(r, 1),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Panics(t, func() {
				simulation.RandomizedGenState(&tt.simState)
			})
		})
	}
}
