package keeper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provenance-io/provenance/testutil/assertions"
)

// assertErrorContentsf is a wrapper for assertions.AssertErrorContentsf for this TestSuite.
func (s *TestSuite) assertErrorContentsf(theError error, contains []string, msg string, args ...interface{}) bool {
	s.T().Helper()
	return assertions.AssertErrorContentsf(s.T(), theError, contains, msg, args...)
}

// assertEqualEvents is a wrapper for assertions.AssertEqualEvents for this TestSuite.
func (s *TestSuite) assertEqualEvents(expected, actual sdk.Events, msgAndArgs ...interface{}) bool {
	s.T().Helper()
	return assertions.AssertEqualEvents(s.T(), expected, actual, msgAndArgs...)
}
