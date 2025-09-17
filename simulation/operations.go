package simulation

import (
	"fmt"
	"math/rand"
	"slices"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

const (
	OpWeightMsgCreateVault     = "op_weight_msg_create_vault"
	OpWeightMsgSwapIn          = "op_weight_msg_swap_in"
	OpWeightMsgSwapOut         = "op_weight_msg_swap_out"
	OpWeightMsgSetInterestRate = "op_weight_msg_set_interest_rate"
	OpWeightMsgToggleSwapIn    = "op_weight_msg_toggle_swap_in"
	OpWeightMsgToggleSwapOut   = "op_weight_msg_toggle_swap_out"
)

const (
	DefaultWeightMsgCreateVault     = 5
	DefaultWeightMsgSwapIn          = 40
	DefaultWeightMsgSwapOut         = 20
	DefaultWeightMsgSetInterestRate = 15
	DefaultWeightMsgToggleSwapIn    = 10
	DefaultWeightMsgToggleSwapOut   = 10
)

func WeightedOperations(simState module.SimulationState, k keeper.Keeper) simulation.WeightedOperations {
	var (
		wCreateVault        int
		wSwapIn             int
		wSwapOut            int
		wUpdateInterestRate int
		wToggleSwapIn       int
		wToggleSwapOut      int
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

	simState.AppParams.GetOrGenerate(
		OpWeightMsgSwapOut,
		&wSwapOut,
		simState.Rand,
		func(r *rand.Rand) {
			wSwapOut = DefaultWeightMsgSwapOut
		},
	)

	simState.AppParams.GetOrGenerate(
		OpWeightMsgSetInterestRate,
		&wUpdateInterestRate,
		simState.Rand,
		func(r *rand.Rand) {
			wUpdateInterestRate = DefaultWeightMsgSetInterestRate
		},
	)

	simState.AppParams.GetOrGenerate(
		OpWeightMsgToggleSwapIn,
		&wToggleSwapIn,
		simState.Rand,
		func(r *rand.Rand) {
			wToggleSwapIn = DefaultWeightMsgToggleSwapIn
		},
	)

	simState.AppParams.GetOrGenerate(
		OpWeightMsgToggleSwapOut,
		&wToggleSwapOut,
		simState.Rand,
		func(r *rand.Rand) {
			wToggleSwapOut = DefaultWeightMsgToggleSwapOut
		},
	)

	return simulation.WeightedOperations{
		simulation.NewWeightedOperation(wCreateVault, SimulateMsgCreateVault(k)),
		simulation.NewWeightedOperation(wSwapIn, SimulateMsgSwapIn(k)),
		simulation.NewWeightedOperation(wSwapOut, SimulateMsgSwapOut(k)),
		simulation.NewWeightedOperation(wUpdateInterestRate, SimulateMsgUpdateInterestRate(k)),
		simulation.NewWeightedOperation(wToggleSwapIn, SimulateMsgToggleSwapIn(k)),
		simulation.NewWeightedOperation(wToggleSwapOut, SimulateMsgToggleSwapOut(k)),
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
		underlyingDenom := "underlying"

		// Obtain Underlying Asset
		balance := k.BankKeeper.GetBalance(ctx, acc.Address, underlyingDenom)
		underlyingAsset := balance.Denom

		// Pick a random vault with matching underlying asset
		vaults, _ := k.GetVaults(ctx)
		iter := utils.Filter(vaults, func(addr sdk.AccAddress) bool {
			vault, _ := k.GetVault(ctx, addr)
			return vault.UnderlyingAsset == underlyingAsset
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

func SimulateMsgSwapOut(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Find an account with the asset
		var acc simtypes.Account
		denom := "underlyingshare"
		for _, a := range accs {
			balance := k.BankKeeper.GetBalance(ctx, a.Address, denom)
			if balance.IsZero() {
				continue
			}
			acc = a
		}

		// Obtain amount of shares
		balance := k.BankKeeper.GetBalance(ctx, acc.Address, denom)

		// Pick a random vault with matching share denom
		vaults, _ := k.GetVaults(ctx)
		iter := utils.Filter(vaults, func(addr sdk.AccAddress) bool {
			vault, _ := k.GetVault(ctx, addr)
			return vault.ShareDenom == denom
		})
		vaults = slices.Collect(iter)
		vaultAddr := sdk.AccAddress{}
		if len(vaults) > 0 {
			vaultAddr = vaults[r.Intn(len(vaults))]
		}

		// Pick a random amount of the underlying asset
		maxAmount := balance.Amount.Int64()
		amount := rand.Int63n(maxAmount-1) + 1
		assets := sdk.NewInt64Coin(denom, amount)

		msg := &types.MsgSwapOutRequest{
			Owner:        acc.Address.String(),
			VaultAddress: vaultAddr.String(),
			Assets:       assets,
		}

		handler := keeper.NewMsgServer(&k)
		_, err := handler.SwapOut(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, ""), nil, nil
	}
}

func SimulateMsgUpdateInterestRate(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Get a random vault
		vaults, err := k.GetVaults(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateInterestRateRequest{}), "unable to get vaults"), nil, err
		}
		if len(vaults) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateInterestRateRequest{}), "no vaults found"), nil, nil
		}
		vaultAddr := vaults[r.Intn(len(vaults))]
		vault, err := k.GetVault(ctx, vaultAddr)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateInterestRateRequest{}), "unable to get vault"), nil, err
		}
		if vault == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateInterestRateRequest{}), "received nil vault"), nil, err
		}

		// Use the vault admin to sign
		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateInterestRateRequest{}), "invalid admin address for vault"), nil, err
		}

		rate := r.Float64()*3 - 1

		msg := &types.MsgUpdateInterestRateRequest{
			VaultAddress: vaultAddr.String(),
			Admin:        adminAddr.String(),
			NewRate:      fmt.Sprintf("%f", rate),
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.UpdateInterestRate(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully set interest rate"), nil, nil
	}
}

func SimulateMsgToggleSwapIn(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Get a random vault
		vaults, err := k.GetVaults(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgToggleSwapInRequest{}), "unable to get vaults"), nil, err
		}
		if len(vaults) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgToggleSwapInRequest{}), "no vaults found"), nil, nil
		}
		vaultAddr := vaults[r.Intn(len(vaults))]
		vault, err := k.GetVault(ctx, vaultAddr)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgToggleSwapInRequest{}), "unable to get vault"), nil, err
		}
		if vault == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgToggleSwapInRequest{}), "received nil vault"), nil, nil
		}

		// Use the vault admin to sign
		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgToggleSwapInRequest{}), "invalid admin address"), nil, err
		}

		msg := &types.MsgToggleSwapInRequest{
			VaultAddress: vaultAddr.String(),
			Admin:        adminAddr.String(),
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.ToggleSwapIn(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully toggled swap in"), nil, nil
	}
}

func SimulateMsgToggleSwapOut(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Get a random vault
		vaults, err := k.GetVaults(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgToggleSwapOutRequest{}), "unable to get vaults"), nil, err
		}
		if len(vaults) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgToggleSwapOutRequest{}), "no vaults found"), nil, nil
		}
		vaultAddr := vaults[r.Intn(len(vaults))]
		vault, err := k.GetVault(ctx, vaultAddr)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgToggleSwapOutRequest{}), "unable to get vault"), nil, err
		}
		if vault == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgToggleSwapOutRequest{}), "received nil vault"), nil, nil
		}

		// Use the vault admin to sign
		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgToggleSwapOutRequest{}), "invalid admin address"), nil, err
		}

		msg := &types.MsgToggleSwapOutRequest{
			VaultAddress: vaultAddr.String(),
			Admin:        adminAddr.String(),
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.ToggleSwapOut(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully toggled swap out"), nil, nil
	}
}
