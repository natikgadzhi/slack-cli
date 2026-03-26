@README.md

# Multi-agent Work Environment

## How It Works

1. The user edits `PROJECT_PROMPT.md` or `README.md` with their project description
2. The lead agent (you, when running in the root of this repo) reads it
3. You decompose the project into a phased task plan
4. You spawn worker agents via the `Agent` tool to execute tasks in parallel
5. Workers build code, write tests, and commit their work
6. You coordinate, review, and handle integration

## Lead Agent Behavior

When the user says **"build"**, **"go"**, **"start"**, or **"launch"**:

1. **Read** `README.md` and other top-level docs to understand the project
2. **Plan** — Break the project into phases and tasks:
   - **Phase 0: Bootstrap** — Project scaffolding, directory structure, config files, dependency setup
   - **Phase 1: Core** — Core data models, types, interfaces, database schema
   - **Phase 2: Implementation** — Feature implementation (parallelize heavily here)
   - **Phase 3: Integration** — Wire components together, integration tests
   - **Phase 4: Polish** — Error handling, edge cases, documentation, final tests
3. **Create tasks** using `TaskCreate` for each work item, with:
   - Clear subject (imperative: "Implement user authentication endpoint")
   - Detailed description with acceptance criteria, file paths, and dependencies
   - `activeForm` in present continuous ("Implementing user authentication")
   - Dependency chains via `addBlockedBy` (Phase 1 tasks block on Phase 0, etc.)
4. **Spawn workers** using the `Agent` tool with `subagent_type: "general-purpose"`
   - Spawn 3-6 workers depending on project complexity
   - Pass the task ID and full task details explicitly in each worker's `prompt`
   - Name them descriptively in the description: `builder-1`, `builder-2`, `reviewer`, `tester`, etc.
   - Generally prefer reviewers to have larger context and bigger models.
   - Keep the main checkout on `main` branch and `git pull --ff` after each task is completed and merged.
5. **Assign tasks** to idle workers as they become available
6. **Track task file state** — move task files between directories to reflect their status:
   - When a worker claims a task: `mv tasks/backlog/<task>.md tasks/in-progress/`
   - When a task's PR is merged and verified: `mv tasks/in-progress/<task>.md tasks/done/`
   - This keeps the file system in sync with actual task status at all times
7. **Monitor progress** — poll `TaskList`/`TaskGet` to track worker progress; spawn the next worker when one completes
8. **Handle conflicts** — if workers produce conflicting changes, resolve them
9. **Shut down** when all tasks are complete

## Worker Agent Instructions

Workers are spawned by the lead via the `Agent` tool. The lead passes the task ID and description in the prompt. Each worker MUST:

1. **Read the task** with `TaskGet <task-id>` to get full requirements
2. **Mark it in-progress** with `TaskUpdate`
3. **Create a git worktree** — ALWAYS use a worktree, never work in the main checkout:

   ```bash
   git fetch origin && git pull --ff   # in main checkout first
   git worktree add ../slack-cli-task-N -b task-N-description
   cd ../slack-cli-task-N
   ```

   - Each task gets its own worktree AND its own branch
   - Branch off the latest `main`
   - Work exclusively inside the worktree directory

4. **Read existing code** before writing — understand the current state
5. **Implement the task** — write code, tests, configs as needed
6. **Verify the work** — run the project's quality checks:
   - `go build ./...` — builds cleanly
   - `go vet ./...` — no vet errors
   - `golangci-lint run ./...` — no lint errors
   - `go test ./...` — all tests pass
7. **Commit the work** with a descriptive message (include task ID):
   - Format: `[task-N] <description>`
   - One logical change per commit
8. **Push and create a pull request** — MANDATORY for every task with code changes:

   ```bash
   git push -u origin task-N-description
   gh pr create --title "type(feat/doc/chore/ref) [task-N] description" --body "..."
   ```

   - Every code change goes through a PR. No direct commits to `main`.
9. **Update the task** via `TaskUpdate` with PR URL and status, so the lead knows to assign a reviewer
10. **Respond to review comments**, push fixes, then merge the PR
11. **Clean up the worktree** after merge:
    ```bash
    cd /path/to/main/checkout
    git worktree remove ../slack-cli-task-N
    ```
12. **Mark the task completed** with `TaskUpdate` (status: completed)

## Reviewer Agent Instructions

Reviewers are spawned by the lead with the PR number and task ID in their prompt. The reviewer MUST:

1. **Check out the PR branch** in a worktree:
   ```bash
   git worktree add ../slack-cli-review-N origin/task-N-description
   cd ../slack-cli-review-N
   ```
2. **Check for conflicts** — ensure the branch rebases cleanly on latest `main`. If conflicts exist, flag them and request the worker to rebase.
3. **Run all quality checks**:
   - `go build ./...` — builds cleanly
   - `go vet ./...` — no vet errors
   - `golangci-lint run ./...` — no lint errors
   - `go test ./...` — all tests pass (including new tests for the feature)
4. **Verify the feature works** — read the task description and acceptance criteria, confirm the implementation satisfies each criterion
5. **Code quality review**:
   - Read every changed file in the diff (`gh pr diff`)
   - Check for dead code, unnecessary complexity, or missing error handling
   - Verify naming conventions and code style match the rest of the codebase
   - Ensure no commented-out code was left behind
6. **Security review**:
   - Ensure secrets and tokens are not logged or exposed
   - Check that user input is validated at API/CLI boundaries
   - Flag any command injection risks (e.g. unsanitized input passed to shell)
7. **Post a PR review** via `gh pr review` with:
   - Summary of what was reviewed
   - Edge cases that ARE covered
   - Edge cases NOT covered (note whether they should be addressed now or later)
   - Any security concerns
   - Approve, request changes, or comment accordingly
8. **Clean up the worktree** after review:
   ```bash
   cd /path/to/main/checkout
   git worktree remove ../slack-cli-review-N
   ```
9. **Update the task** via `TaskUpdate` with the review outcome

## Git Conventions

- The main checkout stays on `main`. Never switch branches here — workers use worktrees.
- Do `git pull --ff` after every task is completed and merged.
- Each task branch is based off the latest `main`.
- Every code change goes through a PR — no direct commits to `main`.
- Workers commit with messages: `[task-N] <description>`
- One logical change per commit — don't bundle unrelated work
- Workers should `git pull --rebase` before pushing to avoid conflicts

## Task File System

Tasks are stored as markdown files in `tasks/` with three subdirectories representing status:

```
tasks/
├── backlog/      # Not yet started
├── in-progress/  # Currently being worked on
└── done/         # Completed and merged
```

- Each task is a numbered markdown file (e.g. `01-bootstrap-go-project.md`)
- The lead agent creates task files in `backlog/` during planning
- Workers move their task file to `in-progress/` when they start work
- Workers move their task file to `done/` after the PR is merged and the task is complete
- Task files contain the full specification: objective, acceptance criteria, dependencies, and notes
- Moving task files between directories should be committed as part of the worker's branch

## Important Rules

- **Never modify PROJECT_PROMPT.md** — that's the user's spec
- **Always read before writing** — understand existing code before changing it
- **Test everything** — write tests for every task. If ambiguous what kind, ask the user
- **Small, focused tasks** — each task should be completable in one agent session
- **Explicit dependencies** — if task B needs task A's output, declare it with `addBlockedBy`
- **No premature abstraction** — build what's needed, not what might be needed
- **Commit early and often** — small atomic commits, not monolithic ones
- **Always verify with end-to-end tests** — business logic correctness matters, not just code presence
