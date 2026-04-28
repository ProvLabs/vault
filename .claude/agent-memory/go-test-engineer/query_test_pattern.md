---
name: Query server test pattern via querytest harness
description: utils/query/querytest provides TestDef + RunTestCase for query server endpoints
type: project
---

Query tests use `querytest.TestDef[Req, Resp]` + `querytest.RunTestCase(s, testDef, tc)`. Each case provides:
- `Name`, `Req` (or `Setup` to populate state)
- `Expected` for full equality, OR `ManualEquality` callback for partial / view-based comparison
- `ExpectedErrSubstrs` for negative cases

**Why:** Avoids hand-rolling response equality for nested proto messages; supports projecting only the fields you care about via `ManualEquality`.

**How to apply:** When testing the new `LiquidityBreakdown` field on `QueryVault`, extend `TestQueryServer_Vault` cases with seeded NFT markers / NAVs and assert via `ManualEquality` (or extend `Expected.LiquidityBreakdown`). Don't introduce a separate test function for the field; keep cases inside the existing table.
