---
name: go-testing-standards
description: The vault repo's testing conventions — table-driven tests, Rule-of-Three setup extraction into suite_test.go, descriptive test names, meaningful assertion failure messages, full branch coverage. Invoke before writing, reviewing, or refactoring any *_test.go file in this repo.
---

# Go Testing Standards (vault repo)

These conventions are mandatory for unit and integration tests in this repo. Apply them whether you are writing tests from scratch, adding cases to an existing test, or reviewing test code.

## 1. Table-driven tests (mandatory)

Every unit and integration test uses a table-driven pattern. Sequential tests are converted to table-driven the first time a new case is added to them.

```go
tests := []struct {
    name                string
    // setup fields
    // input fields
    // expected fields
    expectErr           bool
    expectedErrContains string
}{
    {
        name: "interest period has elapsed, should pay interest",
        // ...
    },
}
for _, tc := range tests {
    s.Run(tc.name, func() {
        // test body
    })
}
```

## 2. Descriptive test case names

Each `name` reads like a business rule, not a developer label.

- Good: `"interest period has elapsed, should pay interest"`
- Good: `"vault marker is restricted, swap-in requires attribute"`
- Bad: `"test1"`, `"happy path"`, `"error"`

## 3. Rule of Three — extract setup helpers

If a setup block (marker creation, vault setup, account funding, state initialization) appears in **three or more** tests, it MUST be extracted into a helper in `suite_test.go`.

- Scan the entire test file before writing a new test. If duplication exists, extract it BEFORE adding the new test.
- Never duplicate setup logic that already has a helper.

## 4. Meaningful require/assert messages

EVERY assertion includes a descriptive failure message with relevant context (denoms, addresses, amounts). The message must let someone debugging the failure see *which* case failed without rerunning.

- Good: `s.Require().NoError(err, "failed to create vault for share denom %s", shareDenom)`
- Good: `s.Equal(expectedSupply, actualSupply, "vault marker supply mismatch after swap-in for user %s", userAddr)`
- Bad: `s.Require().NoError(err)` — no message
- Bad: `s.Equal(expected, actual, "values should be equal")` — generic message

## 5. Use `Require` vs `Assert` deliberately

- `s.Require()` halts the test on failure. Use it for setup steps and for assertions where later code would panic on bad state.
- `s.Assert()` / `s.Equal()` etc. without Require keep going. Use them for independent checks where one failure shouldn't mask the others.

## 6. Error path coverage

- Test every error path explicitly, not just the happy path.
- When testing errors, verify the message contains the expected context: `s.ErrorContains(err, "expected fragment", "failure message for this case")`.
- Validate error wrapping conforms to `fmt.Errorf("failed to [action]: %w", err)` (or `errorsmod.Wrap...` for cosmos-sdk error types).

## 7. Coverage targets

When working off a branch diff, aim for **near 100% coverage of changed code** — not the whole codebase. For each changed function:
- All `if/else`, `switch`, and `select` branches.
- All boundary conditions and edge cases.
- All error returns.
- All state mutations and their expected post-conditions.

After writing tests, walk each branch of the changed code and confirm at least one case exercises it. Call out any branch you couldn't cover and why.

## 8. Context threading

- `ctx` is the first argument in all service-layer calls under test.
- Use `sdk.Context` for keeper/module tests and `context.Context` for gRPC server tests.

## 9. Number formatting in tests

Test amounts use underscore separators too: `sdk.NewInt(1_000_000)`, not `sdk.NewInt(1000000)`.

## 10. Logging assertions

Only assert on log output when it is directly relevant to the case being validated. Don't over-specify.

## 11. No Godocs on test functions

`TestXxx` functions and methods do NOT require Godoc comments, and reviewers should not request them. The general "Godocs on every exported symbol" rule does not apply to tests — a `TestXxx` method's intent must instead be carried by:

- a descriptive test name (`TestKeeper_SetVaultNAV_RejectsZeroPriceForAcceptedDenom`),
- descriptive `name` fields on each table case,
- descriptive variable names, and
- high-context `Require`/`Assert` failure messages.

This keeps large, fast-moving suites maintainable. Adding a Godoc is allowed when it genuinely clarifies a subtle scenario, but it is never required and its absence is not a finding.

Test helpers, shared fixtures, and the suite type/methods in `suite_test.go` are NOT tests — they are exported support code and still require Godocs per the standard rule.

## Self-review checklist

Before declaring tests done, confirm:

- [ ] All test cases are table-driven
- [ ] Every case has a descriptive name
- [ ] Every assertion has a meaningful failure message with context
- [ ] No setup duplication (Rule of Three enforced)
- [ ] All error paths covered
- [ ] All happy paths covered
- [ ] Large numeric literals use underscore separators
- [ ] No inline comments that restate the code
- [ ] Test functions are not flagged for missing Godocs (helpers/suite code still need them)
- [ ] `ctx` is first argument in all service-layer calls
