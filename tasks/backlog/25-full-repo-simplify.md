# Task 25: Full repository overview and simplification

**Phase**: 4 — Polish
**Blocked by**: #17, #18, #24
**Blocks**: (none)

## Objective

Run a comprehensive `/simplify` review across the entire repository after all feature work is merged. This is a full-codebase pass covering code reuse, quality, and efficiency.

## Scope

Review every Go source file in the repository:
- `cmd/slack-cli/`
- `internal/api/`
- `internal/auth/`
- `internal/cache/`
- `internal/channels/`
- `internal/commands/`
- `internal/config/`
- `internal/formatting/`
- `internal/output/`
- `internal/users/`
- `tests/`

## Review checklist

1. **Code reuse**: duplicated logic, helpers that should be shared, patterns that diverged across commands
2. **Code quality**: redundant state, parameter sprawl, copy-paste with variation, leaky abstractions, stringly-typed code
3. **Efficiency**: unnecessary work, missed concurrency, hot-path bloat, memory issues, overly broad operations

## Acceptance criteria

- [ ] `/simplify` run completed with all three review agents
- [ ] All identified issues fixed or explicitly noted as non-issues
- [ ] All tests pass after fixes
- [ ] golangci-lint clean
