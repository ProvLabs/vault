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
	OpWeightMsgCreateVault           = "op_weight_msg_create_vault"
	OpWeightMsgSwapIn                = "op_weight_msg_swap_in"
	OpWeightMsgSwapOut               = "op_weight_msg_swap_out"
	OpWeightMsgUpdateInterestRate    = "op_weight_msg_update_interest_rate"
	OpWeightMsgUpdateMinInterestRate = "op_weight_msg_update_min_interest_rate"
	OpWeightMsgUpdateMaxInterestRate = "op_weight_msg_update_max_interest_rate"
	OpWeightMsgToggleSwapIn          = "op_weight_msg_toggle_swap_in"
	OpWeightMsgToggleSwapOut         = "op_weight_msg_toggle_swap_out"
	OpWeightMsgDepositInterest       = "op_weight_msg_deposit_interest"
	OpWeightMsgWithdrawInterest      = "op_weight_msg_withdraw_interest"
	OpWeightMsgDepositPrincipal      = "op_weight_msg_deposit_principal"
	OpWeightMsgWithdrawPrincipal     = "op_weight_msg_withdraw_principal"
	OpWeightMsgExpediteSwap          = "op_weight_msg_expedite_swap"
	OpWeightMsgPauseVault            = "op_weight_msg_pause_vault"
	OpWeightMsgUnpauseVault          = "op_weight_msg_unpause_vault"
)

const (
	DefaultWeightMsgCreateVault           = 5
	DefaultWeightMsgSwapIn                = 35
	DefaultWeightMsgSwapOut               = 15
	DefaultWeightMsgUpdateInterestRate    = 10
	DefaultWeightMsgUpdateMinInterestRate = 5
	DefaultWeightMsgUpdateMaxInterestRate = 5
	DefaultWeightMsgToggleSwapIn          = 10
	DefaultWeightMsgToggleSwapOut         = 10
	DefaultWeightMsgDepositInterest       = 5
	DefaultWeightMsgWithdrawInterest      = 5
	DefaultWeightMsgDepositPrincipal      = 5
	DefaultWeightMsgWithdrawPrincipal     = 5
	DefaultWeightMsgExpediteSwap          = 2
	DefaultWeightMsgPauseVault            = 1
	DefaultWeightMsgUnpauseVault          = 1
)

func getRandomVault(r *rand.Rand, k keeper.Keeper, ctx sdk.Context) (*types.VaultAccount, error) {
	vaults, err := k.GetVaults(ctx)
	if err != nil {
		return nil, err
	}
	if len(vaults) == 0 {
		return nil, fmt.Errorf("no vaults found")
	}
	vaultAddr := vaults[r.Intn(len(vaults))]
	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil {
		return nil, err
	}
	if vault == nil {
		return nil, fmt.Errorf("received nil vault")
	}
	return vault, nil
}

func WeightedOperations(simState module.SimulationState, k keeper.Keeper) simulation.WeightedOperations {
	var (
		wCreateVault           int
		wSwapIn                int
		wSwapOut               int
		wUpdateInterestRate    int
		wUpdateMinInterestRate int
		wUpdateMaxInterestRate int
		wToggleSwapIn          int
		wToggleSwapOut         int
		wDepositInterest       int
		wWithdrawInterest      int
		wDepositPrincipal      int
		wWithdrawPrincipal     int
		wExpediteSwap          int
		wPauseVault            int
		wUnpauseVault          int
	)

	simState.AppParams.GetOrGenerate(OpWeightMsgCreateVault, &wCreateVault, simState.Rand, func(r *rand.Rand) { wCreateVault = DefaultWeightMsgCreateVault })
	simState.AppParams.GetOrGenerate(OpWeightMsgSwapIn, &wSwapIn, simState.Rand, func(r *rand.Rand) { wSwapIn = DefaultWeightMsgSwapIn })
	simState.AppParams.GetOrGenerate(OpWeightMsgSwapOut, &wSwapOut, simState.Rand, func(r *rand.Rand) { wSwapOut = DefaultWeightMsgSwapOut })
	simState.AppParams.GetOrGenerate(OpWeightMsgUpdateInterestRate, &wUpdateInterestRate, simState.Rand, func(r *rand.Rand) { wUpdateInterestRate = DefaultWeightMsgUpdateInterestRate })
	simState.AppParams.GetOrGenerate(OpWeightMsgUpdateMinInterestRate, &wUpdateMinInterestRate, simState.Rand, func(r *rand.Rand) { wUpdateMinInterestRate = DefaultWeightMsgUpdateMinInterestRate })
	simState.AppParams.GetOrGenerate(OpWeightMsgUpdateMaxInterestRate, &wUpdateMaxInterestRate, simState.Rand, func(r *rand.Rand) { wUpdateMaxInterestRate = DefaultWeightMsgUpdateMaxInterestRate })
	simState.AppParams.GetOrGenerate(OpWeightMsgToggleSwapIn, &wToggleSwapIn, simState.Rand, func(r *rand.Rand) { wToggleSwapIn = DefaultWeightMsgToggleSwapIn })
	simState.AppParams.GetOrGenerate(OpWeightMsgToggleSwapOut, &wToggleSwapOut, simState.Rand, func(r *rand.Rand) { wToggleSwapOut = DefaultWeightMsgToggleSwapOut })
	simState.AppParams.GetOrGenerate(OpWeightMsgDepositInterest, &wDepositInterest, simState.Rand, func(r *rand.Rand) { wDepositInterest = DefaultWeightMsgDepositInterest })
	simState.AppParams.GetOrGenerate(OpWeightMsgWithdrawInterest, &wWithdrawInterest, simState.Rand, func(r *rand.Rand) { wWithdrawInterest = DefaultWeightMsgWithdrawInterest })
	simState.AppParams.GetOrGenerate(OpWeightMsgDepositPrincipal, &wDepositPrincipal, simState.Rand, func(r *rand.Rand) { wDepositPrincipal = DefaultWeightMsgDepositPrincipal })
	simState.AppParams.GetOrGenerate(OpWeightMsgWithdrawPrincipal, &wWithdrawPrincipal, simState.Rand, func(r *rand.Rand) { wWithdrawPrincipal = DefaultWeightMsgWithdrawPrincipal })
	simState.AppParams.GetOrGenerate(OpWeightMsgExpediteSwap, &wExpediteSwap, simState.Rand, func(r *rand.Rand) { wExpediteSwap = DefaultWeightMsgExpediteSwap })
	simState.AppParams.GetOrGenerate(OpWeightMsgPauseVault, &wPauseVault, simState.Rand, func(r *rand.Rand) { wPauseVault = DefaultWeightMsgPauseVault })
	simState.AppParams.GetOrGenerate(OpWeightMsgUnpauseVault, &wUnpauseVault, simState.Rand, func(r *rand.Rand) { wUnpauseVault = DefaultWeightMsgUnpauseVault })

	return simulation.WeightedOperations{
		simulation.NewWeightedOperation(wCreateVault, SimulateMsgCreateVault(k)),
		simulation.NewWeightedOperation(wSwapIn, SimulateMsgSwapIn(k)),
		simulation.NewWeightedOperation(wSwapOut, SimulateMsgSwapOut(k)),
		simulation.NewWeightedOperation(wUpdateInterestRate, SimulateMsgUpdateInterestRate(k)),
		simulation.NewWeightedOperation(wUpdateMinInterestRate, SimulateMsgUpdateMinInterestRate(k)),
		simulation.NewWeightedOperation(wUpdateMaxInterestRate, SimulateMsgUpdateMaxInterestRate(k)),
		simulation.NewWeightedOperation(wToggleSwapIn, SimulateMsgToggleSwapIn(k)),
		simulation.NewWeightedOperation(wToggleSwapOut, SimulateMsgToggleSwapOut(k)),
		simulation.NewWeightedOperation(wDepositInterest, SimulateMsgDepositInterestFunds(k)),
		simulation.NewWeightedOperation(wWithdrawInterest, SimulateMsgWithdrawInterestFunds(k)),
		simulation.NewWeightedOperation(wDepositPrincipal, SimulateMsgDepositPrincipalFunds(k)),
		simulation.NewWeightedOperation(wWithdrawPrincipal, SimulateMsgWithdrawPrincipalFunds(k)),
		simulation.NewWeightedOperation(wExpediteSwap, SimulateMsgExpeditePendingSwapOut(k)),
		simulation.NewWeightedOperation(wPauseVault, SimulateMsgPauseVault(k)),
		simulation.NewWeightedOperation(wUnpauseVault, SimulateMsgUnpauseVault(k)),
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

		return simtypes.NewOperationMsg(msg, true, "successfully updated interest rate"), nil, nil
	}
}

func SimulateMsgUpdateMinInterestRate(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Get a random vault
		vaults, err := k.GetVaults(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMinInterestRateRequest{}), "unable to get vaults"), nil, err
		}
		if len(vaults) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMinInterestRateRequest{}), "no vaults found"), nil, nil
		}
		vaultAddr := vaults[r.Intn(len(vaults))]
		vault, err := k.GetVault(ctx, vaultAddr)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMinInterestRateRequest{}), "unable to get vault"), nil, err
		}
		if vault == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMinInterestRateRequest{}), "received nil vault"), nil, err
		}

		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMinInterestRateRequest{}), "invalid admin address for vault"), nil, err
		}

		rate := r.Float64()*3 - 1

		msg := &types.MsgUpdateMinInterestRateRequest{
			VaultAddress: vaultAddr.String(),
			Admin:        adminAddr.String(),
			MinRate:      fmt.Sprintf("%f", rate),
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.UpdateMinInterestRate(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully updated min interest rate"), nil, nil
	}
}

func SimulateMsgUpdateMaxInterestRate(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Get a random vault
		vaults, err := k.GetVaults(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMaxInterestRateRequest{}), "unable to get vaults"), nil, err
		}
		if len(vaults) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMaxInterestRateRequest{}), "no vaults found"), nil, nil
		}
		vaultAddr := vaults[r.Intn(len(vaults))]
		vault, err := k.GetVault(ctx, vaultAddr)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMaxInterestRateRequest{}), "unable to get vault"), nil, err
		}
		if vault == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMaxInterestRateRequest{}), "received nil vault"), nil, err
		}

		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMaxInterestRateRequest{}), "invalid admin address for vault"), nil, err
		}

		// Ensure max rate is > min rate if min rate is set
		rate := r.Float64()*3 - 1

		msg := &types.MsgUpdateMaxInterestRateRequest{
			VaultAddress: vaultAddr.String(),
			Admin:        adminAddr.String(),
			MaxRate:      fmt.Sprintf("%f", rate),
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.UpdateMaxInterestRate(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully updated max interest rate"), nil, nil
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

func SimulateMsgDepositInterestFunds(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Get a random vault
		vaults, err := k.GetVaults(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositInterestFundsRequest{}), "unable to get vaults"), nil, err
		}
		if len(vaults) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositInterestFundsRequest{}), "no vaults found"), nil, nil
		}
		vaultAddr := vaults[r.Intn(len(vaults))]
		vault, err := k.GetVault(ctx, vaultAddr)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositInterestFundsRequest{}), "unable to get vault"), nil, err
		}
		if vault == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositInterestFundsRequest{}), "received nil vault"), nil, nil
		}

		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositInterestFundsRequest{}), "invalid admin address"), nil, err
		}

		amount := sdk.NewInt64Coin(vault.UnderlyingAsset, r.Int63n(100000))

		msg := &types.MsgDepositInterestFundsRequest{
			VaultAddress: vaultAddr.String(),
			Admin:        adminAddr.String(),
			Amount:       amount,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.DepositInterestFunds(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully deposited interest funds"), nil, nil
	}
}

func SimulateMsgWithdrawInterestFunds(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Get a random vault
		vaults, err := k.GetVaults(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgWithdrawInterestFundsRequest{}), "unable to get vaults"), nil, err
		}
		if len(vaults) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgWithdrawInterestFundsRequest{}), "no vaults found"), nil, nil
		}
		vaultAddr := vaults[r.Intn(len(vaults))]
		vault, err := k.GetVault(ctx, vaultAddr)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgWithdrawInterestFundsRequest{}), "unable to get vault"), nil, err
		}
		if vault == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgWithdrawInterestFundsRequest{}), "received nil vault"), nil, nil
		}

		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgWithdrawInterestFundsRequest{}), "invalid admin address"), nil, err
		}

		balance := k.BankKeeper.GetBalance(ctx, vaultAddr, vault.UnderlyingAsset)
		amount := sdk.NewInt64Coin(vault.UnderlyingAsset, r.Int63n(balance.Amount.Int64()))

		msg := &types.MsgWithdrawInterestFundsRequest{
			VaultAddress: vaultAddr.String(),
			Admin:        adminAddr.String(),
			Amount:       amount,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.WithdrawInterestFunds(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully withdrew interest funds"), nil, nil
	}
}

func SimulateMsgDepositPrincipalFunds(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Get a random vault
		vaults, err := k.GetVaults(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositPrincipalFundsRequest{}), "unable to get vaults"), nil, err
		}
		if len(vaults) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositPrincipalFundsRequest{}), "no vaults found"), nil, nil
		}
		vaultAddr := vaults[r.Intn(len(vaults))]
		vault, err := k.GetVault(ctx, vaultAddr)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositPrincipalFundsRequest{}), "unable to get vault"), nil, err
		}
		if vault == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositPrincipalFundsRequest{}), "received nil vault"), nil, nil
		}

		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositPrincipalFundsRequest{}), "invalid admin address"), nil, err
		}

		amount := sdk.NewInt64Coin(vault.UnderlyingAsset, r.Int63n(100000))

		msg := &types.MsgDepositPrincipalFundsRequest{
			VaultAddress: vaultAddr.String(),
			Admin:        adminAddr.String(),
			Amount:       amount,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.DepositPrincipalFunds(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully deposited principal funds"), nil, nil
	}
}

func SimulateMsgWithdrawPrincipalFunds(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Get a random vault
		vaults, err := k.GetVaults(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgWithdrawPrincipalFundsRequest{}), "unable to get vaults"), nil, err
		}
		if len(vaults) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgWithdrawPrincipalFundsRequest{}), "no vaults found"), nil, nil
		}
		vaultAddr := vaults[r.Intn(len(vaults))]
		vault, err := k.GetVault(ctx, vaultAddr)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgWithdrawPrincipalFundsRequest{}), "unable to get vault"), nil, err
		}
		if vault == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgWithdrawPrincipalFundsRequest{}), "received nil vault"), nil, nil
		}

		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgWithdrawPrincipalFundsRequest{}), "invalid admin address"), nil, err
		}

		principalAddr := vault.PrincipalMarkerAddress()
		balance := k.BankKeeper.GetBalance(ctx, principalAddr, vault.UnderlyingAsset)
		amount := sdk.NewInt64Coin(vault.UnderlyingAsset, r.Int63n(balance.Amount.Int64()))

		msg := &types.MsgWithdrawPrincipalFundsRequest{
			VaultAddress: vaultAddr.String(),
			Admin:        adminAddr.String(),
			Amount:       amount,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.WithdrawPrincipalFunds(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully withdrew principal funds"), nil, nil
	}
}

func SimulateMsgExpeditePendingSwapOut(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Get a random vault
		vaults, err := k.GetVaults(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgExpeditePendingSwapOutRequest{}), "unable to get vaults"), nil, err
		}
		if len(vaults) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgExpeditePendingSwapOutRequest{}), "no vaults found"), nil, nil
		}
		vaultAddr := vaults[r.Intn(len(vaults))]
		vault, err := k.GetVault(ctx, vaultAddr)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgExpeditePendingSwapOutRequest{}), "unable to get vault"), nil, err
		}
		if vault == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgExpeditePendingSwapOutRequest{}), "received nil vault"), nil, nil
		}

		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgExpeditePendingSwapOutRequest{}), "invalid admin address"), nil, err
		}

		// TODO: get a real swap id
		swapID := uint64(1)

		msg := &types.MsgExpeditePendingSwapOutRequest{
			Admin:     adminAddr.String(),
			RequestId: swapID,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.ExpeditePendingSwapOut(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully expedited swap out"), nil, nil
	}
}

func SimulateMsgPauseVault(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Get a random vault
		vaults, err := k.GetVaults(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgPauseVaultRequest{}), "unable to get vaults"), nil, err
		}
		if len(vaults) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgPauseVaultRequest{}), "no vaults found"), nil, nil
		}
		vaultAddr := vaults[r.Intn(len(vaults))]
		vault, err := k.GetVault(ctx, vaultAddr)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgPauseVaultRequest{}), "unable to get vault"), nil, err
		}
		if vault == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgPauseVaultRequest{}), "received nil vault"), nil, nil
		}

		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgPauseVaultRequest{}), "invalid admin address"), nil, err
		}

		msg := &types.MsgPauseVaultRequest{
			VaultAddress: vaultAddr.String(),
			Admin:        adminAddr.String(),
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.PauseVault(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully paused vault"), nil, nil
	}
}

func SimulateMsgUnpauseVault(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		// Get a random vault
		vaults, err := k.GetVaults(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUnpauseVaultRequest{}), "unable to get vaults"), nil, err
		}
		if len(vaults) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUnpauseVaultRequest{}), "no vaults found"), nil, nil
		}
		vaultAddr := vaults[r.Intn(len(vaults))]
		vault, err := k.GetVault(ctx, vaultAddr)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUnpauseVaultRequest{}), "unable to get vault"), nil, err
		}
		if vault == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUnpauseVaultRequest{}), "received nil vault"), nil, nil
		}

		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUnpauseVaultRequest{}), "invalid admin address"), nil, err
		}

		msg := &types.MsgUnpauseVaultRequest{
			VaultAddress: vaultAddr.String(),
			Admin:        adminAddr.String(),
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.UnpauseVault(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully unpaused vault"), nil, nil
	}
}
