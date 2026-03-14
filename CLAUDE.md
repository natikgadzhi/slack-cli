@README.md

# Multi-agent Work Environment

## How It Works

1. The user edits `PROJECT_PROMPT.md` or `README.md` or other documentation with their project description
2. The lead agent (you, when running in the root of this repo) reads it
3. You decompose the project into a phased task plan
4. You create a team and spawn worker agents to execute tasks in parallel
5. Workers build code, write tests, and commit their work
6. You coordinate, review, and handle integration

## Lead Agent Behavior

When the user says **"build"**, **"go"**, **"start"**, or **"launch"**:

1. **Read** `README.md` and other top level docs to understand the project
2. **Plan** — Break the project into phases and tasks:
   - **Phase 0: Bootstrap** — Project scaffolding, directory structure, config files, dependency setup
   - **Phase 1: Core** — Core data models, types, interfaces, database schema
   - **Phase 2: Implementation** — Feature implementation (parallelize heavily here)
   - **Phase 3: Integration** — Wire components together, integration tests
   - **Phase 4: Polish** — Error handling, edge cases, documentation, final tests
3. **Create a team** using `TeamCreate` with name based on the project
4. **Create tasks** using `TaskCreate` for each work item, with:
   - Clear subject (imperative: "Implement user authentication endpoint")
   - Detailed description with acceptance criteria, file paths, and dependencies
   - `activeForm` in present continuous ("Implementing user authentication")
   - Dependency chains via `addBlockedBy` (Phase 1 tasks block on Phase 0, etc.)
5. **Spawn workers** using `Task` tool with `subagent_type: "general-purpose"`, `team_name`, and `name`
   - Spawn 3-6 workers depending on project complexity
   - Name them descriptively: `builder-1`, `builder-2`, `reviewer`, `tester`, etc.
   - Generally prefer reviewers to have larger context and bigger models.
   - Keep the main checkout on `main` branch and `git pull --ff` after each task is completed and merged. Apply whatever migrations and changes required, so the user can see progress.
6. **Assign tasks** to idle workers as they become available
7. **Track task file state** — move task files between directories to reflect their status:
   - When a worker claims a task: `mv tasks/backlog/<task>.md tasks/in-progress/`
   - When a task's PR is merged and verified: `mv tasks/in-progress/<task>.md tasks/done/`
   - This keeps the file system in sync with actual task status at all times
8. **Monitor progress** — when workers complete tasks, assign them the next unblocked task
9. **Handle conflicts** — if workers produce conflicting changes, resolve them.
10. **Shutdown** workers when all tasks are complete

## Worker Agent Instructions

Workers receive tasks via the team task system. Each worker MUST:

1. **Read the task** with `TaskGet` to understand full requirements
2. **Mark it in-progress** with `TaskUpdate`
3. **Create a git worktree** — ALWAYS use a worktree, never work in the main checkout:

   ```bash
   git worktree add ../strava-cleaner-task-N -b task-N-description
   cd ../strava-cleaner-task-N
   ```

   - Each task gets its own worktree AND its own branch
   - Branch off the latest `main` (do `git fetch origin && git pull --ff` in the main checkout first)
   - Work exclusively inside the worktree directory, not the main checkout

4. **Read existing code** before writing — understand the current state
5. **Implement the task** — write code, tests, configs as needed
6. **Verify the work** — run tests, linters, type checks if available
7. **Commit the work** with a descriptive message (include task ID)
8. **Push and create a pull request** — this is MANDATORY for every task with code changes:

   ```bash
   git push -u origin task-N-description
   gh pr create --title "type(feat/doc/chore/ref) [task-N] description" --body "..."
   ```

   - Every code change goes through a PR. No direct commits to `main`.
   - PR title format: `type(feat / doc / chore / ref) [task-N] <description>`

9. **Tell the lead to assign reviewer** via `TaskUpdate`. If you are assigned a review, post comments and add updates to the task file, then push a `TaskUpdate`.
10. **Respond to review comments and clean things up, then merge the PR**
11. **Clean up the worktree** after merge:
    ```bash
    cd /path/to/main/checkout
    git worktree remove ../strava-cleaner-task-N
    ```
12. **Mark the task completed** with `TaskUpdate`
13. **Message the lead** via `SendMessage` with a summary of what was done
14. **Check TaskList** for the next available task

## Reviewer Agent Instructions

Reviewers are assigned to PRs by the lead agent. When assigned a review, the reviewer MUST:

1. **Check out the PR branch** in a worktree:
   ```bash
   git worktree add ../strava-cleaner-review-N origin/task-N-description
   cd ../strava-cleaner-review-N
   pnpm install
   ```
2. **Check for conflicts** — ensure the branch rebases cleanly on latest `main` and doesn't conflict with other in-progress worker branches. If there are conflicts, flag them and request the worker to rebase.
3. **Run all quality checks**:
   - `pnpm lint` — no lint errors
   - `pnpm check` — TypeScript types pass
   - `pnpm build` — production build succeeds
   - `pnpm test` — unit tests pass (including new tests for the feature)
   - `pnpm test:e2e` — Playwright tests pass (if configured)
4. **Verify the feature works** — read the task description and acceptance criteria, then confirm the implementation satisfies each criterion. If the app needs to be running, use `docker compose up` and test manually via curl or browser.
5. **Code quality review**:
   - Read every changed file in the diff (`gh pr diff`)
   - Check for dead code, unnecessary complexity, or missing error handling
   - Verify naming conventions and code style match the rest of the codebase
   - Ensure no commented-out code was left behind
   - Check that new code follows existing patterns (Svelte 5 runes, Drizzle queries, etc.)
6. **Security review**:
   - Check for injection vulnerabilities (SQL injection, XSS, command injection)
   - Verify authentication/authorization is enforced on new endpoints
   - Ensure secrets and tokens are not logged or exposed
   - Check that user input is validated at API boundaries
   - Flag any use of `eval`, `innerHTML`, or raw SQL without parameterization
7. **Post a PR review comment** via `gh pr review` with:
   - Summary of what was reviewed
   - Edge cases that ARE covered by the implementation
   - Edge cases that are NOT covered (and whether they should be addressed now or noted for future work)
   - Any security concerns
   - Approve, request changes, or comment accordingly
8. **Clean up the worktree** after review:
   ```bash
   cd /path/to/main/checkout
   git worktree remove ../strava-cleaner-review-N
   ```
9. **Update the task** via `TaskUpdate` and message the lead with the review outcome

## Git Conventions

- The project checkout directory stays on `main` branch. Do not switch branches here — each worker agent should have it's own git worktree. If you something did switch branches, lead agent switches back to `main` and does `git pull --ff` when we are done.
- Do `git pull --ff` after every task is completed and merged.
- Each task branch should be based on the latest `main`.
- Each task that requires code changes should push a pull request that gets reviewed and then merged.
- Workers use git worktrees for isolation for each branch.
- Workers commit with messages: `[task-N] <description>`
- One logical change per commit — don't bundle unrelated work
- Workers should `git pull --rebase` before committing to avoid conflicts
- If a rebase has conflicts, the worker should resolve them or ask the lead for help

## File Organization

```
.
├── README.md      # <-- User edits this (the project spec)
├── CLAUDE.md              # <-- You're reading this (agent instructions)
├── tasks/                 # <-- File-based task tracking (for distributed mode)
│   ├── backlog/           #     Unclaimed tasks
│   ├── in-progress/       #     Claimed and active
│   └── done/              #     Completed tasks
├── docs/                  # <-- Generated progress logs
│   └── progress.md        #     Auto-updated by lead agent
├── scripts/               # <-- Fleet management scripts
│   ├── launch-fleet.sh    #     Launch N distributed workers
│   ├── agent-worker.sh    #     Individual worker loop
│   └── status.sh          #     Fleet status dashboard
└── src/                   # <-- Generated project code goes here
```

## Task File Format (for distributed/file-based mode)

When using the shell scripts (distributed mode), tasks are tracked as files:

```
tasks/backlog/001-bootstrap-project.md
tasks/in-progress/002-implement-auth.md  (contains "CLAIMED_BY: agent-xyz")
tasks/done/001-bootstrap-project.md
```

Each task file contains:

```markdown
# Task: <title>

Priority: <high|medium|low>
Depends-On: <task-ids or "none">
Claimed-By: <agent-id or "unclaimed">

## Description

<what to build>

## Acceptance Criteria

- [ ] criterion 1
- [ ] criterion 2

## Files to Create/Modify

- path/to/file.ext

## Notes

<implementation hints>
```

## Important Rules

- **Never modify PROJECT_PROMPT.md** — that's the user's spec
- **Always read before writing** — understand existing code before changing it
- **Test everything** — if a test framework is specified, write tests. If no test framework is specified, or it is ambiguous what type of tests to write for a given task, ask the user what to do
- **Small, focused tasks** — each task should be completable in one agent session.
- **Explicit dependencies** — if task B needs task A's output, declare it
- **No premature abstraction** — build what's needed, not what might be needed
- **Commit early and often** — small atomic commits, not monolithic ones
- **Always verify tasks with end to end tests** - we care about business impact and outcomes and business logic behavior being correct, not about code just being there
