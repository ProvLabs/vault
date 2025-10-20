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

	reg.RegisterInterface(
		"provlabs.vault.v1.VaultAccount",
		(*types.VaultAccountI)(nil),
		&types.VaultAccount{},
	)
	reg.RegisterInterface(
		"provlabs.vault.v1.VaultAccount",
		(*sdk.AccountI)(nil),
		&types.VaultAccount{},
	)
	reg.RegisterInterface(
		"provlabs.vault.v1.VaultAccount",
		(*authtypes.GenesisAccount)(nil),
		&types.VaultAccount{},
	)
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
	exampleVaultAddr := "pb1z3x5c7v9b2n4m6f8h0j1k3l5p7r9s0t2w4y6"
	exampleOwnerAddr := "pb1a2b3c4d5e6f7g8h9j0k1l2m3n4p5q6r7s8t"
	exampleBridgeAddr := "pb1b2r3i4d5g6e7a8d9d0e1m2o3s4i5g6n7e8r9"
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
					Example:   fmt.Sprintf("%s create %s nhash svnhash", txStart, exampleAdminAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "underlying_asset"},
						{ProtoField: "share_denom"},
					},
				},
				{
					RpcMethod: "SwapIn",
					Use:       "swap-in [owner] [vault_address] [assets]",
					Alias:     []string{"si"},
					Short:     "Deposit underlying assets into a vault to mint shares",
					Example:   fmt.Sprintf("%s swap-in %s %s 1000nhash", txStart, exampleOwnerAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "owner"},
						{ProtoField: "vault_address"},
						{ProtoField: "assets"},
					},
				},
				{
					RpcMethod: "SwapOut",
					Use:       "swap-out [owner] [vault_address] [assets]",
					Alias:     []string{"so"},
					Short:     "Withdraw underlying assets from a vault by burning shares",
					Example:   fmt.Sprintf("%s swap-out %s %s 100svnhash", txStart, exampleOwnerAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "owner"},
						{ProtoField: "vault_address"},
						{ProtoField: "assets"},
					},
				},
				{
					RpcMethod: "UpdateMinInterestRate",
					Use:       "update-min-interest-rate [admin] [vault_address] [min_rate]",
					Alias:     []string{"umir"},
					Short:     "Sets the vault's minimum annual interest rate (e.g., \"0.9\" for 90% and \"0.9001353\" for 90.01353%) or clears it when not provided.",
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
					Short:     "Sets the vault's maximum annual interest rate (e.g., \"0.9\" for 90% and \"0.9001353\" for 90.01353%) or clears it when not provided.",
					Example:   fmt.Sprintf("%s update-max-interest-rate %s %s 0.1", txStart, exampleAdminAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "max_rate"},
					},
				},
				{
					RpcMethod: "UpdateInterestRate",
					Use:       "update-interest-rate [admin] [vault_address] [new_rate]",
					Alias:     []string{"uir"},
					Short:     "Updates the current annual interest rate (e.g., \"0.9\" for 90% and \"0.9001353\" for 90.01353%) for the vault.",
					Example:   fmt.Sprintf("%s update-interest-rate %s %s 0.05", txStart, exampleAdminAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "new_rate"},
					},
				},
				{
					RpcMethod: "ToggleSwapIn",
					Use:       "toggle-swap-in [admin] [vault_address] [enabled]",
					Alias:     []string{"tsi"},
					Short:     "Enable or disable swap-in operations for a vault",
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
					Example:   fmt.Sprintf("%s toggle-swap-out %s %s false", txStart, exampleAdminAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "enabled"},
					},
				},
				{
					RpcMethod: "DepositInterestFunds",
					Use:       "deposit-interest-funds [admin] [vault_address] [amount]",
					Alias:     []string{"dif"},
					Short:     "Deposit funds into a vault for paying interest",
					Example:   fmt.Sprintf("%s deposit-interest-funds %s %s 5000nhash", txStart, exampleAdminAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "amount"},
					},
				},
				{
					RpcMethod: "WithdrawInterestFunds",
					Use:       "withdraw-interest-funds [admin] [vault_address] [amount]",
					Alias:     []string{"wif"},
					Short:     "Withdraw unused interest funds from a vault",
					Example:   fmt.Sprintf("%s withdraw-interest-funds %s %s 1000nhash", txStart, exampleAdminAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "amount"},
					},
				},
				{
					RpcMethod: "DepositPrincipalFunds",
					Use:       "deposit-principal-funds [admin] [vault_address] [amount]",
					Alias:     []string{"dpf"},
					Short:     "Deposit principal funds into the vault’s marker",
					Example:   fmt.Sprintf("%s deposit-principal-funds %s %s 100000nhash", txStart, exampleAdminAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "amount"},
					},
				},
				{
					RpcMethod: "WithdrawPrincipalFunds",
					Use:       "withdraw-principal-funds [admin] [vault_address] [amount]",
					Alias:     []string{"wpf"},
					Short:     "Withdraw principal funds from the vault’s marker",
					Example:   fmt.Sprintf("%s withdraw-principal-funds %s %s 10000nhash", txStart, exampleAdminAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "amount"},
					},
				},
				{
					RpcMethod: "ExpeditePendingSwapOut",
					Use:       "expedite-pending-swap-out [admin] [request_id]",
					Alias:     []string{"epso"},
					Short:     "Expedite a pending swap out from a vault",
					Example:   fmt.Sprintf("%s expedite-pending-swap-out %s 1", txStart, exampleAdminAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "request_id"},
					},
				},
				{
					RpcMethod: "PauseVault",
					Use:       "pause [admin] [vault_address] [reason]",
					Alias:     []string{"pv"},
					Short:     "Pause a vault, disabling all user-facing operations",
					Example:   fmt.Sprintf("%s pause %s %s 'rebalancing collateral'", txStart, exampleAdminAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "reason"},
					},
				},
				{
					RpcMethod: "UnpauseVault",
					Use:       "unpause [admin] [vault_address]",
					Alias:     []string{"upv"},
					Short:     "Unpause a vault, re-enabling all user-facing operations",
					Example:   fmt.Sprintf("%s unpause %s %s", txStart, exampleAdminAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
					},
				},
				{
					RpcMethod: "SetBridgeAddress",
					Use:       "set-bridge-address [admin] [vault_address] [bridge_address]",
					Alias:     []string{"sba"},
					Short:     "Set the single external bridge address allowed to mint/burn shares for a vault",
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
					Example:   fmt.Sprintf("%s bridge-burn-shares %s %s 100svnhash", txStart, exampleBridgeAddr, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "bridge"},
						{ProtoField: "vault_address"},
						{ProtoField: "shares"},
					},
				},
				{
					RpcMethod: "SetShareDenomMetadata",
					Use:       "set-share-denom-metadata [admin] [vault_address] [metadata]",
					Alias:     []string{"ssdm"},
					Short:     "Set Bank metadata for a vault’s share denom",
					Example:   fmt.Sprintf("%s set-share-denom-metadata %s %s '%s'", txStart, exampleAdminAddr, exampleVaultAddr, exampleMetadata),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "admin"},
						{ProtoField: "vault_address"},
						{ProtoField: "metadata"},
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
					Example:   fmt.Sprintf("%s list", queryStart),
				},
				{
					RpcMethod: "Vault",
					Use:       "get [id]",
					Alias:     []string{"g"},
					Short:     "Query a specific vault's configuration and state by vault address or share denom",
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
					Example:   fmt.Sprintf("%s estimate-swap-out %s 1000000 nhash", queryStart, exampleVaultAddr),
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "vault_address"},
						{ProtoField: "shares"},
						{ProtoField: "redeem_denom"},
					},
				},
				{
					RpcMethod: "PendingSwapOuts",
					Use:       "pending-swap-outs",
					Alias:     []string{"pso"},
					Short:     "Query all pending swap outs",
					Example:   fmt.Sprintf("%s pending-swap-outs", queryStart),
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
