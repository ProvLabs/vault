package keeper_test

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	attrtypes "github.com/provenance-io/provenance/x/attribute/types"
)

func (s *TestSuite) TestCheckPayoutRestrictions() {
	requiredAttr := "kyc.verified"

	tests := []struct {
		name          string
		isRestricted  bool
		hasAttribute  bool
		expectedError string
	}{
		{
			name:         "Success - Unrestricted asset",
			isRestricted: false,
			hasAttribute: false,
		},
		{
			name:         "Success - Restricted asset with required attribute",
			isRestricted: true,
			hasAttribute: true,
		},
		{
			name:          "Failure - Restricted asset without required attribute",
			isRestricted:  true,
			hasAttribute:  false,
			expectedError: "failed to pass send restrictions test",
		},
	}

	for i, tc := range tests {
		s.Run(tc.name, func() {
			// s.SetupTest() is called by the suite before each Test* method, 
			// but s.Run subtests might need their own isolation if they share state.
			// However, requireAddFinalizeAndActivateMarker will fail if denom is reused.
			underlyingDenom := fmt.Sprintf("underlying%d", i)
			shareDenom := fmt.Sprintf("vshare%d", i)

			// 1. Setup Underlying Asset Marker
			var reqAttrs []string
			if tc.isRestricted {
				reqAttrs = []string{requiredAttr}
			}
			s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlyingDenom, 1_000_000), s.adminAddr, reqAttrs...)

			// 2. Create Vault
			vault := s.setupBaseVault(underlyingDenom, shareDenom)

			// 3. Setup Owner (Recipient)
			ownerAddr := s.CreateAndFundAccount(sdk.NewInt64Coin("stake", 1))
			if tc.hasAttribute {
				if !s.simApp.NameKeeper.NameExists(s.ctx, requiredAttr) {
					s.Require().NoError(s.simApp.NameKeeper.SetNameRecord(s.ctx, requiredAttr, s.adminAddr, false))
				}
				expireTime := s.ctx.BlockTime().Add(24 * time.Hour)
				attr := attrtypes.NewAttribute(requiredAttr, ownerAddr.String(), attrtypes.AttributeType_String, []byte("true"), &expireTime, "")
				s.Require().NoError(s.simApp.AttributeKeeper.SetAttribute(s.ctx, attr, s.adminAddr))
			}

			// 4. Test checkPayoutRestrictions
			assets := sdk.NewInt64Coin(underlyingDenom, 100)
			err := s.k.TestAccessor_checkPayoutRestrictions(s.T(), s.ctx, vault, ownerAddr, assets)

			if tc.expectedError == "" {
				s.Require().NoError(err, "should not error for case: %s", tc.name)
			} else {
				s.Require().Error(err, "should error for case: %s", tc.name)
				s.Require().Contains(err.Error(), tc.expectedError, "error message mismatch for case: %s", tc.name)
			}
		})
	}
}
