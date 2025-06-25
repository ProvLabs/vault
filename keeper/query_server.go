package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/types"
)

var _ types.QueryServer = &queryServer{}

type queryServer struct {
	*Keeper
}

func NewQueryServer(keeper *Keeper) types.QueryServer {
	return &queryServer{Keeper: keeper}
}

// Vaults returns a paginated list of all vaults.
func (k queryServer) Vaults(goCtx context.Context, req *types.QueryVaultsRequest) (*types.QueryVaultsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultMap, err := k.Keeper.GetVaults(ctx)
	if err != nil {
		return nil, err
	}

	vaults := make([]types.Vault, 0, len(vaultMap))
	for _, v := range vaultMap {
		vaults = append(vaults, v)
	}

	// TODO add pagination
	return &types.QueryVaultsResponse{
		Vaults: vaults,
	}, nil
}

// Vault returns the configuration and state of a specific vault.
func (k queryServer) Vault(goCtx context.Context, req *types.QueryVaultRequest) (*types.QueryVaultResponse, error) {
	// TODO allow queries by share denom
	if req == nil || req.VaultAddress == "" {
		return nil, fmt.Errorf("vault_address must be provided")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr, err := sdk.AccAddressFromBech32(req.VaultAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid vault_address: %w", err)
	}

	vault, err := k.Keeper.Vaults.Get(ctx, vaultAddr)
	if err != nil {
		return nil, fmt.Errorf("vault with address %q not found", req.VaultAddress)
	}

	return &types.QueryVaultResponse{
		Vault: vault,
	}, nil
}

// TotalAssets returns the total amount of the underlying asset managed by the vault.
func (k queryServer) TotalAssets(goCtx context.Context, req *types.QueryTotalAssetsRequest) (*types.QueryTotalAssetsResponse, error) {
	panic("not implemented")
}

// Params returns the params for the module.
func (q queryServer) Params(ctx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	panic("not implemented")
}
