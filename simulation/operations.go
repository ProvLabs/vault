package simulation

import (
	"fmt"
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
)

const (
	OpWeightMsgCreateVault = "op_weight_msg_create_vault"
	OpWeightMsgSwapIn      = "op_weight_msg_swap_in"
)

const (
	DefaultWeightMsgCreateVault = 100
	DefaultWeightMsgSwapIn      = 50
)

func WeightedOperations(simState module.SimulationState, k keeper.Keeper) simulation.WeightedOperations {
	var (
		wCreateVault int
		wSwapIn      int
	)

	simState.AppParams.GetOrGenerate(
		OpWeightMsgCreateVault,
		&wCreateVault,
		simState.Rand,
		func(r *rand.Rand) {
			wCreateVault = DefaultWeightMsgCreateVault
		},
	)

	simState.AppParams.GetOrGenerate(
		OpWeightMsgSwapIn,
		&wSwapIn,
		simState.Rand,
		func(r *rand.Rand) {
			wSwapIn = DefaultWeightMsgSwapIn
		},
	)

	return simulation.WeightedOperations{
		simulation.NewWeightedOperation(wCreateVault, SimulateMsgCreateVault(k)),
		simulation.NewWeightedOperation(wSwapIn, SimulateMsgSwapIn(
			k,
			DefaultVaultAddrSelector,
			DefaultUnderlyingAssetSelector,
			DefaultUnderlyingAmountSelector,
		)),
	}
}

func SimulateMsgCreateVault(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		admin, _ := simtypes.RandomAcc(r, accs)

		denom := fmt.Sprintf("vaulttoken%d", r.Intn(100000))
		underlying := fmt.Sprintf("underlying%d", r.Intn(100000))

		// Simulate the marker existing
		grants := []markertypes.AccessGrant{
			{
				Address: admin.Address.String(),
				Permissions: markertypes.AccessList{
					markertypes.Access_Mint, markertypes.Access_Burn,
					markertypes.Access_Deposit, markertypes.Access_Withdraw, markertypes.Access_Delete,
				},
			},
		}
		underlyingMarker := markertypes.NewEmptyMarkerAccount(underlying, admin.Address.String(), grants)
		underlyingMarker.MarkerType = markertypes.MarkerType_Coin
		k.MarkerKeeper.AddFinalizeAndActivateMarker(ctx, underlyingMarker)

		msg := &types.MsgCreateVaultRequest{
			Admin:           admin.Address.String(),
			ShareDenom:      denom,
			UnderlyingAsset: underlying,
		}

		handler := keeper.NewMsgServer(&k)
		_, err := handler.CreateVault(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, ""), nil, nil
	}
}

type VaultAddrSelector func(r *rand.Rand) string

func DefaultVaultAddrSelector(r *rand.Rand) string {
	// Pick a vault address
	// 1. A random address
	// 2. One of the pre-existing vaults
	// 3. Bad address
	// 4. One with a specific denom?
	return ""
}

type UnderlyingAssetSelector func(r *rand.Rand, acc sdk.AccAddress) string

func DefaultUnderlyingAssetSelector(r *rand.Rand, acc sdk.AccAddress) string {
	// Pick a denom
	// 1. A random working denom
	// 2. One of their assets
	// 3. Bad denom
	return ""
}

type UnderlyingAmountSelector func(r *rand.Rand, acc sdk.AccAddress, denom string) sdk.Coin

func DefaultUnderlyingAmountSelector(r *rand.Rand, acc sdk.AccAddress, denom string) sdk.Coin {
	// Pick amount for the denom
	// 1. Just a random amount from their [0, supply * 2)
	// 2. If it doesn't exist then just a random amount [0, 10000]
	return sdk.NewInt64Coin(denom, r.Int63n(100000))
}

func SimulateMsgSwapIn(k keeper.Keeper, vaultSelector VaultAddrSelector, assetSelector UnderlyingAssetSelector, amountSelector UnderlyingAmountSelector) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		admin, _ := simtypes.RandomAcc(r, accs)

		/*
			denom := fmt.Sprintf("vaulttoken%d", r.Intn(100000))
			underlying := fmt.Sprintf("underlying%d", r.Intn(100000))
			amount := r.Int63n(100000)
		*/

		vaultAddr := vaultSelector(r)
		underlyingAsset := assetSelector(r, admin.Address)
		assets := amountSelector(r, admin.Address, underlyingAsset)

		msg := &types.MsgSwapInRequest{
			Owner:        admin.Address.String(),
			VaultAddress: vaultAddr,
			Assets:       assets,
		}

		handler := keeper.NewMsgServer(&k)
		_, err := handler.SwapIn(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, ""), nil, nil
	}
}
