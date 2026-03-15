# Task 15: Remove Python source and config

**Phase**: 4 — Polish
**Blocked by**: #14
**Blocks**: #18

## Objective

Clean up the repository by removing all Python artifacts now that the Go rewrite is verified.

## Files to remove

- [ ] `src/` — entire directory (all Python source)
- [ ] `tests/` — entire directory (Python tests)
- [ ] `pyproject.toml`
- [ ] `uv.lock`
- [ ] `.python-version`
- [ ] `.pytest_cache/` — entire directory

## Files to review (keep or remove)

- `slack-sharable.sh` — keep if still useful as a reference, otherwise remove
- `slack-tokens.md` — keep if still useful as auth documentation, otherwise remove

## Files to update

- [ ] `.gitignore` — remove Python patterns (*.pyc, __pycache__, .venv, etc.), ensure Go patterns present (binary name, vendor/, etc.)
- [ ] `Makefile` — remove Python targets (was already replaced in Task #1, verify)

## Acceptance criteria

- [ ] No Python files remain in the repo (except maybe docs)
- [ ] `go build ./...` still works
- [ ] `go test ./...` still passes
- [ ] `.gitignore` is clean and appropriate for a Go project
- [ ] Commit message clearly describes what was removed and why
