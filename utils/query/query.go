package query

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDef is the definition of a QueryServer endpoint to be tested.
// R is the request message type. S is the response message type.
type TestDef[R any, S any] struct {
	// QueryName is the name of the query being tested.
	QueryName string
	// Query is the query function to invoke.
	Query func(goCtx context.Context, req *R) (*S, error)
	// PostCheck is a function that runs any desired followup assertions to help pinpoint
	// differences between the expected and actual. It's only called if they're not equal and neither are nil.
	PostCheck func(expected, actual *S)
}

// TestCase is a test case for a QueryServer endpoint.
// R is the request message type. S is the response message type.
type TestCase[R any, S any] struct {
	// Name is the name of the test case.
	Name string
	// Setup is a function that does any needed app/state setup.
	// A cached context is used for tests, so this setup will not carry over between test cases.
	Setup func()
	// Req is the request message to provide to the query.
	Req *R
	// ExpectedResp is the expected response from the query
	ExpectedResp *S
	// ExpectedErrSubstrs is the strings that are expected to be in the error returned by the endpoint.
	// If empty, that error is expected to be nil.
	ExpectedErrSubstrs []string
}

type TestSuiter interface {
	Context() sdk.Context
	SetContext(ctx sdk.Context)
	Require() *require.Assertions
	Assert() *assert.Assertions
}

// RunTestCase runs a unit test on a QueryServer endpoint.
// A cached context is used so each test case won't affect the others.
// R is the request message type. S is the response message type.
func RunTestCase[R any, S any](s TestSuiter, td TestDef[R, S], tc TestCase[R, S]) {
	origCtx := s.Context()
	defer func() {
		s.SetContext(origCtx)
	}()
	ctx, _ := s.Context().CacheContext()
	s.SetContext(ctx)

	if tc.Setup != nil {
		tc.Setup()
	}

	goCtx := sdk.WrapSDKContext(s.Context())
	var resp *S
	var err error
	testFunc := func() {
		resp, err = td.Query(goCtx, tc.Req)
	}
	s.Require().NotPanics(testFunc, td.QueryName)

	if len(tc.ExpectedErrSubstrs) == 0 {
		s.Assert().NoErrorf(err, "%s error", td.QueryName)
		s.Assert().Equalf(tc.ExpectedResp, resp, "%s response", td.QueryName)
	} else {
		s.Assert().Errorf(err, "%s error", td.QueryName)
		for _, substr := range tc.ExpectedErrSubstrs {
			s.Assert().Containsf(err.Error(), substr, "%s error missing expected substring", td.QueryName)
		}
		return
	}

	if td.PostCheck != nil && tc.ExpectedResp != nil && resp != nil {
		td.PostCheck(tc.ExpectedResp, resp)
	}
}
