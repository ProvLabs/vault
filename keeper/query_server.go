package keeper

import (
	"context"

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
	panic("not implemented")
}

// Vault returns the configuration and state of a specific vault.
func (k queryServer) Vault(goCtx context.Context, req *types.QueryVaultRequest) (*types.QueryVaultResponse, error) {
	panic("not implemented")
}

// TotalAssets returns the total amount of the underlying asset managed by the vault.
func (k queryServer) TotalAssets(goCtx context.Context, req *types.QueryTotalAssetsRequest) (*types.QueryTotalAssetsResponse, error) {
	panic("not implemented")
}

// Params returns the params for the module.
func (q queryServer) Params(ctx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	panic("not implemented")
}
