package simulation

import (
	"fmt"
	"math/rand"
	"slices"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
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
		simulation.NewWeightedOperation(wSwapIn, SimulateMsgSwapIn(k)),
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

func SimulateMsgSwapIn(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Pick a random account
		acc, _ := simtypes.RandomAcc(r, accs)
		underlyingDenom := "stake"

		// Obtain Underlying Asset
		balance := k.BankKeeper.GetBalance(ctx, acc.Address, underlyingDenom)
		underlyingAsset := balance.Denom

		// Pick a random vault with matching underlying asset
		vaults, _ := k.GetVaults(ctx)
		iter := utils.Filter(vaults, func(addr sdk.AccAddress) bool {
			vault, _ := k.GetVault(ctx, addr)
			return vault.UnderlyingAssets[0] == underlyingAsset
		})
		vaults = slices.Collect(iter)
		vaultAddr := sdk.AccAddress{}
		if len(vaults) > 0 {
			vaultAddr = vaults[r.Intn(len(vaults))]
		}

		// Pick a random amount of the underlying asset
		maxAmount := balance.Amount.Int64()
		amount := rand.Int63n(maxAmount-1) + 1
		assets := sdk.NewInt64Coin(underlyingAsset, amount)

		msg := &types.MsgSwapInRequest{
			Owner:        acc.Address.String(),
			VaultAddress: vaultAddr.String(),
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
