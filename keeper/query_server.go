package keeper

import (
	"context"
	"fmt"
	"time"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
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
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	vaults := []types.VaultAccount{}

	_, pageRes, err := query.CollectionFilteredPaginate(
		ctx,
		k.Keeper.Vaults,
		req.Pagination,
		func(key sdk.AccAddress, _ []byte) (include bool, err error) {
			vault, _ := k.GetVault(ctx, key)
			vaults = append(vaults, *vault)
			return true, nil
		},
		func(_ sdk.AccAddress, value []byte) ([]byte, error) {
			return value, nil
		},
	)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryVaultsResponse{
		Vaults:     vaults,
		Pagination: pageRes,
	}, nil
}

// Vault returns the configuration and state of a specific vault.
func (k queryServer) Vault(goCtx context.Context, req *types.QueryVaultRequest) (*types.QueryVaultResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id must be provided")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	vault, err := k.FindVaultAccount(ctx, req.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	marker, err := k.MarkerKeeper.GetMarkerByDenom(ctx, vault.TotalShares.Denom)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	principal := k.BankKeeper.GetAllBalances(goCtx, marker.GetAddress())
	reserves := k.BankKeeper.GetAllBalances(goCtx, vault.GetAddress())

	return &types.QueryVaultResponse{
		Vault: *vault,
		Principal: types.AccountBalance{
			Address: marker.GetAddress().String(),
			Coins:   principal,
		},
		Reserves: types.AccountBalance{
			Address: vault.GetAddress().String(),
			Coins:   reserves,
		},
	}, nil
}

func (k queryServer) EstimateSwapIn(goCtx context.Context, req *types.QueryEstimateSwapInRequest) (*types.QueryEstimateSwapInResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.VaultAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "vault_address must be provided")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr, err := sdk.AccAddressFromBech32(req.VaultAddress)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid vault_address: %v", err)
	}

	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil || vault == nil {
		return nil, status.Errorf(codes.NotFound, "vault with address %q not found", req.VaultAddress)
	}

	if !vault.IsAcceptedDenom(req.Assets.Denom) {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported deposit denom: %q", req.Assets.Denom)
	}

	priceNum, priceDen, err := k.UnitPriceFraction(ctx, req.Assets.Denom, *vault)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "no NAV for %s/%s: %v", req.Assets.Denom, vault.UnderlyingAsset, err)
	}
	if priceDen.IsZero() {
		return nil, status.Errorf(codes.InvalidArgument, "invalid NAV: zero volume for %s/%s", req.Assets.Denom, vault.UnderlyingAsset)
	}

	principalAddress := vault.PrincipalMarkerAddress()
	totalShares := vault.TotalShares.Amount
	totalAssets := k.BankKeeper.GetBalance(ctx, principalAddress, vault.UnderlyingAsset)

	estimatedTotalAssets, err := k.CalculateVaultTotalAssets(ctx, vault, totalAssets)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to estimate total assets: %v", err)
	}

	amountNum := req.Assets.Amount.Mul(priceNum)
	estimatedShares, err := utils.CalculateSharesProRataFraction(
		amountNum,
		priceDen,
		estimatedTotalAssets,
		totalShares,
		vault.TotalShares.Denom,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate shares: %w", err)
	}

	return &types.QueryEstimateSwapInResponse{
		Assets: estimatedShares,
		Height: ctx.BlockHeight(),
		Time:   ctx.BlockTime().UTC(),
	}, nil
}

// EstimateSwapOut estimates the amount of payout assets (underlying or payout denom) received for a given amount of shares.
func (k queryServer) EstimateSwapOut(goCtx context.Context, req *types.QueryEstimateSwapOutRequest) (*types.QueryEstimateSwapOutResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.VaultAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "vault_address must be provided")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr, err := sdk.AccAddressFromBech32(req.VaultAddress)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid vault_address: %v", err)
	}

	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil || vault == nil {
		return nil, status.Errorf(codes.NotFound, "vault with address %q not found", req.VaultAddress)
	}
	redeemDenom := req.RedeemDenom
	if redeemDenom == "" {
		redeemDenom = vault.UnderlyingAsset
	}
	if !vault.IsAcceptedDenom(redeemDenom) {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported redeem denom: %q", redeemDenom)
	}

	principalAddress := vault.PrincipalMarkerAddress()
	totalShares := vault.TotalShares.Amount
	totalAssets := k.BankKeeper.GetBalance(ctx, principalAddress, vault.UnderlyingAsset)

	estimatedTotalAssets, err := k.CalculateVaultTotalAssets(ctx, vault, totalAssets)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to estimate total assets: %v", err)
	}

	priceNum, priceDen, err := k.UnitPriceFraction(ctx, redeemDenom, *vault)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "no NAV for %s/%s: %v", redeemDenom, vault.UnderlyingAsset, err)
	}
	if priceNum.IsZero() {
		return nil, status.Errorf(codes.InvalidArgument, "zero price for %s/%s", redeemDenom, vault.UnderlyingAsset)
	}

	estimatedPayout, err := utils.CalculateRedeemProRataFraction(
		req.Shares,
		totalShares,
		estimatedTotalAssets,
		priceNum,
		priceDen,
		redeemDenom,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate redeem estimate: %w", err)
	}

	return &types.QueryEstimateSwapOutResponse{
		Assets: estimatedPayout,
		Height: ctx.BlockHeight(),
		Time:   ctx.BlockTime().UTC(),
	}, nil
}

// PendingSwapOuts returns a paginated list of all pending swap outs.
func (k queryServer) PendingSwapOuts(goCtx context.Context, req *types.QueryPendingSwapOutsRequest) (*types.QueryPendingSwapOutsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	swapOuts := []types.PendingSwapOutWithTimeout{}

	_, pageRes, err := query.CollectionPaginate(
		ctx,
		k.PendingSwapOutQueue.IndexedMap,
		req.Pagination,
		func(key collections.Triple[int64, uint64, sdk.AccAddress], value types.PendingSwapOut) (include bool, err error) {
			swapOuts = append(swapOuts, types.PendingSwapOutWithTimeout{
				RequestId:      key.K2(),
				Timeout:        time.Unix(key.K1(), 0),
				PendingSwapOut: value,
			})
			return true, nil
		},
	)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryPendingSwapOutsResponse{
		PendingSwapOuts: swapOuts,
		Pagination:      pageRes,
	}, nil
}

// VaultPendingSwapOuts returns a paginated list of all pending swap outs for a specific vault.
func (k queryServer) VaultPendingSwapOuts(goCtx context.Context, req *types.QueryVaultPendingSwapOutsRequest) (*types.QueryVaultPendingSwapOutsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	vault, err := k.FindVaultAccount(ctx, req.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	swapOuts, pageRes, err := query.CollectionFilteredPaginate(
		ctx,
		k.PendingSwapOutQueue.IndexedMap,
		req.Pagination,
		func(key collections.Triple[int64, uint64, sdk.AccAddress], value types.PendingSwapOut) (include bool, err error) {
			return vault.Address == key.K3().String(), nil
		},
		func(key collections.Triple[int64, uint64, sdk.AccAddress], value types.PendingSwapOut) (types.PendingSwapOutWithTimeout, error) {
			return types.PendingSwapOutWithTimeout{
				RequestId:      key.K2(),
				Timeout:        time.Unix(key.K1(), 0),
				PendingSwapOut: value,
			}, nil
		},
	)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryVaultPendingSwapOutsResponse{
		PendingSwapOuts: swapOuts,
		Pagination:      pageRes,
	}, nil
}
