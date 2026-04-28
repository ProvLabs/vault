---
name: Msg server table-driven test pattern
description: Generic msgServerTestDef[Req, Resp, postCheckArgs] harness in keeper/msg_server_test.go
type: project
---

Msg server tests use a generic harness pattern, NOT inline assertions:

```go
testDef := msgServerTestDef[types.MsgFooRequest, types.MsgFooResponse, postCheckArgs]{
    endpointName: "Foo",
    endpoint:     keeper.NewMsgServer(s.simApp.VaultKeeper).Foo,
    postCheck: func(msg *types.MsgFooRequest, args postCheckArgs) {
        // verify state
    },
}
tests := []struct {
    name               string
    setup              func()
    msg                types.MsgFooRequest
    postCheckArgs      postCheckArgs
    expectedEvents     sdk.Events
    expectedErrSubstrs []string
}{ ... }
for _, tc := range tests { s.Run(tc.name, func() { runMsgServerTestCase(s, testDef, tc) }) }
```

**Why:** Centralizes assertion plumbing (events, error substrings, postcheck), keeps cases declarative.

**How to apply:** New msg server endpoints (UpdateVaultAssetNAV, VaultDepositAsset, VaultWithdrawAsset, VaultSettleAssetPayment, VaultRejectAssetPayment) should follow this exact pattern. Define a per-test `postCheckArgs` struct describing the state to verify; thread it through tc.postCheckArgs.

For sim ops, the pattern is `s.app.VaultKeeper`/`s.accs`/`s.random` invoked via `op(s.random, s.app.BaseApp, s.ctx, s.accs, "")` and verified with `opMsg.OK`, `opMsg.Name`, `opMsg.Route`, futureOps length.
