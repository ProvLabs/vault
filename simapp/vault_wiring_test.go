package simapp

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	metadatatypes "github.com/provenance-io/provenance/x/metadata/types"
)

func TestVaultKeeperProvenanceWiring(t *testing.T) {
	app, _ := setup(t, false, 5, "vaulty-1")
	ctx := app.NewContext(true)

	scopeID := metadatatypes.ScopeMetadataAddress(uuid.MustParse("00000000-0000-4000-8000-0000000000aa"))

	tests := []struct {
		name string
		read func()
	}{
		{
			name: "marker keeper",
			read: func() { _, _ = app.VaultKeeper.MarkerKeeper.GetMarkerByDenom(ctx, "notaregisteredmarker") },
		},
		{
			name: "metadata keeper",
			read: func() { _, _ = app.VaultKeeper.MetadataKeeper.GetScope(ctx, scopeID) },
		},
		{
			name: "name keeper",
			read: func() { app.VaultKeeper.NameKeeper.NameExists(ctx, "notaregisteredname") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotPanicsf(t, tt.read,
				"a state read through the vault keeper's %s should reach a configured store; RegisterProvenanceModules must replace the depinject stub, which carries no store key", tt.name)
		})
	}

	require.NotNil(t, app.VaultKeeper.AttrKeeper, "vault keeper AttrKeeper should be wired")
	require.NotNil(t, app.VaultKeeper.ExchangeKeeper, "vault keeper ExchangeKeeper should be wired")
	require.NotNil(t, app.VaultKeeper.ExchangeQueryServer, "vault keeper ExchangeQueryServer should be wired")
}
