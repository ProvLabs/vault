package types_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
)

func TestNewEventNAVUpdated(t *testing.T) {
	vaultAddr := utils.TestAddress().Bech32
	signer := utils.TestAddress().Bech32

	tests := []struct {
		name           string
		vaultAddress   string
		nav            types.VaultNAV
		signer         string
		expectedVault  string
		expectedDenom  string
		expectedPrice  string
		expectedVolume string
		expectedSource string
		expectedSigner string
		expectedHeight int64
	}{
		{
			name:         "all fields populated including source and block height",
			vaultAddress: vaultAddr,
			nav: types.VaultNAV{
				Denom:              "rwa",
				Price:              sdk.NewInt64Coin("under", 1_000_000),
				Volume:             sdkmath.NewInt(500_000),
				Source:             "oracle-x",
				UpdatedBlockHeight: 42,
			},
			signer:         signer,
			expectedVault:  vaultAddr,
			expectedDenom:  "rwa",
			expectedPrice:  "1000000under",
			expectedVolume: "500000",
			expectedSource: "oracle-x",
			expectedSigner: signer,
			expectedHeight: 42,
		},
		{
			name:         "empty source and zero block height",
			vaultAddress: vaultAddr,
			nav: types.VaultNAV{
				Denom:              "usdc",
				Price:              sdk.NewInt64Coin("nhash", 250),
				Volume:             sdkmath.NewInt(1),
				Source:             "",
				UpdatedBlockHeight: 0,
			},
			signer:         signer,
			expectedVault:  vaultAddr,
			expectedDenom:  "usdc",
			expectedPrice:  "250nhash",
			expectedVolume: "1",
			expectedSource: "",
			expectedSigner: signer,
			expectedHeight: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			event := types.NewEventNAVUpdated(tc.vaultAddress, tc.nav, tc.signer)
			assert.Equal(t, tc.expectedVault, event.VaultAddress, "VaultAddress mismatch for case: %s", tc.name)
			assert.Equal(t, tc.expectedDenom, event.Denom, "Denom mismatch for case: %s", tc.name)
			assert.Equal(t, tc.expectedPrice, event.Price, "Price mismatch for case: %s — expected sdk.Coin.String() encoding", tc.name)
			assert.Equal(t, tc.expectedVolume, event.Volume, "Volume mismatch for case: %s — expected sdkmath.Int.String() encoding", tc.name)
			assert.Equal(t, tc.expectedSource, event.Source, "Source mismatch for case: %s", tc.name)
			assert.Equal(t, tc.expectedSigner, event.Signer, "Signer mismatch for case: %s", tc.name)
			assert.Equal(t, tc.expectedHeight, event.UpdatedBlockHeight, "UpdatedBlockHeight mismatch for case: %s", tc.name)
		})
	}
}

func TestNewEventNAVAuthorityUpdated(t *testing.T) {
	vaultAddr := utils.TestAddress().Bech32
	admin := utils.TestAddress().Bech32
	newAuthority := utils.TestAddress().Bech32

	tests := []struct {
		name            string
		vaultAddress    string
		admin           string
		newAuthority    string
		expectedVault   string
		expectedAdmin   string
		expectedNewAuth string
	}{
		{
			name:            "all fields set",
			vaultAddress:    vaultAddr,
			admin:           admin,
			newAuthority:    newAuthority,
			expectedVault:   vaultAddr,
			expectedAdmin:   admin,
			expectedNewAuth: newAuthority,
		},
		{
			name:            "empty new authority",
			vaultAddress:    vaultAddr,
			admin:           admin,
			newAuthority:    "",
			expectedVault:   vaultAddr,
			expectedAdmin:   admin,
			expectedNewAuth: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			event := types.NewEventNAVAuthorityUpdated(tc.vaultAddress, tc.admin, tc.newAuthority)
			assert.Equal(t, tc.expectedVault, event.VaultAddress, "VaultAddress mismatch for case: %s", tc.name)
			assert.Equal(t, tc.expectedAdmin, event.Admin, "Admin mismatch for case: %s", tc.name)
			assert.Equal(t, tc.expectedNewAuth, event.NewAuthority, "NewAuthority mismatch for case: %s", tc.name)
		})
	}
}
