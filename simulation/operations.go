package simulation

import (
	"fmt"
	"math/rand"
	"slices"

	"cosmossdk.io/math"
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

type VaultAddrSelector func(ctx sdk.Context, k keeper.Keeper, underlying string, r *rand.Rand) string
type UnderlyingAssetSelector func(r *rand.Rand, ctx sdk.Context, k keeper.Keeper, acc sdk.AccAddress) string
type UnderlyingAmountSelector func(r *rand.Rand, ctx sdk.Context, k keeper.Keeper, acc sdk.AccAddress, denom string) sdk.Coin

func DefaultVaultAddrSelector(ctx sdk.Context, k keeper.Keeper, underlying string, r *rand.Rand) string {
	choice := r.Intn(3)
	switch choice {
	case 0:
		// Random address
		return utils.TestAddress().Bech32
	case 1:
		// Random vault
		vaults, _ := k.GetVaults(ctx)
		return vaults[r.Intn(len(vaults))].String()
	}

	// Vault with matching underlying asset
	vaults, _ := k.GetVaults(ctx)
	iter := utils.Filter(vaults, func(addr sdk.AccAddress) bool {
		vault, _ := k.GetVault(ctx, addr)
		return vault.UnderlyingAssets[0] == underlying
	})
	vaults = slices.Collect(iter)

	if len(vaults) == 0 {
		// No vault exists having that underlying asset
		return "invalid-vault-address"
	}

	return vaults[0].String()
}

func DefaultUnderlyingAssetSelector(r *rand.Rand, ctx sdk.Context, k keeper.Keeper, acc sdk.AccAddress) string {
	choice := r.Intn(3)
	switch choice {
	case 0:
		// Random underlying asset from one of the vaults
		vaultAddrs, _ := k.GetVaults(ctx)
		randomVaultAddr := vaultAddrs[r.Intn(len(vaultAddrs))]
		vault, _ := k.GetVault(ctx, randomVaultAddr)
		return vault.UnderlyingAssets[0]
	case 1:
		// Random underlying asset belonging to the account
		balances := k.BankKeeper.GetAllBalances(ctx, acc)
		if !balances.Empty() {
			return balances[r.Intn(len(balances))].Denom
		}
	}
	// Random bad denom
	return "bad-denom!"
}

func DefaultUnderlyingAmountSelector(r *rand.Rand, ctx sdk.Context, k keeper.Keeper, acc sdk.AccAddress, denom string) sdk.Coin {
	balance := k.BankKeeper.GetBalance(ctx, acc, denom)

	// Denom doesn't exist so just make up amount
	if balance.IsZero() {
		amount := r.Int63n(100000)
		return sdk.Coin{
			Denom:  denom,
			Amount: math.NewInt(amount),
		}
	}

	// Amount exists so 50% for it to be valid amount
	maxAmount := balance.Amount.MulRaw(2).Int64()
	amount := rand.Int63n(maxAmount)
	return sdk.Coin{
		Denom:  denom,
		Amount: math.NewInt(amount),
	}
}

func SimulateMsgSwapIn(k keeper.Keeper, vaultSelector VaultAddrSelector, assetSelector UnderlyingAssetSelector, amountSelector UnderlyingAmountSelector) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		admin, _ := simtypes.RandomAcc(r, accs)

		underlyingAsset := assetSelector(r, ctx, k, admin.Address)
		vaultAddr := vaultSelector(ctx, k, underlyingAsset, r)
		assets := amountSelector(r, ctx, k, admin.Address, underlyingAsset)

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
