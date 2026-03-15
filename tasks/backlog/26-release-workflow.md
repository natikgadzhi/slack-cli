# Task 26: Release workflow with version bump and changelog

**Phase**: 4 — Polish
**Blocked by**: #17
**Blocks**: (none)

## Objective

Add a `workflow_dispatch` GitHub Action that takes a version bump type, generates a changelog from merged PRs, creates a tag, and triggers the existing release pipeline (GoReleaser → binaries → Homebrew formula).

## Design

### Workflow: `.github/workflows/create-release.yml`

**Trigger**: `workflow_dispatch` with input:
- `bump`: choice of `major`, `minor`, `patch` (default: `patch`)

**Steps**:

1. **Checkout** with `fetch-depth: 0` and `fetch-tags: true`
2. **Get latest tag**: `git describe --tags --abbrev=0` → e.g. `v0.1.0`
   - If no tags exist, default to `v0.0.0`
3. **Compute next version**: Parse semver, increment the selected component
   - `v0.1.0` + `patch` → `v0.1.1`
   - `v0.1.0` + `minor` → `v0.2.0`
   - `v0.1.0` + `major` → `v1.0.0`
4. **Generate changelog**: Use `gh` CLI to list merged PRs since last release date:
   ```bash
   LAST_TAG_DATE=$(git log -1 --format=%aI $LAST_TAG)
   gh pr list --state merged --search "merged:>$LAST_TAG_DATE" \
     --json number,title,labels --template '...'
   ```
   Format as markdown:
   ```
   ## What's Changed
   - feat: Add --output-dir for markdown output (#23)
   - fix: Migrate to golangci-lint v2 (#17)
   - docs: Rewrite README for Go (#21)
   ```
5. **Create annotated tag**:
   ```bash
   git tag -a $NEXT_VERSION -m "$CHANGELOG"
   ```
6. **Push tag**: `git push origin $NEXT_VERSION`
   - This triggers the existing `release.yml` workflow
   - GoReleaser builds binaries, creates GitHub release, pushes Homebrew formula

### Permissions

```yaml
permissions:
  contents: write
```

Needs write access to push tags.

### Version computation (shell)

```bash
#!/bin/bash
LAST_TAG=${1:-v0.0.0}
BUMP=${2:-patch}

# Strip 'v' prefix, split into components
VERSION=${LAST_TAG#v}
IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"

case $BUMP in
  major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
  minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
  patch) PATCH=$((PATCH + 1)) ;;
esac

echo "v${MAJOR}.${MINOR}.${PATCH}"
```

## Acceptance criteria

- [ ] `workflow_dispatch` with `bump` input (major/minor/patch)
- [ ] Correctly computes next semver from latest tag
- [ ] Generates changelog from merged PRs since last release
- [ ] Creates annotated tag and pushes it
- [ ] Existing release.yml picks up the tag and runs GoReleaser
- [ ] End result: GitHub release with binaries + updated Homebrew formula
- [ ] Works for first release (no prior tags → defaults to v0.0.0)
- [ ] No secrets beyond the default GITHUB_TOKEN needed for this workflow
