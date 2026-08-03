package keeper_test

import (
	"github.com/provlabs/vault/types"
)

func (s *TestSuite) TestIsVaultCreationGovOnly() {
	tests := []struct {
		name     string
		setup    func()
		expected bool
	}{
		{
			name:     "gate enabled in params",
			setup:    func() { s.SetGovOnlyVaultCreation(true) },
			expected: true,
		},
		{
			name:     "gate disabled in params",
			setup:    func() { s.SetGovOnlyVaultCreation(false) },
			expected: false,
		},
		{
			name: "no params stored falls back to the module default",
			setup: func() {
				s.Require().NoError(s.k.Params.Remove(s.ctx), "failed to clear stored params")
			},
			expected: types.DefaultParams().GovOnlyVaultCreation,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			origCtx := s.ctx
			defer func() { s.ctx = origCtx }()
			s.ctx, _ = s.ctx.CacheContext()

			tc.setup()

			govOnly, err := s.k.IsVaultCreationGovOnly(s.ctx)
			s.Require().NoError(err, "IsVaultCreationGovOnly should not error")
			s.Assert().Equal(tc.expected, govOnly, "gov-only vault creation gate")
		})
	}
}
