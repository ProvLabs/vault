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
- **Naming over Comments**: Use clear variable naming and logic flow to explain intent. Avoid redundant comments; use comments only for counter-intuitive business logic or complex architectural context.
- **Cleanliness**: Proactively eliminate unused code and obsolete comments.
- **Error Handling**: **Always** wrap errors with context: `fmt.Errorf("failed to [action]: %w", err)`.
- **Logging**: Use the module-scoped logger via `k.getLogger(ctx)`. Log messages should be structured and informative.
- **Context Awareness**: `ctx sdk.Context` (or `context.Context` for gRPC) must be the first argument for all service-layer functions.

### Documentation
- **Godocs for Everything**: Every exported function, interface, and type must have clear, high-context Godoc comments describing the "why" and architectural context.
- **Protobuf Documentation**: Ensure all fields and messages in `.proto` files are thoroughly documented, as these propagate to generated code and public API specs.
- **GEMINI.md**: Update this file when adding new modules or significant architectural patterns.

### Testing
- **Table-Driven Tests**: Use standard table-driven patterns for unit and integration tests to ensure exhaustive coverage of edge cases.
- **Meaningful Require/Assert Messages**: **Every** assertion must include a descriptive failure message that provides context:
    - *Good*: `s.Require().NoError(err, "failed to create vault for share denom %s", sharedenom)`
    - *Good*: `s.Equal(expectedSupply, actualSupply, "vault marker supply mismatch after swap-in for user %s", userAddr)`

### Module Development Lifecycle
1. **Schema**: Define state and interfaces in `proto/` (with full documentation) and run `make proto-all`.
2. **Validation**: Implement message validation and type logic in `types/`.
3. **Logic**: Build core functionality in `keeper/` using descriptive naming and minimal comments.
4. **Exposure**: Connect logic to `MsgServer` and `QueryServer`.
5. **Verification**: Validate via exhaustive table-driven tests with meaningful failure messages.

---

## Tooling & Automation
- **`buf`**: For Protobuf linting, formatting, and generation.
- **`make proto-all`**: Regenerates all Go code from Protobuf files.
- **`./local.sh`**: Spins up a local single-node blockchain (`vaulty-1`) for development.
- **`./scripts/`**: Use `tx.sh` and `query.sh` for easy interaction with the local node.
- **`go test ./...`**: Runs the full test suite.

### Task Context Persistence (`.gemini/context.json`)
For multi-stage tasks, track state across steps to ensure continuity.
- **Location**: `.gemini/context.json`
- **Usage**:
    - **Initialization**: Populate `run_id` and `global_inputs`.
    - **Step Updates**: Append success/failure status and specific outputs (e.g., `tx_hash`) to the `steps` array.

#### Context Template
```json
{
  "run_id": "unix_timestamp",
  "global_inputs": {
    "vault_address": "string",
    "underlying_denom": "string"
  },
  "steps": [
    {
      "project": "vault",
      "status": "success | failed",
      "outputs": {
        "tx_hash": "string"
      }
    }
  ]
}
```
