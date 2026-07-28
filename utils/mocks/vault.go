package mocks

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"

	"cosmossdk.io/core/header"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	codectestutil "github.com/cosmos/cosmos-sdk/codec/testutil"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	metadatatypes "github.com/provenance-io/provenance/x/metadata/types"
)

var _ types.AccountKeeper = (*MockAuthKeeper)(nil)

type MockAuthKeeper struct {
	accounts      map[string]sdk.AccountI
	nextAccNumber uint64
}

func NewMockAuthKeeper() *MockAuthKeeper {
	return &MockAuthKeeper{
		accounts:      make(map[string]sdk.AccountI),
		nextAccNumber: 1,
	}
}

func addrKey(addr sdk.AccAddress) string { return string(addr) }

// NewAccount assigns an account number if needed and returns the (possibly updated) account.
func (m *MockAuthKeeper) NewAccount(_ context.Context, acc sdk.AccountI) sdk.AccountI {
	if acc.GetAccountNumber() == 0 {
		if ba, ok := acc.(*authtypes.BaseAccount); ok {
			_ = ba.SetAccountNumber(m.nextAccNumber)
			m.nextAccNumber++
			return ba
		}
		if withNum, ok := acc.(interface{ SetAccountNumber(uint64) error }); ok {
			_ = withNum.SetAccountNumber(m.nextAccNumber)
			m.nextAccNumber++
			return acc
		}
		m.nextAccNumber++
	}
	return acc
}

func (m *MockAuthKeeper) NewAccountWithAddress(_ context.Context, addr sdk.AccAddress) sdk.AccountI {
	return authtypes.NewBaseAccountWithAddress(addr)
}

func (m *MockAuthKeeper) GetAccount(_ context.Context, addr sdk.AccAddress) sdk.AccountI {
	return m.accounts[addrKey(addr)]
}

func (m *MockAuthKeeper) GetAllAccounts(_ context.Context) []sdk.AccountI {
	out := make([]sdk.AccountI, 0, len(m.accounts))
	for _, a := range m.accounts {
		out = append(out, a)
	}
	return out
}

func (m *MockAuthKeeper) HasAccount(_ context.Context, addr sdk.AccAddress) bool {
	_, ok := m.accounts[addrKey(addr)]
	return ok
}

func (m *MockAuthKeeper) SetAccount(_ context.Context, acc sdk.AccountI) {
	m.accounts[addrKey(acc.GetAddress())] = acc
}

func (m *MockAuthKeeper) GetModuleAddress(moduleName string) sdk.AccAddress {
	return sdk.AccAddress(moduleName)
}

var _ types.MetadataKeeper = (*MockMetadataKeeper)(nil)

// MockMetadataKeeper holds the scopes a test wants the vault module to find. With none
// recorded it reports every scope as missing, so the NAV denom check returns its error for
// an nft/<scope-id> denom rather than dereferencing a nil keeper.
type MockMetadataKeeper struct {
	scopes map[string]metadatatypes.Scope
}

// NewMockMetadataKeeper returns a MockMetadataKeeper with no scopes recorded.
func NewMockMetadataKeeper() *MockMetadataKeeper {
	return &MockMetadataKeeper{scopes: make(map[string]metadatatypes.Scope)}
}

// SetScope records a scope so GetScope reports it as present, letting a test price an
// nft/<scope-id> NAV denom.
func (m *MockMetadataKeeper) SetScope(scope metadatatypes.Scope) {
	m.scopes[scope.ScopeId.String()] = scope
}

// GetScope returns a recorded scope and whether it was found, satisfying
// types.MetadataKeeper.
func (m *MockMetadataKeeper) GetScope(_ sdk.Context, id metadatatypes.MetadataAddress) (metadatatypes.Scope, bool) {
	scope, found := m.scopes[id.String()]
	return scope, found
}

// NewVaultKeeper returns an instance of the Keeper with all dependencies mocked.
func NewVaultKeeper(
	t testing.TB,
) (sdk.Context, *keeper.Keeper) {
	key := storetypes.NewKVStoreKey(types.ModuleName)
	tkey := storetypes.NewTransientStoreKey(fmt.Sprintf("transient_%s", types.ModuleName))
	wrapper := testutil.DefaultContextWithDB(t, key, tkey)
	cfg := MakeTestEncodingConfig("provlabs")
	sdkCfg := sdk.GetConfig()
	sdkCfg.SetBech32PrefixForAccount("provlabs", "provlabspub")
	sdkCfg.SetBech32PrefixForValidator("provlabsvaloper", "provlabsvaloperpub")
	sdkCfg.SetBech32PrefixForConsensusNode("provlabsvalcons", "provlabsvalconspub")
	types.RegisterInterfaces(cfg.InterfaceRegistry)
	authMock := NewMockAuthKeeper()

	k := keeper.NewKeeper(
		cfg.Codec,
		runtime.NewKVStoreService(key),

		runtime.ProvideEventService(),
		addresscodec.NewBech32Codec("provlabs"),
		authtypes.NewModuleAddress(govtypes.ModuleName),
		authMock,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		NewMockMetadataKeeper(),
	)

	ctx := wrapper.Ctx.WithHeaderInfo(header.Info{Time: time.Now().UTC()})
	return ctx, k
}

// MakeTestEncodingConfig is a modified testutil.MakeTestEncodingConfig that
// sets a custom Bech32 prefix in the interface registry.
func MakeTestEncodingConfig(prefix string, modules ...module.AppModuleBasic) moduletestutil.TestEncodingConfig {
	aminoCodec := codec.NewLegacyAmino()
	interfaceRegistry := codectestutil.CodecOptions{
		AccAddressPrefix: prefix,
	}.NewInterfaceRegistry()
	codec := codec.NewProtoCodec(interfaceRegistry)

	encCfg := moduletestutil.TestEncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Codec:             codec,
		TxConfig:          tx.NewTxConfig(codec, tx.DefaultSignModes),
		Amino:             aminoCodec,
	}

	mb := module.NewBasicManager(modules...)

	std.RegisterLegacyAminoCodec(encCfg.Amino)
	std.RegisterInterfaces(encCfg.InterfaceRegistry)
	mb.RegisterLegacyAminoCodec(encCfg.Amino)
	mb.RegisterInterfaces(encCfg.InterfaceRegistry)

	return encCfg
}
