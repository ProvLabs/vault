package vault

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	modulev1 "github.com/provlabs/vault/api/provlabs/vault/module/v1"
	vaultv1 "github.com/provlabs/vault/api/provlabs/vault/v1"
	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/simulation"
	"github.com/provlabs/vault/types"

	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	"cosmossdk.io/core/address"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/event"
	"cosmossdk.io/core/header"
	"cosmossdk.io/core/store"
	"cosmossdk.io/depinject"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/version"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// ConsensusVersion defines the current x/vault module consensus version.
const ConsensusVersion = 1

var (
	_ module.AppModuleBasic      = AppModule{}
	_ appmodule.AppModule        = AppModule{}
	_ module.HasConsensusVersion = AppModule{}
	_ module.HasGenesis          = AppModule{}
	_ module.HasGenesisBasics    = AppModuleBasic{}
	_ module.HasServices         = AppModule{}
	_ module.AppModuleSimulation = AppModule{}
)

// AppModuleBasic implements the basic methods for the vault module.
type AppModuleBasic struct{}

// NewAppModuleBasic creates a new AppModuleBasic.
func NewAppModuleBasic() AppModuleBasic {
	return AppModuleBasic{}
}

// Name returns the vault module name.
func (AppModuleBasic) Name() string { return types.ModuleName }

// RegisterLegacyAminoCodec registers the legacy amino codec.
func (AppModuleBasic) RegisterLegacyAminoCodec(_ *codec.LegacyAmino) {}

// RegisterInterfaces registers vault interfaces to the interface registry.
func (AppModuleBasic) RegisterInterfaces(reg codectypes.InterfaceRegistry) {
	types.RegisterInterfaces(reg)
}

// RegisterGRPCGatewayRoutes sets up gRPC gateway routes.
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	if err := types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(clientCtx)); err != nil {
		panic(err)
	}
}

// DefaultGenesis returns default genesis state as raw bytes.
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(types.DefaultGenesisState())
}

// ValidateGenesis validates the vault genesis state.
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	var genesis types.GenesisState
	if err := cdc.UnmarshalJSON(bz, &genesis); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}
	return genesis.Validate()
}

// AppModule implements the core vault module functionality.
type AppModule struct {
	AppModuleBasic
	keeper       *keeper.Keeper
	addressCodec address.Codec
	markerKeeper types.MarkerKeeper
	bankKeeper   types.BankKeeper
}

// NewAppModule creates a new AppModule instance.
func NewAppModule(keeper *keeper.Keeper, mk types.MarkerKeeper, bk types.BankKeeper, addressCodec address.Codec) AppModule {
	return AppModule{
		AppModuleBasic: NewAppModuleBasic(),
		keeper:         keeper,
		addressCodec:   addressCodec,
		markerKeeper:   mk,
		bankKeeper:     bk,
	}
}

// IsOnePerModuleType asserts one module per type.
func (AppModule) IsOnePerModuleType() {}

// IsAppModule asserts this is an app module.
func (AppModule) IsAppModule() {}

// ConsensusVersion returns the module consensus version.
func (AppModule) ConsensusVersion() uint64 { return ConsensusVersion }

// InitGenesis initializes the module's state from genesis.
func (m AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, bz json.RawMessage) {
	var genesis types.GenesisState
	cdc.MustUnmarshalJSON(bz, &genesis)
	m.keeper.InitGenesis(ctx, &genesis)
}

// ExportGenesis exports the module's state to genesis.
func (m AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	genesis := m.keeper.ExportGenesis(ctx)
	return cdc.MustMarshalJSON(genesis)
}

// BeginBlock returns the begin blocker for the vault module.
func (m AppModule) BeginBlock(ctx context.Context) error {
	return m.keeper.BeginBlocker(ctx)
}

// EndBlock returns the end blocker for the vault module.
func (m AppModule) EndBlock(ctx context.Context) error {
	return m.keeper.EndBlocker(ctx)
}

// RegisterServices registers gRPC query and message services.
func (m AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServer(m.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQueryServer(m.keeper))
}

// AutoCLIOptions defines CLI commands for tx and query.
func (AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	txStart := fmt.Sprintf("%s tx %s", version.AppName, types.ModuleName)
	queryStart := fmt.Sprintf("%s query %s", version.AppName, types.ModuleName)

	exampleAdminAddr := "pb1g4s2q6c0a8y9c0s6e1f4h7j9k2l4m6n8p0q2r"
	exampleAuthorityAddr := "pb1m4n5g6r7m8n9g0r1a2s3s4e5t6m7a8n9a0g"
	exampleVaultAddr := "pb1z3x5c7v9b2n4m6f8h0j1k3l5p7r9s0t2w4y6"
	exampleOwnerAddr := "pb1a2b3c4d5e6f7g8h9j0k1l2m3n4p5q6r7s8t9u"
	exampleBridgeAddr := "pb1b2r3i4d5g6e7a8d9d0e1m2o3s4i5g6n7e8r9"
	exampleAssetMgrAddr := "pb1a5s6e7t8m9g0r1m2a3n4a5g6e7r8a9d0r1s"
	exampleMetadata := `{"base":"nushare","name":"Nu Vault Share","symbol":"NU","description":"Share token for the Nu Vault","display":"ushare","denom_units":[{"denom":"nushare","exponent":0},{"denom":"ushare","exponent":6}]}`

	return &autocliv1.ModuleOptions{
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service: vaultv1.Msg_ServiceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "CreateVault",
					Use:       "create [admin] [underlying_asset] [share_denom]",
					Alias:     []string{"c", "new"},
					Short:     "Create a new vault",
					Long:      "Create a new vault with an underlying asset and share denom. Optionally set a default payment denom for redemptions and a withdrawal delay that queues swap-outs until the delay elapses.",
					Example:   fmt.Sprintf("%s create %s nhash svnhash --payment-denom nhash --withdrawal-delay-seconds 86400", txStart, exampleAdminAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "underlying_asset"},
						{ProtoField: "share_denom"},
					},
					FlagOptions: []*autocliv1.FlagOption{
						{
							Name:  "payment-denom",
							Usage: "Payment denom used for swap-outs and payouts. If omitted, the underlying asset is used.",
						},
						{
							Name:  "withdrawal-delay-seconds",
							Usage: "Minimum delay (in seconds) before a queued swap-out can complete. Set to 0 for no delay.",
						},
					},
				},
				{
					RpcMethod: "SetShareDenomMetadata",
					Use:       "set-share-denom-metadata [admin] [vault_address] [metadata]",
					Alias:     []string{"ssdm"},
					Short:     "Set bank metadata for a vault’s share denom",
					Long:      "Set on-chain bank metadata for the share denom of a vault. Provide a full metadata JSON object.",
					Example:   fmt.Sprintf("%s set-share-denom-metadata %s %s '%s'", txStart, exampleAdminAddr, exampleVaultAddr, exampleMetadata),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "metadata"},
					},
				},
				{
					RpcMethod: "SwapIn",
					Use:       "swap-in [owner] [vault_address] [assets]",
					Alias:     []string{"si"},
					Short:     "Deposit underlying assets into a vault to mint shares",
					Long:      "Deposit one or more coins of the underlying asset to mint vault shares at the current conversion rate.",
					Example:   fmt.Sprintf("%s swap-in %s %s 1000nhash", txStart, exampleOwnerAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "owner"},
						{ProtoField: "vault_address"},
						{ProtoField: "assets"},
					},
				},
				{
					RpcMethod: "SwapOut",
					Use:       "swap-out [owner] [vault_address] [assets] [redeem_denom]",
					Alias:     []string{"so"},
					Short:     "Queue a withdrawal by redeeming shares for assets",
					Long:      "Redeem shares to receive assets. The request is queued and completed later according to vault policy and any configured withdrawal delay.",
					Example:   fmt.Sprintf("%s swap-out %s %s 100svnhash nhash", txStart, exampleOwnerAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "owner"},
						{ProtoField: "vault_address"},
						{ProtoField: "assets"},
						{ProtoField: "redeem_denom", Optional: true},
					},
				},
				{
					RpcMethod: "UpdateMinInterestRate",
					Use:       "update-min-interest-rate [admin] [vault_address] [min_rate]",
					Alias:     []string{"umir"},
					Short:     "Set the vault's minimum annual interest rate or clear it with empty string",
					Long:      "Set a floor for the annual interest rate. Pass an empty string to clear. Rate is a decimal (e.g. 0.01 for 1%).",
					Example:   fmt.Sprintf("%s update-min-interest-rate %s %s 0.01", txStart, exampleAdminAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "min_rate"},
					},
				},
				{
					RpcMethod: "UpdateMaxInterestRate",
					Use:       "update-max-interest-rate [admin] [vault_address] [max_rate]",
					Alias:     []string{"umaxir"},
					Short:     "Set the vault's maximum annual interest rate or clear it with empty string",
					Long:      "Set a ceiling for the annual interest rate. Pass an empty string to clear. Rate is a decimal (e.g. 0.10 for 10%).",
					Example:   fmt.Sprintf("%s update-max-interest-rate %s %s 0.10", txStart, exampleAdminAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "max_rate"},
					},
				},
				{
					RpcMethod: "UpdateInterestRate",
					Use:       "update-interest-rate [authority] [vault_address] [new_rate]",
					Alias:     []string{"uir"},
					Short:     "Update the vault's current/desired annual interest rate (admin or asset manager)",
					Long:      "Update the current or target annual interest rate for a vault. Rate is a decimal (e.g. 0.05 for 5%).",
					Example:   fmt.Sprintf("%s update-interest-rate %s %s 0.05", txStart, exampleAuthorityAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "authority"},
						{ProtoField: "vault_address"},
						{ProtoField: "new_rate"},
					},
				},
				{
					RpcMethod: "ToggleSwapIn",
					Use:       "toggle-swap-in [admin] [vault_address] [enabled]",
					Alias:     []string{"tsi"},
					Short:     "Enable or disable swap-in operations for a vault",
					Long:      "Enable or disable user deposits (swap-in). Set enabled to true or false.",
					Example:   fmt.Sprintf("%s toggle-swap-in %s %s false", txStart, exampleAdminAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "enabled"},
					},
				},
				{
					RpcMethod: "ToggleSwapOut",
					Use:       "toggle-swap-out [admin] [vault_address] [enabled]",
					Alias:     []string{"tso"},
					Short:     "Enable or disable swap-out operations for a vault",
					Long:      "Enable or disable user redemptions (swap-out). Set enabled to true or false.",
					Example:   fmt.Sprintf("%s toggle-swap-out %s %s false", txStart, exampleAdminAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "enabled"},
					},
				},
				{
					RpcMethod: "DepositInterestFunds",
					Use:       "deposit-interest-funds [authority] [vault_address] [amount]",
					Alias:     []string{"dif"},
					Short:     "Deposit funds into a vault for paying interest (admin or asset manager)",
					Long:      "Deposit coins used exclusively for interest payouts. Amount is one or more coins, e.g. 5000nhash.",
					Example:   fmt.Sprintf("%s deposit-interest-funds %s %s 5000nhash", txStart, exampleAuthorityAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "authority"},
						{ProtoField: "vault_address"},
						{ProtoField: "amount"},
					},
				},
				{
					RpcMethod: "WithdrawInterestFunds",
					Use:       "withdraw-interest-funds [authority] [vault_address] [amount]",
					Alias:     []string{"wif"},
					Short:     "Withdraw unused interest funds (admin or asset manager)",
					Long:      "Withdraw coins from the vault’s interest pool that are not allocated or required for current payouts.",
					Example:   fmt.Sprintf("%s withdraw-interest-funds %s %s 1000nhash", txStart, exampleAuthorityAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "authority"},
						{ProtoField: "vault_address"},
						{ProtoField: "amount"},
					},
				},
				{
					RpcMethod: "DepositPrincipalFunds",
					Use:       "deposit-principal-funds [authority] [vault_address] [amount]",
					Alias:     []string{"dpf"},
					Short:     "Deposit principal funds into the vault’s marker (admin or asset manager)",
					Long:      "Deposit principal coins managed by the vault. These funds back redemptions and operations per policy.",
					Example:   fmt.Sprintf("%s deposit-principal-funds %s %s 100000nhash", txStart, exampleAuthorityAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "authority"},
						{ProtoField: "vault_address"},
						{ProtoField: "amount"},
					},
				},
				{
					RpcMethod: "WithdrawPrincipalFunds",
					Use:       "withdraw-principal-funds [authority] [vault_address] [amount]",
					Alias:     []string{"wpf"},
					Short:     "Withdraw principal funds from the vault’s marker (admin or asset manager)",
					Long:      "Withdraw principal coins managed by the vault. This reduces available backing for redemptions.",
					Example:   fmt.Sprintf("%s withdraw-principal-funds %s %s 10000nhash", txStart, exampleAuthorityAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "authority"},
						{ProtoField: "vault_address"},
						{ProtoField: "amount"},
					},
				},
				{
					RpcMethod: "ExpeditePendingSwapOut",
					Use:       "expedite-pending-swap-out [authority] [request_id]",
					Alias:     []string{"epso"},
					Short:     "Expedite a pending swap out (admin or asset manager)",
					Long:      "Prioritize a pending swap-out request to be processed sooner if policy permits.",
					Example:   fmt.Sprintf("%s expedite-pending-swap-out %s 1", txStart, exampleAuthorityAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "authority"},
						{ProtoField: "request_id"},
					},
				},
				{
					RpcMethod: "PauseVault",
					Use:       "pause [authority] [vault_address] [reason]",
					Alias:     []string{"pv"},
					Short:     "Pause a vault (admin or asset manager), disabling user-facing operations",
					Long:      "Pause user-facing operations such as swap-in and swap-out. Provide a short reason string.",
					Example:   fmt.Sprintf("%s pause %s %s 'rebalancing collateral'", txStart, exampleAuthorityAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "authority"},
						{ProtoField: "vault_address"},
						{ProtoField: "reason"},
					},
				},
				{
					RpcMethod: "UnpauseVault",
					Use:       "unpause [authority] [vault_address]",
					Alias:     []string{"upv"},
					Short:     "Unpause a vault (admin or asset manager)",
					Long:      "Resume user-facing operations after a pause.",
					Example:   fmt.Sprintf("%s unpause %s %s", txStart, exampleAuthorityAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "authority"},
						{ProtoField: "vault_address"},
					},
				},
				{
					RpcMethod: "SetBridgeAddress",
					Use:       "set-bridge-address [admin] [vault_address] [bridge_address]",
					Alias:     []string{"sba"},
					Short:     "Set the single external bridge address allowed to mint/burn shares for a vault",
					Long:      "Configure the bridge address authorized to mint or burn share supply for cross-domain workflows.",
					Example:   fmt.Sprintf("%s set-bridge-address %s %s %s", txStart, exampleAdminAddr, exampleVaultAddr, exampleBridgeAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "bridge_address"},
					},
				},
				{
					RpcMethod: "ToggleBridge",
					Use:       "toggle-bridge [admin] [vault_address] [enabled]",
					Alias:     []string{"tb"},
					Short:     "Enable or disable the bridge functionality for a vault",
					Long:      "Turn the bridge capability on or off for the specified vault.",
					Example:   fmt.Sprintf("%s toggle-bridge %s %s true", txStart, exampleAdminAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "enabled"},
					},
				},
				{
					RpcMethod: "BridgeMintShares",
					Use:       "bridge-mint-shares [bridge] [vault_address] [shares]",
					Alias:     []string{"bms"},
					Short:     "Mint local share marker supply; must be signed by the configured bridge address",
					Long:      "Mint share supply on the local chain for a bridged asset. Requires the configured bridge signer.",
					Example:   fmt.Sprintf("%s bridge-mint-shares %s %s 100svnhash", txStart, exampleBridgeAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "bridge"},
						{ProtoField: "vault_address"},
						{ProtoField: "shares"},
					},
				},
				{
					RpcMethod: "BridgeBurnShares",
					Use:       "bridge-burn-shares [bridge] [vault_address] [shares]",
					Alias:     []string{"bbs"},
					Short:     "Burn local share marker supply; must be signed by the configured bridge address",
					Long:      "Burn share supply on the local chain for a bridged asset. Requires the configured bridge signer.",
					Example:   fmt.Sprintf("%s bridge-burn-shares %s %s 100svnhash", txStart, exampleBridgeAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "bridge"},
						{ProtoField: "vault_address"},
						{ProtoField: "shares"},
					},
				},
				{
					RpcMethod: "SetAssetManager",
					Use:       "set-asset-manager [admin] [vault_address] [asset_manager]",
					Alias:     []string{"sam"},
					Short:     "Set or clear the asset manager address for a vault (pass empty string to clear)",
					Long:      "Configure an asset manager with authority over specific admin actions. Pass empty string to clear.",
					Example:   fmt.Sprintf("%s set-asset-manager %s %s %s", txStart, exampleAdminAddr, exampleVaultAddr, exampleAssetMgrAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "asset_manager"},
					},
				},
			},
		},
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: vaultv1.Query_ServiceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "Vaults",
					Use:       "list",
					Alias:     []string{"l", "ls"},
					Short:     "Query all vaults",
					Long:      "Return a paginated list of all vaults on-chain.",
					Example:   fmt.Sprintf("%s list", queryStart),
				},
				{
					RpcMethod: "Vault",
					Use:       "get [id]",
					Alias:     []string{"g"},
					Short:     "Query a specific vault's configuration and state by vault address or share denom",
					Long:      "Fetch a vault by bech32 vault address or by its share denom. Returns configuration and current state.",
					Example:   fmt.Sprintf("%s get %s", queryStart, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "id"},
					},
				},
				{
					RpcMethod: "EstimateSwapIn",
					Use:       "estimate-swap-in [vault_address] [assets]",
					Alias:     []string{"esi"},
					Short:     "Estimate the number of shares received for assets",
					Long:      "Simulate a swap-in to estimate the shares minted for the provided assets at current conversion.",
					Example:   fmt.Sprintf("%s estimate-swap-in %s 1000nhash", queryStart, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "vault_address"},
						{ProtoField: "assets"},
					},
				},
				{
					RpcMethod: "EstimateSwapOut",
					Use:       "estimate-swap-out [vault_address] [shares] [redeem_denom]",
					Alias:     []string{"eso"},
					Short:     "Estimate assets received for redeeming shares",
					Long:      "Simulate a swap-out to estimate the assets returned for the provided shares. Optionally specify a redeem denom.",
					Example:   fmt.Sprintf("%s estimate-swap-out %s 1000000svnhash nhash", queryStart, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "vault_address"},
						{ProtoField: "shares"},
						{ProtoField: "redeem_denom", Optional: true},
					},
				},
				{
					RpcMethod: "PendingSwapOuts",
					Use:       "pending-swap-outs",
					Alias:     []string{"pso"},
					Short:     "Query all pending swap outs",
					Long:      "List all pending swap-out requests across all vaults.",
					Example:   fmt.Sprintf("%s pending-swap-outs", queryStart),
				},
				{
					RpcMethod: "VaultPendingSwapOuts",
					Use:       "vault-pending-swap-outs [id]",
					Alias:     []string{"vpso"},
					Short:     "Query all pending swap outs for a specific vault",
					Long:      "List pending swap-out requests for the provided vault address or share denom.",
					Example:   fmt.Sprintf("%s vault-pending-swap-outs %s", queryStart, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "id"},
					},
				},
			},
		},
	}
}

// init registers the vault module with the appmodule framework.
func init() {
	appmodule.Register(&modulev1.Module{}, appmodule.Provide(ProvideModule))
}

// ModuleInputs defines the inputs required to initialize the vault module.
type ModuleInputs struct {
	depinject.In
	Config        *modulev1.Module
	StoreService  store.KVStoreService
	HeaderService header.Service
	EventService  event.Service
	Codec         codec.Codec
	AddressCodec  address.Codec
	AuthKeeper    types.AccountKeeper
	MarkerKeeper  types.MarkerKeeper
	BankKeeper    types.BankKeeper
}

// ModuleOutputs defines the outputs of the vault module provider.
type ModuleOutputs struct {
	depinject.Out
	Keeper *keeper.Keeper
	Module appmodule.AppModule
}

// ProvideModule wires up the vault module and its keeper.
func ProvideModule(in ModuleInputs) ModuleOutputs {
	authority := authtypes.NewModuleAddress(types.GovModuleName)
	if in.Config.Authority != "" {
		authority = authtypes.NewModuleAddressOrBech32Address(in.Config.Authority)
	}

	k := keeper.NewKeeper(
		in.Codec,
		in.StoreService,
		in.EventService,
		in.AddressCodec,
		authority,
		in.AuthKeeper,
		in.MarkerKeeper,
		in.BankKeeper,
	)
	m := NewAppModule(k, in.MarkerKeeper, in.BankKeeper, in.AddressCodec)
	return ModuleOutputs{Keeper: k, Module: m}
}

// GenerateGenesisState creates a randomized GenState of the bank module.
func (m AppModule) GenerateGenesisState(simState *module.SimulationState) {
	simulation.RandomizedGenState(simState)
}

// RegisterStoreDecoder registers a decoder for supply module's types
func (m AppModule) RegisterStoreDecoder(_ simtypes.StoreDecoderRegistry) {
}

// WeightedOperations returns the all the gov module operations with their respective weights.
func (m AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	return simulation.WeightedOperations(simState, *m.keeper)
}
