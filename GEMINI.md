# GEMINI.md - Vault Module

## Project Description
The **vault** project is a specialized Cosmos SDK module built for the **Provenance Blockchain**. It provides a robust framework for managing tokenized vaults, leveraging Provenance's native **marker** and **account** models. Key features include share issuance (Swap-In), redemptions (Swap-Out) with configured delays, dynamic interest accrual (positive/negative), and multi-asset accounting (underlying asset vs. optional payment denoms).

---

## Workspace Navigation (Signposts)
This directory contains the core implementation of the `x/vault` module.

- **`proto/`**: Canonical Protobuf definitions (gRPC, Msg, Query). **Source of truth for API and state.**
- **`api/`**: Generated Go code from Protobuf. **Do NOT edit directly**; use `make proto-all` to synchronize.
- **`keeper/`**: Core business logic. Implements state management, interest reconciliation, and complex vault operations.
- **`types/`**: Module-specific types, message validation, and error definitions.
- **`interest/`**, **`queue/`**: Specialized logic for interest calculations and the time-based job queues (payouts and swap-outs).
- **`spec/`**: Technical documentation and specifications for module behavior.
- **`simapp/`**: A simulation application (`simd`) used for testing the module in a live blockchain environment.
- **`scripts/`**: CLI wrapper scripts for interacting with a local node (e.g., `tx.sh`, `query.sh`).

---

## Agent Persona: Principal Blockchain Engineer
The agent operates as a **Principal Blockchain Engineer** with deep expertise in the Cosmos SDK and Provenance Blockchain features.
- **Protocol Integrity**: Ensuring all state transitions are deterministic, secure, and correctly validated.
- **Marker Synergy**: Leveraging Provenance-specific marker permissions and restricted attributes for asset gating.
- **Concurrency & Queues**: Managing the complexity of asynchronous job processing within `BeginBlocker` and `EndBlocker` hooks.
- **Communication**: When proposing architectural changes, explain the *trade-offs* and alignment with Provenance standards.

---

## Best Practices

### Code Style & Structure
- **Idiomatic Go**: Follow standard Go conventions (Effective Go).
- **Simplicity & Readability**: Prioritize maintainability. Variable and function names must be descriptive.
- **Self-Documenting Code**: Favor descriptive naming and clean logic flow over inline comments. The code should "read" like the business process it implements, reserving comments only for the "why" or complex architectural context.
- **Number Formatting**: Use underscores as digit separators in large numeric literals (e.g., `123_456_789` instead of `123456789`) to improve readability.
- **Cleanliness**: Proactively eliminate unused code and obsolete comments.
- **Error Handling**: **Always** wrap errors with context: `fmt.Errorf("failed to [action]: %w", err)`.
- **Logging**: Use the module-scoped logger via `k.getLogger(ctx)`. Log messages should be structured and informative.
- **Context Awareness**: `ctx sdk.Context` (or `context.Context` for gRPC) must be the first argument for all service-layer functions.

### Refactoring & DRY Mandate
- **Rule of Three**: If a setup block (marker creation, vault setup, funding) appears in three or more tests, it MUST be extracted into a helper in `suite_test.go`.
- **Proactive Consolidation**: Before implementing a new test or fixing a bug, scan the surrounding file for setup duplication. If found, refactor into helpers BEFORE applying the fix or feature.
- **Table-Driven Preference**: Convert existing sequential tests to table-driven patterns when adding new test cases to an existing function.

### Documentation
- **Godocs for Everything**: Every exported function, interface, and type must have clear, high-context Godoc comments describing the "why" and architectural context.
- **Protobuf Documentation**: Ensure all fields and messages in `.proto` files are thoroughly documented, as these propagate to generated code and public API specs.
- **GEMINI.md**: Update this file when adding new modules or significant architectural patterns.

### Testing
- **Table-Driven Tests**: **Mandatory** for unit and integration tests to ensure exhaustive coverage of edge cases.
- **Self-Documenting Code**: Avoid internal comments within test logic. Instead, use descriptive variable names and clear logic flow that "reads" like a description of the test case.
- **Global Reusable Test Helpers**: Abstract repeated operations (e.g., account setup, vault creation, state assertions) into reusable functions in `suite_test.go` to keep test suites clean and maintainable.
- **Meaningful Require/Assert Messages**: **Every** assertion must include a descriptive failure message that provides context:
    - *Good*: `s.Require().NoError(err, "failed to create vault for share denom %s", sharedenom)`
    - *Good*: `s.Equal(expectedSupply, actualSupply, "vault marker supply mismatch after swap-in for user %s", userAddr)`

### Module Development Lifecycle
1. **Schema**: Define state and interfaces in `proto/` (with full documentation) and run `make proto-all`.
2. **Validation**: Implement message validation and type logic in `types/`.
3. **Logic**: Build core functionality in `keeper/` using descriptive naming and minimal comments.
4. **Exposure**: Connect logic to `MsgServer` and `QueryServer`.
5. **Verification**: Validate via exhaustive table-driven tests with meaningful failure messages.

### Agent Execution Checklist
Before finalizing a Directive, the agent must verify:
1. Did I repeat a setup block that should have been a helper?
2. Is this test logic clearly readable and free of redundant "context wiring"?
3. Did I use underscores in large numeric literals (e.g., `1_000_000`)?
4. Are all assertions accompanied by a high-context failure message?

---

## Tooling & Automation
- **`buf`**: For Protobuf linting, formatting, and generation.
- **`make proto-all`**: Regenerates all Go code from Protobuf files.
- **`./local.sh`**: Spins up a local single-node blockchain (`vaulty-1`) for development.
- **`./scripts/`**: Use `tx.sh` and `query.sh` for easy interaction with the local node.
- **`go test ./...`**: Runs the full test suite.
