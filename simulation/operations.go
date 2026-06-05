package simulation

import (
	"fmt"
	"math/rand"

	"github.com/provlabs/vault/interest"
	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	markerkeeper "github.com/provenance-io/provenance/x/marker/keeper"
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
	OpWeightMsgToggleBridge          = "op_weight_msg_toggle_bridge"
	OpWeightMsgSetBridgeAddress      = "op_weight_msg_set_bridge_address"
	OpWeightMsgBridgeMintShares      = "op_weight_msg_bridge_mint_shares"
	OpWeightMsgBridgeBurnShares      = "op_weight_msg_bridge_burn_shares"
	OpWeightMsgUpdateWithdrawalDelay = "op_weight_msg_update_withdrawal_delay"
	OpWeightMsgUpdateParams          = "op_weight_msg_update_params"
	OpWeightMsgUpdateVaultAUMFeeBips = "op_weight_msg_update_vault_aum_fee_bips"
	OpWeightMsgUpdateMinSwapInValue  = "op_weight_msg_update_min_swap_in_value"
	OpWeightMsgUpdateMinSwapOutValue = "op_weight_msg_update_min_swap_out_value"
	OpWeightMsgUpdateMaxSwapInValue  = "op_weight_msg_update_max_swap_in_value"
	OpWeightMsgUpdateMaxSwapOutValue = "op_weight_msg_update_max_swap_out_value"
)

var DefaultWeights = map[string]int{
	OpWeightMsgCreateVault:           4,
	OpWeightMsgSwapIn:                18,
	OpWeightMsgSwapOut:               8,
	OpWeightMsgUpdateInterestRate:    7,
	OpWeightMsgUpdateMinInterestRate: 2,
	OpWeightMsgUpdateMaxInterestRate: 2,
	OpWeightMsgToggleSwapIn:          6,
	OpWeightMsgToggleSwapOut:         6,
	OpWeightMsgDepositInterest:       3,
	OpWeightMsgWithdrawInterest:      3,
	OpWeightMsgDepositPrincipal:      3,
	OpWeightMsgWithdrawPrincipal:     3,
	OpWeightMsgExpediteSwap:          2,
	OpWeightMsgPauseVault:            2,
	OpWeightMsgUnpauseVault:          2,
	OpWeightMsgToggleBridge:          2,
	OpWeightMsgSetBridgeAddress:      2,
	OpWeightMsgBridgeMintShares:      6,
	OpWeightMsgBridgeBurnShares:      6,
	OpWeightMsgUpdateWithdrawalDelay: 2,
	OpWeightMsgUpdateParams:          1,
	OpWeightMsgUpdateVaultAUMFeeBips: 2,
	OpWeightMsgUpdateMinSwapInValue:  2,
	OpWeightMsgUpdateMinSwapOutValue: 2,
	OpWeightMsgUpdateMaxSwapInValue:  2,
	OpWeightMsgUpdateMaxSwapOutValue: 2,
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
		wToggleBridge          int
		wSetBridgeAddress      int
		wBridgeMintShares      int
		wBridgeBurnShares      int
		wUpdateWithdrawalDelay int
		wUpdateParams          int
		wUpdateVaultAUMFeeBips int
		wUpdateMinSwapInValue  int
		wUpdateMinSwapOutValue int
		wUpdateMaxSwapInValue  int
		wUpdateMaxSwapOutValue int
	)

	simState.AppParams.GetOrGenerate(OpWeightMsgCreateVault, &wCreateVault, simState.Rand, func(_ *rand.Rand) { wCreateVault = DefaultWeights[OpWeightMsgCreateVault] })
	simState.AppParams.GetOrGenerate(OpWeightMsgSwapIn, &wSwapIn, simState.Rand, func(_ *rand.Rand) { wSwapIn = DefaultWeights[OpWeightMsgSwapIn] })
	simState.AppParams.GetOrGenerate(OpWeightMsgSwapOut, &wSwapOut, simState.Rand, func(_ *rand.Rand) { wSwapOut = DefaultWeights[OpWeightMsgSwapOut] })
	simState.AppParams.GetOrGenerate(OpWeightMsgUpdateInterestRate, &wUpdateInterestRate, simState.Rand, func(_ *rand.Rand) { wUpdateInterestRate = DefaultWeights[OpWeightMsgUpdateInterestRate] })
	simState.AppParams.GetOrGenerate(OpWeightMsgUpdateMinInterestRate, &wUpdateMinInterestRate, simState.Rand, func(_ *rand.Rand) { wUpdateMinInterestRate = DefaultWeights[OpWeightMsgUpdateMinInterestRate] })
	simState.AppParams.GetOrGenerate(OpWeightMsgUpdateMaxInterestRate, &wUpdateMaxInterestRate, simState.Rand, func(_ *rand.Rand) { wUpdateMaxInterestRate = DefaultWeights[OpWeightMsgUpdateMaxInterestRate] })
	simState.AppParams.GetOrGenerate(OpWeightMsgToggleSwapIn, &wToggleSwapIn, simState.Rand, func(_ *rand.Rand) { wToggleSwapIn = DefaultWeights[OpWeightMsgToggleSwapIn] })
	simState.AppParams.GetOrGenerate(OpWeightMsgToggleSwapOut, &wToggleSwapOut, simState.Rand, func(_ *rand.Rand) { wToggleSwapOut = DefaultWeights[OpWeightMsgToggleSwapOut] })
	simState.AppParams.GetOrGenerate(OpWeightMsgDepositInterest, &wDepositInterest, simState.Rand, func(_ *rand.Rand) { wDepositInterest = DefaultWeights[OpWeightMsgDepositInterest] })
	simState.AppParams.GetOrGenerate(OpWeightMsgWithdrawInterest, &wWithdrawInterest, simState.Rand, func(_ *rand.Rand) { wWithdrawInterest = DefaultWeights[OpWeightMsgWithdrawInterest] })
	simState.AppParams.GetOrGenerate(OpWeightMsgDepositPrincipal, &wDepositPrincipal, simState.Rand, func(_ *rand.Rand) { wDepositPrincipal = DefaultWeights[OpWeightMsgDepositPrincipal] })
	simState.AppParams.GetOrGenerate(OpWeightMsgWithdrawPrincipal, &wWithdrawPrincipal, simState.Rand, func(_ *rand.Rand) { wWithdrawPrincipal = DefaultWeights[OpWeightMsgWithdrawPrincipal] })
	simState.AppParams.GetOrGenerate(OpWeightMsgExpediteSwap, &wExpediteSwap, simState.Rand, func(_ *rand.Rand) { wExpediteSwap = DefaultWeights[OpWeightMsgExpediteSwap] })
	simState.AppParams.GetOrGenerate(OpWeightMsgPauseVault, &wPauseVault, simState.Rand, func(_ *rand.Rand) { wPauseVault = DefaultWeights[OpWeightMsgPauseVault] })
	simState.AppParams.GetOrGenerate(OpWeightMsgUnpauseVault, &wUnpauseVault, simState.Rand, func(_ *rand.Rand) { wUnpauseVault = DefaultWeights[OpWeightMsgUnpauseVault] })
	simState.AppParams.GetOrGenerate(OpWeightMsgToggleBridge, &wToggleBridge, simState.Rand, func(_ *rand.Rand) { wToggleBridge = DefaultWeights[OpWeightMsgToggleBridge] })
	simState.AppParams.GetOrGenerate(OpWeightMsgSetBridgeAddress, &wSetBridgeAddress, simState.Rand, func(_ *rand.Rand) { wSetBridgeAddress = DefaultWeights[OpWeightMsgSetBridgeAddress] })
	simState.AppParams.GetOrGenerate(OpWeightMsgBridgeMintShares, &wBridgeMintShares, simState.Rand, func(_ *rand.Rand) { wBridgeMintShares = DefaultWeights[OpWeightMsgBridgeMintShares] })
	simState.AppParams.GetOrGenerate(OpWeightMsgBridgeBurnShares, &wBridgeBurnShares, simState.Rand, func(_ *rand.Rand) { wBridgeBurnShares = DefaultWeights[OpWeightMsgBridgeBurnShares] })
	simState.AppParams.GetOrGenerate(OpWeightMsgUpdateWithdrawalDelay, &wUpdateWithdrawalDelay, simState.Rand, func(_ *rand.Rand) { wUpdateWithdrawalDelay = DefaultWeights[OpWeightMsgUpdateWithdrawalDelay] })
	simState.AppParams.GetOrGenerate(OpWeightMsgUpdateParams, &wUpdateParams, simState.Rand, func(_ *rand.Rand) { wUpdateParams = DefaultWeights[OpWeightMsgUpdateParams] })
	simState.AppParams.GetOrGenerate(OpWeightMsgUpdateVaultAUMFeeBips, &wUpdateVaultAUMFeeBips, simState.Rand, func(_ *rand.Rand) { wUpdateVaultAUMFeeBips = DefaultWeights[OpWeightMsgUpdateVaultAUMFeeBips] })
	simState.AppParams.GetOrGenerate(OpWeightMsgUpdateMinSwapInValue, &wUpdateMinSwapInValue, simState.Rand, func(_ *rand.Rand) { wUpdateMinSwapInValue = DefaultWeights[OpWeightMsgUpdateMinSwapInValue] })
	simState.AppParams.GetOrGenerate(OpWeightMsgUpdateMinSwapOutValue, &wUpdateMinSwapOutValue, simState.Rand, func(_ *rand.Rand) { wUpdateMinSwapOutValue = DefaultWeights[OpWeightMsgUpdateMinSwapOutValue] })
	simState.AppParams.GetOrGenerate(OpWeightMsgUpdateMaxSwapInValue, &wUpdateMaxSwapInValue, simState.Rand, func(_ *rand.Rand) { wUpdateMaxSwapInValue = DefaultWeights[OpWeightMsgUpdateMaxSwapInValue] })
	simState.AppParams.GetOrGenerate(OpWeightMsgUpdateMaxSwapOutValue, &wUpdateMaxSwapOutValue, simState.Rand, func(_ *rand.Rand) { wUpdateMaxSwapOutValue = DefaultWeights[OpWeightMsgUpdateMaxSwapOutValue] })

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
		simulation.NewWeightedOperation(wToggleBridge, SimulateMsgToggleBridge(k)),
		simulation.NewWeightedOperation(wSetBridgeAddress, SimulateMsgSetBridgeAddress(k)),
		simulation.NewWeightedOperation(wBridgeMintShares, SimulateMsgBridgeMintShares(k)),
		simulation.NewWeightedOperation(wBridgeBurnShares, SimulateMsgBridgeBurnShares(k)),
		simulation.NewWeightedOperation(wUpdateWithdrawalDelay, SimulateMsgUpdateWithdrawalDelay(k)),
		simulation.NewWeightedOperation(wUpdateParams, SimulateMsgUpdateParams(k)),
		simulation.NewWeightedOperation(wUpdateVaultAUMFeeBips, SimulateMsgUpdateVaultAUMFeeBips(k)),
		simulation.NewWeightedOperation(wUpdateMinSwapInValue, SimulateMsgUpdateMinSwapInValue(k)),
		simulation.NewWeightedOperation(wUpdateMinSwapOutValue, SimulateMsgUpdateMinSwapOutValue(k)),
		simulation.NewWeightedOperation(wUpdateMaxSwapInValue, SimulateMsgUpdateMaxSwapInValue(k)),
		simulation.NewWeightedOperation(wUpdateMaxSwapOutValue, SimulateMsgUpdateMaxSwapOutValue(k)),
	}
}

func SimulateMsgCreateVault(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		admin, _ := simtypes.RandomAcc(r, accs)
		denom := fmt.Sprintf("vaulttoken%d", r.Intn(100_000))

		underlying, err := getRandomDenom(r, k, ctx, admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to get random denom for underlying"), nil, nil
		}
		payment, err := getRandomDenom(r, k, ctx, admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to get random denom for payment"), nil, nil
		}
		if payment == underlying {
			payment = ""
		}

		markerKeeper, ok := k.MarkerKeeper.(markerkeeper.Keeper)
		if !ok {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "marker keeper is not of type markerkeeper.Keeper"), nil, fmt.Errorf("marker keeper is not of type markerkeeper.Keeper")
		}
		if err = PrepareVaultMarkers(ctx, k.AuthKeeper, markerKeeper, underlying, payment, denom); err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "failed to prepare vault markers"), nil, err
		}

		msg := &types.MsgCreateVaultRequest{
			Admin:                  admin.Address.String(),
			ShareDenom:             denom,
			UnderlyingAsset:        underlying,
			PaymentDenom:           payment,
			WithdrawalDelaySeconds: interest.SecondsPerDay,
		}

		if r.Intn(2) == 0 {
			msg.MinSwapInValue = math.NewInt(int64(r.Intn(1000))).String()
		}
		if r.Intn(2) == 0 {
			msg.MinSwapOutValue = math.NewInt(int64(r.Intn(1000))).String()
		}
		if r.Intn(2) == 0 {
			minSwapIn, ok := math.NewIntFromString(msg.MinSwapInValue)
			if !ok {
				minSwapIn = math.ZeroInt()
			}
			msg.MaxSwapInValue = minSwapIn.Add(math.NewInt(int64(r.Intn(10000) + 1))).String()
		}
		if r.Intn(2) == 0 {
			minSwapOut, ok := math.NewIntFromString(msg.MinSwapOutValue)
			if !ok {
				minSwapOut = math.ZeroInt()
			}
			msg.MaxSwapOutValue = minSwapOut.Add(math.NewInt(int64(r.Intn(10000) + 1))).String()
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.CreateVault(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, ""), nil, nil
	}
}

func SimulateMsgSwapIn(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgSwapInRequest{}), "unable to get random vault"), nil, err
		}

		assetDenom := getRandomVaultAsset(r, vault)
		owner, balance, err := getRandomAccountWithDenom(r, k, ctx, accs, assetDenom)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgSwapInRequest{}), "no account has funds for this vault's accepted assets"), nil, nil
		}

		// Calculate 1/1000 of the balance
		portion := balance.Amount.Quo(math.NewInt(1_000))
		if portion.IsZero() {
			// If portion is zero, the balance is too low to deposit a meaningful portion.
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgSwapInRequest{}), "balance too low to swap in a portion"), nil, nil
		}

		// Pick a random amount of their balance up to portion
		amount, err := simtypes.RandPositiveInt(r, portion)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgSwapInRequest{}), "balance amount is not positive"), nil, nil
		}
		asset := sdk.NewCoin(assetDenom, amount)

		// Create and dispatch the message
		msg := &types.MsgSwapInRequest{
			Owner:        owner.Address.String(),
			VaultAddress: vault.GetAddress().String(),
			Assets:       asset,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.SwapIn(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "failed to swap in"), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully swapped in"), nil, nil
	}
}

func SimulateMsgSwapOut(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgSwapOutRequest{}), "unable to get random vault"), nil, err
		}

		// Find an account that has shares in this vault
		owner, balance, err := getRandomAccountWithDenom(r, k, ctx, accs, vault.TotalShares.Denom)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgSwapOutRequest{}), "no account has shares in this vault"), nil, nil
		}

		// Pick a random amount of shares to swap out
		amount, err := simtypes.RandPositiveInt(r, balance.Amount)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgSwapOutRequest{}), "balance amount is not positive"), nil, nil
		}
		shares := sdk.NewCoin(vault.TotalShares.Denom, amount)

		// Pick a random asset to receive it in
		redeemDenom := getRandomVaultAsset(r, vault)

		// Create and dispatch the message
		msg := &types.MsgSwapOutRequest{
			Owner:        owner.Address.String(),
			VaultAddress: vault.GetAddress().String(),
			Assets:       shares,
			RedeemDenom:  redeemDenom,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.SwapOut(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "failed to swap out"), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully swapped out"), nil, nil
	}
}

func SimulateMsgUpdateInterestRate(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateInterestRateRequest{}), "unable to get random vault"), nil, err
		}

		authority, err := getRandomManagementAuthority(r, ctx, vault, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateInterestRateRequest{}), "unable to get random authority"), nil, nil
		}

		rate, err := getRandomInterestRate(r, k, ctx, vault.GetAddress())
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateInterestRateRequest{}), "unable to get random interest rate"), nil, err
		}

		msg := &types.MsgUpdateInterestRateRequest{
			VaultAddress: vault.GetAddress().String(),
			Authority:    authority.Address.String(),
			NewRate:      rate,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.UpdateInterestRate(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully updated interest rate"), nil, nil
	}
}

func SimulateMsgUpdateMinInterestRate(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMinInterestRateRequest{}), "unable to get random vault"), nil, err
		}

		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMinInterestRateRequest{}), "invalid admin address for vault"), nil, err
		}

		rate, err := getRandomMinInterestRate(r, k, ctx, vault.GetAddress())
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMinInterestRateRequest{}), "unable to get random min interest rate"), nil, err
		}

		msg := &types.MsgUpdateMinInterestRateRequest{
			VaultAddress: vault.GetAddress().String(),
			Admin:        adminAddr.String(),
			MinRate:      rate,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.UpdateMinInterestRate(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully updated min interest rate"), nil, nil
	}
}

func SimulateMsgUpdateMaxInterestRate(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMaxInterestRateRequest{}), "unable to get random vault"), nil, err
		}

		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMaxInterestRateRequest{}), "invalid admin address for vault"), nil, err
		}

		rate, err := getRandomMaxInterestRate(r, k, ctx, vault.GetAddress())
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMaxInterestRateRequest{}), "unable to get random max interest rate"), nil, err
		}

		msg := &types.MsgUpdateMaxInterestRateRequest{
			VaultAddress: vault.GetAddress().String(),
			Admin:        adminAddr.String(),
			MaxRate:      rate,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.UpdateMaxInterestRate(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully updated max interest rate"), nil, nil
	}
}

func SimulateMsgToggleSwapIn(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgToggleSwapInRequest{}), "unable to get random vault"), nil, err
		}

		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgToggleSwapInRequest{}), "invalid admin address"), nil, err
		}

		msg := &types.MsgToggleSwapInRequest{
			VaultAddress: vault.GetAddress().String(),
			Admin:        adminAddr.String(),
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.ToggleSwapIn(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully toggled swap in"), nil, nil
	}
}

func SimulateMsgToggleSwapOut(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgToggleSwapOutRequest{}), "unable to get random vault"), nil, err
		}

		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgToggleSwapOutRequest{}), "invalid admin address"), nil, err
		}

		msg := &types.MsgToggleSwapOutRequest{
			VaultAddress: vault.GetAddress().String(),
			Admin:        adminAddr.String(),
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.ToggleSwapOut(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully toggled swap out"), nil, nil
	}
}

func SimulateMsgDepositInterestFunds(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositInterestFundsRequest{}), "unable to get random vault"), nil, err
		}

		authority, err := getRandomManagementAuthority(r, ctx, vault, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositInterestFundsRequest{}), "unable to get random authority"), nil, nil
		}

		// Find the authority's balance of the underlying asset
		balance := k.BankKeeper.GetBalance(ctx, authority.Address, vault.UnderlyingAsset)
		if balance.IsZero() {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositInterestFundsRequest{}), "authority has no funds to deposit"), nil, nil
		}

		// Calculate 1% of the balance
		portion := balance.Amount.Quo(math.NewInt(100))
		if portion.IsZero() {
			// If 1% is zero, the balance is too low to deposit a meaningful portion.
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositInterestFundsRequest{}), "balance too low to deposit a portion"), nil, nil
		}

		// Deposit a random amount up to 1% of the authority's balance
		amountInt, err := simtypes.RandPositiveInt(r, portion)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositInterestFundsRequest{}), "error generating random amount"), nil, err
		}
		amount := sdk.NewCoin(vault.UnderlyingAsset, amountInt)

		msg := &types.MsgDepositInterestFundsRequest{
			VaultAddress: vault.GetAddress().String(),
			Authority:    authority.Address.String(),
			Amount:       amount,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.DepositInterestFunds(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully deposited interest funds"), nil, nil
	}
}

func SimulateMsgWithdrawInterestFunds(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgWithdrawInterestFundsRequest{}), "unable to get random vault"), nil, err
		}

		authority, err := getRandomManagementAuthority(r, ctx, vault, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgWithdrawInterestFundsRequest{}), "unable to get random authority"), nil, nil
		}

		balance := k.BankKeeper.GetBalance(ctx, vault.GetAddress(), vault.UnderlyingAsset)
		if balance.IsZero() {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgWithdrawInterestFundsRequest{}), "no underlying asset funds"), nil, nil
		}
		amount := sdk.NewInt64Coin(vault.UnderlyingAsset, r.Int63n(balance.Amount.Int64()))

		msg := &types.MsgWithdrawInterestFundsRequest{
			VaultAddress: vault.GetAddress().String(),
			Authority:    authority.Address.String(),
			Amount:       amount,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.WithdrawInterestFunds(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully withdrew interest funds"), nil, nil
	}
}

func SimulateMsgDepositPrincipalFunds(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositPrincipalFundsRequest{}), "unable to get random vault"), nil, err
		}

		authority, err := getRandomManagementAuthority(r, ctx, vault, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositPrincipalFundsRequest{}), "unable to get random authority"), nil, nil
		}

		asset := getRandomVaultAsset(r, vault)
		balance := k.BankKeeper.GetBalance(ctx, authority.Address, asset)
		if balance.IsZero() {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositPrincipalFundsRequest{}), "authority has no funds to deposit"), nil, nil
		}

		// Calculate 1% of the balance
		portion := balance.Amount.Quo(math.NewInt(100))
		if portion.IsZero() {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositPrincipalFundsRequest{}), "balance too low to deposit a portion"), nil, nil
		}

		// Deposit a random amount up to 1% of the authority's balance
		amountInt, err := simtypes.RandPositiveInt(r, portion)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDepositPrincipalFundsRequest{}), "error generating random amount"), nil, err
		}
		amount := sdk.NewCoin(asset, amountInt)

		msg := &types.MsgDepositPrincipalFundsRequest{
			VaultAddress: vault.GetAddress().String(),
			Authority:    authority.Address.String(),
			Amount:       amount,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.DepositPrincipalFunds(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully deposited principal funds"), nil, nil
	}
}

func SimulateMsgWithdrawPrincipalFunds(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgWithdrawPrincipalFundsRequest{}), "unable to get random vault"), nil, err
		}

		authority, err := getRandomManagementAuthority(r, ctx, vault, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgWithdrawPrincipalFundsRequest{}), "unable to get random authority"), nil, nil
		}

		principalAddr := vault.PrincipalMarkerAddress()
		asset := getRandomVaultAsset(r, vault)
		balance := k.BankKeeper.GetBalance(ctx, principalAddr, asset)
		if balance.IsZero() {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgWithdrawPrincipalFundsRequest{}), "no underlying asset funds"), nil, nil
		}
		amount := sdk.NewInt64Coin(asset, r.Int63n(balance.Amount.Int64()))

		msg := &types.MsgWithdrawPrincipalFundsRequest{
			VaultAddress: vault.GetAddress().String(),
			Authority:    authority.Address.String(),
			Amount:       amount,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.WithdrawPrincipalFunds(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully withdrew principal funds"), nil, nil
	}
}

func SimulateMsgExpeditePendingSwapOut(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgExpeditePendingSwapOutRequest{}), "unable to get random vault"), nil, err
		}

		authority, err := getRandomManagementAuthority(r, ctx, vault, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgExpeditePendingSwapOutRequest{}), "unable to get random authority"), nil, nil
		}

		swapID, err := getRandomPendingSwapOut(r, k, ctx, vault.GetAddress())
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgExpeditePendingSwapOutRequest{}), "unable to get random pending swap out"), nil, nil
		}

		msg := &types.MsgExpeditePendingSwapOutRequest{
			Authority: authority.Address.String(),
			RequestId: swapID,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.ExpeditePendingSwapOut(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully expedited swap out"), nil, nil
	}
}

func SimulateMsgPauseVault(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgPauseVaultRequest{}), "unable to get random vault"), nil, err
		}

		authority, err := getRandomManagementAuthority(r, ctx, vault, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgPauseVaultRequest{}), "unable to get random authority"), nil, nil
		}

		msg := &types.MsgPauseVaultRequest{
			VaultAddress: vault.GetAddress().String(),
			Authority:    authority.Address.String(),
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.PauseVault(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully paused vault"), nil, nil
	}
}

func SimulateMsgUnpauseVault(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUnpauseVaultRequest{}), "unable to get random vault"), nil, err
		}

		authority, err := getRandomManagementAuthority(r, ctx, vault, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUnpauseVaultRequest{}), "unable to get random authority"), nil, nil
		}

		msg := &types.MsgUnpauseVaultRequest{
			VaultAddress: vault.GetAddress().String(),
			Authority:    authority.Address.String(),
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.UnpauseVault(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully unpaused vault"), nil, nil
	}
}

func SimulateMsgToggleBridge(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgToggleBridgeRequest{}), "unable to get random vault"), nil, err
		}

		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgToggleBridgeRequest{}), "invalid admin address"), nil, err
		}

		msg := &types.MsgToggleBridgeRequest{
			VaultAddress: vault.GetAddress().String(),
			Admin:        adminAddr.String(),
			Enabled:      !vault.BridgeEnabled,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.ToggleBridge(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully toggled bridge"), nil, nil
	}
}

// SimulateMsgSetBridgeAddress creates a message to set the bridge address for a vault
func SimulateMsgSetBridgeAddress(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgSetBridgeAddressRequest{}), "unable to get random vault"), nil, err
		}

		adminAddr, err := sdk.AccAddressFromBech32(vault.Admin)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgSetBridgeAddressRequest{}), "invalid admin address"), nil, err
		}

		newBridgeAccount, _ := simtypes.RandomAcc(r, accs)

		msg := &types.MsgSetBridgeAddressRequest{
			VaultAddress:  vault.GetAddress().String(),
			Admin:         adminAddr.String(),
			BridgeAddress: newBridgeAccount.Address.String(),
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.SetBridgeAddress(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully set bridge address"), nil, nil
	}
}

// SimulateMsgBridgeMintShares creates a message to mint shares to the bridge address
func SimulateMsgBridgeMintShares(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomBridgedVault(r, k, ctx, accs, false)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgBridgeMintSharesRequest{}), err.Error()), nil, nil
		}
		bridgeAddr, _ := sdk.AccAddressFromBech32(vault.BridgeAddress)

		// Calculate available capacity for minting
		supply := k.BankKeeper.GetSupply(ctx, vault.TotalShares.Denom)
		availableToMint := vault.TotalShares.Amount.Sub(supply.Amount)

		if !availableToMint.IsPositive() {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgBridgeMintSharesRequest{}), "no capacity available to mint new shares"), nil, nil
		}

		amount, err := simtypes.RandPositiveInt(r, availableToMint)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgBridgeMintSharesRequest{}), "unable to get random mint amount"), nil, err
		}
		shares := sdk.NewCoin(vault.TotalShares.Denom, amount)
		msg := &types.MsgBridgeMintSharesRequest{
			VaultAddress: vault.GetAddress().String(),
			Bridge:       bridgeAddr.String(),
			Shares:       shares,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.BridgeMintShares(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully minted shares to bridge"), nil, nil
	}
}

// SimulateMsgBridgeBurnShares creates a message to burn shares from the bridge address
func SimulateMsgBridgeBurnShares(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomBridgedVault(r, k, ctx, accs, true)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgBridgeBurnSharesRequest{}), err.Error()), nil, nil
		}

		bridgeAddr, _ := sdk.AccAddressFromBech32(vault.BridgeAddress)
		balance := k.BankKeeper.GetBalance(ctx, bridgeAddr, vault.TotalShares.Denom)
		amount, err := simtypes.RandPositiveInt(r, balance.Amount)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgBridgeBurnSharesRequest{}), "unable to get random burn amount"), nil, err
		}
		shares := sdk.NewCoin(vault.TotalShares.Denom, amount)

		msg := &types.MsgBridgeBurnSharesRequest{
			VaultAddress: vault.GetAddress().String(),
			Bridge:       bridgeAddr.String(),
			Shares:       shares,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.BridgeBurnShares(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully burned shares from bridge"), nil, nil
	}
}

func SimulateMsgUpdateWithdrawalDelay(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateWithdrawalDelayRequest{}), "unable to get random vault"), nil, err
		}

		authority, err := getRandomManagementAuthority(r, ctx, vault, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateWithdrawalDelayRequest{}), "unable to get random authority"), nil, nil
		}

		var delay uint64
		switch r.Intn(4) {
		case 0:
			delay = 0
		case 1:
			delay = interest.SecondsPerDay
		case 2:
			delay = interest.SecondsPerDay * 7
		default:
			delay = uint64(r.Intn(int(interest.SecondsPerDay*30))) + 1
		}

		msg := &types.MsgUpdateWithdrawalDelayRequest{
			Authority:              authority.Address.String(),
			VaultAddress:           vault.GetAddress().String(),
			WithdrawalDelaySeconds: delay,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.UpdateWithdrawalDelay(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully updated withdrawal delay"), nil, nil
	}
}
func SimulateMsgUpdateParams(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		techFeeAddr := accs[r.Intn(len(accs))].Address
		defaultBips := uint32(r.Intn(1001))

		authority, err := k.AddressCodec.BytesToString(k.GetAuthority())
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateParamsRequest{}), "unable to get authority address"), nil, err
		}

		msg := &types.MsgUpdateParamsRequest{
			Authority: authority,
			Params: types.Params{
				TechFeeAddress:    techFeeAddr.String(),
				DefaultAumFeeBips: defaultBips,
			},
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.UpdateParams(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully updated params"), nil, nil
	}
}

func SimulateMsgUpdateVaultAUMFeeBips(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateVaultAUMFeeBipsRequest{}), "unable to get random vault"), nil, err
		}

		params, err := k.Params.Get(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateVaultAUMFeeBipsRequest{}), "unable to get params"), nil, err
		}

		authority, err := sdk.AccAddressFromBech32(params.TechFeeAddress)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateVaultAUMFeeBipsRequest{}), "invalid tech fee address"), nil, err
		}

		msg := &types.MsgUpdateVaultAUMFeeBipsRequest{
			Authority:    authority.String(),
			VaultAddress: vault.GetAddress().String(),
			AumFeeBips:   uint32(r.Intn(1001)),
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.UpdateVaultAUMFeeBips(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully updated vault aum fee bips"), nil, nil
	}
}

func SimulateMsgUpdateMinSwapInValue(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMinSwapInValueRequest{}), "unable to get random vault"), nil, nil
		}

		authority, err := getRandomManagementAuthority(r, ctx, vault, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMinSwapInValueRequest{}), "unable to get random authority"), nil, nil
		}

		val, err := getRandomMinSwapValue(r, k, ctx, vault.GetAddress(), true)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMinSwapInValueRequest{}), "unable to get random min swap in"), nil, nil
		}

		msg := &types.MsgUpdateMinSwapInValueRequest{
			Authority:      authority.Address.String(),
			VaultAddress:   vault.GetAddress().String(),
			MinSwapInValue: val,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.UpdateMinSwapInValue(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully updated min swap in"), nil, nil
	}
}

func SimulateMsgUpdateMinSwapOutValue(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMinSwapOutValueRequest{}), "unable to get random vault"), nil, nil
		}

		authority, err := getRandomManagementAuthority(r, ctx, vault, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMinSwapOutValueRequest{}), "unable to get random authority"), nil, nil
		}

		val, err := getRandomMinSwapValue(r, k, ctx, vault.GetAddress(), false)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMinSwapOutValueRequest{}), "unable to get random min swap out"), nil, nil
		}

		msg := &types.MsgUpdateMinSwapOutValueRequest{
			Authority:       authority.Address.String(),
			VaultAddress:    vault.GetAddress().String(),
			MinSwapOutValue: val,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.UpdateMinSwapOutValue(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully updated min swap out"), nil, nil
	}
}

func SimulateMsgUpdateMaxSwapInValue(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMaxSwapInValueRequest{}), "unable to get random vault"), nil, nil
		}

		authority, err := getRandomManagementAuthority(r, ctx, vault, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMaxSwapInValueRequest{}), "unable to get random authority"), nil, nil
		}

		val, err := getRandomMaxSwapValue(r, k, ctx, vault.GetAddress(), true)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMaxSwapInValueRequest{}), "unable to get random max swap in"), nil, nil
		}

		msg := &types.MsgUpdateMaxSwapInValueRequest{
			Authority:      authority.Address.String(),
			VaultAddress:   vault.GetAddress().String(),
			MaxSwapInValue: val,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.UpdateMaxSwapInValue(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully updated max swap in"), nil, nil
	}
}

func SimulateMsgUpdateMaxSwapOutValue(k keeper.Keeper) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		err := Setup(ctx, r, k, k.AuthKeeper, k.BankKeeper, k.MarkerKeeper, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCreateVaultRequest{}), "unable to setup initial state"), nil, err
		}

		vault, err := getRandomVault(r, k, ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMaxSwapOutValueRequest{}), "unable to get random vault"), nil, nil
		}

		authority, err := getRandomManagementAuthority(r, ctx, vault, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMaxSwapOutValueRequest{}), "unable to get random authority"), nil, nil
		}

		val, err := getRandomMaxSwapValue(r, k, ctx, vault.GetAddress(), false)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgUpdateMaxSwapOutValueRequest{}), "unable to get random max swap out"), nil, nil
		}

		msg := &types.MsgUpdateMaxSwapOutValueRequest{
			Authority:       authority.Address.String(),
			VaultAddress:    vault.GetAddress().String(),
			MaxSwapOutValue: val,
		}

		handler := keeper.NewMsgServer(&k)
		_, err = handler.UpdateMaxSwapOutValue(ctx, msg)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
		}

		return simtypes.NewOperationMsg(msg, true, "successfully updated max swap out"), nil, nil
	}
}
