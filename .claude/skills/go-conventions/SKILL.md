---
name: go-conventions
description: The vault repo's non-negotiable Go style and engineering standards — error wrapping, godocs, naming, number underscore separators, structured logging, ctx-first arguments, dead-code removal. Invoke before reviewing, writing, or modifying any .go file in this repo.
---

# Go Conventions (vault repo)

These are the standards every change in the vault repo must meet. They are derived from Effective Go and the Go Code Review Comments guide, plus a few repo-specific decisions. Apply them uniformly.

## 1. Idiomatic Go

- Follow Effective Go and the Go Code Review Comments guide.
- Exported identifiers have correct, meaningful names — no stutter (`pkg.PkgFoo` is wrong).
- Receivers are short (single or double letter) and consistent across the file.
- No misuse of `init()`, type aliases, or goroutines/channels where simpler constructs work.

## 2. Simplicity & readability

- Variable and function names are descriptive and unambiguous. Single-letter names are only acceptable as loop indices.
- Logic flows top-to-bottom like the business process it implements. Prefer early returns / guard clauses over deeply nested conditionals.
- If a simpler construct gives the same result, use it.

## 3. Self-documenting code & comment hygiene

- **Inline comments**: every inline comment must explain *why*, not *what*. If a comment restates what the code does, delete the comment and rename/restructure the code so the intent is obvious.
- **Godocs**: every exported function, type, and interface MUST have a Godoc comment that explains *why* and the architectural context. `// Foo does foo.` is not acceptable.
- **Protobuf**: every `.proto` message and field must be documented — proto docs flow into generated code and public APIs.
- **Obsolete comments**: delete comments that reference removed code, outdated behavior, or completed TODOs.

## 4. Number formatting

All large numeric literals use underscore digit separators: `1_000_000`, not `1000000`. This is non-negotiable.

## 5. Error handling

- Every error return from a called function must be wrapped with context: `fmt.Errorf("failed to [specific action]: %w", err)`.
- Bare `return err` or `errors.New(...)` without wrapping is not acceptable when the error originated from a called function.
- The wrapping message must be specific enough to pinpoint the failure location and action without reading the stack.
- For cosmos-sdk error types, use `errorsmod.Wrap` / `errorsmod.Wrapf` instead of `fmt.Errorf` so the typed error code is preserved.

## 6. Logging

- All logging in the `keeper/` package goes through the module-scoped logger via `k.getLogger(ctx)`.
- `ctx.Logger()` is **not permitted** in keeper files — it produces unscoped logs that are harder to filter and attribute to the vault module. The only exception is the `getLogger` helper itself, which wraps `ctx.Logger()` to add the `module=x/vault` field.
- When replacing or writing log calls, preserve existing structured fields (e.g. `"vault"`, `"err"`).
- No `fmt.Println`, `log.Println`, or other unstructured logging.
- Log messages are structured (key-value fields), informative, and not redundant with the error being returned.

## 7. Context propagation

- `ctx sdk.Context` (or `context.Context` for gRPC) is the **first argument** of every service-layer function.
- Never store `ctx` in a struct.

## 8. Dead code & reuse

- Actively scan for functions, methods, types, and variables that are defined but never referenced. Flag them for removal.
- Before adding a new method, check whether an existing helper already does the same or similar thing. Call the existing helper instead of duplicating logic.

## 9. GEMINI.md updates

If a change introduces a new module or a significant architectural pattern, flag that `GEMINI.md` needs to be updated and suggest what to document.

## Severity language for findings

When reviewing or proposing changes, label findings with one of:
- **BLOCKER** — must fix before merge (correctness, security, or convention violations that affect public API or state).
- **MAJOR** — strong recommendation (will create tech debt or rework if shipped).
- **MINOR** — polish or preference.

Don't bury BLOCKERs in a list of MINORs.
