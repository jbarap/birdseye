---
name: birdseye-orchestrator
description: Orchestrate a fleet of parallel coding agents over git worktrees using birdseye's `be agents` verbs — spawn workers on isolated worktrees, poll their status, steer them with follow-up prompts, and tear them down. Use when the user wants to fan a task out across several agents working in parallel, or to supervise agents already running.
---

# birdseye orchestrator

You drive a fleet of worker agents through birdseye's headless verbs. Each worker runs
in its own git worktree (a sibling directory) under its own tmux window, so workers never
collide on the working tree. You spawn them, watch their status, send them follow-ups, and
close or delete them when done. **birdseye is substrate, not orchestrator** — the
opinion lives here, in this skill, composed entirely from stable verbs.

## The verbs you compose

All state is derived fresh from git + tmux on every call; there is no database to keep in
sync. Address a unit of work by its **handle**: `<repo>` (the primary worktree / base),
`<repo>/<worktree>` (a linked worktree), or a tmux pane id like `%37` (an agent with no
worktree).

```
be agents list --json                       # the whole fleet: handle, kind, status, branch, tmux location
be agents status <handle> --json            # one work item
be agents spawn <dir> --branch <b> [--prompt <text>]   # new sibling worktree + agent; prints the new handle
be agents send <handle> <input...>          # type a line into the agent (best-effort; not confirmed received)
be agents jump <handle>                      # attach your tmux client to that work (interactive; rarely needed here)
be agents close <handle>                      # close the tmux window, keep the worktree on disk (reversible)
be agents delete <handle> [--force]          # close the window AND git-worktree-remove (destructive; --force if dirty)
be sessions --json                            # enumerate launch targets (repos/dirs) you can spawn into
```

`--json` output is a stable contract; parse it, don't scrape the human table.

## Workflow

1. **Pick the repo.** If the user named a directory, use it. Otherwise run
   `be sessions --json` and choose the repo directory the user means (the `dir` field of a
   `create` candidate, or an existing managed repo from `be agents list --json`).

2. **Fan out.** For each parallel task, spawn a worker on a fresh branch:

   ```
   be agents spawn /path/to/repo --branch fix-login --prompt "Fix the login redirect bug. Run the tests."
   ```

   Capture the printed handle (e.g. `repo/fix-login`). Give each worker a distinct,
   descriptive branch so the handles read well. Spawn the next without waiting — they run
   in parallel.

3. **Poll.** Loop `be agents list --json` (or `status <handle> --json` per worker) and read
   the `status` field. Statuses surface from the agent's own hooks:
   - `working` — busy; leave it.
   - `idle` / `done` — finished a turn; ready for review or a follow-up.
   - `attn` (needs attention) — waiting on input or a permission prompt; act on it.
   Re-poll on an interval rather than busy-looping; report a concise fleet summary to the
   user between polls.

4. **Steer.** When a worker needs direction, send a follow-up:

   ```
   be agents send repo/fix-login "Also add a regression test, then commit."
   ```

   `send` is best-effort with no read receipt — after sending, confirm via the next
   `status` poll that the worker picked it up, don't assume.

5. **Tear down.** When a worker's work is captured (committed/pushed/PR-opened):
   - `be agents close <handle>` to free the tmux window but **keep** the worktree (it
     becomes a reusable slot) — reversible, no disk loss.
   - `be agents delete <handle>` to also remove the git worktree once you are sure nothing
     unmerged remains. If it reports uncommitted changes, surface that to the user and only
     pass `--force` on explicit confirmation. The base (`<repo>`) is never deletable.

## Rules

- **Never guess a destructive action.** `delete` removes a worktree; confirm with the user
  before `--force`. `close` is safe (reversible) and needs no confirmation.
- **One worker per branch/worktree.** Don't spawn two agents into the same branch.
- **Report, don't flood.** Summarize fleet status for the user; don't paste raw JSON unless
  asked.
- **Compose only stable verbs.** Everything here is `be agents …` and `be sessions --json`;
  do not reach into tmux or git directly when a verb exists.
- If a handle is ambiguous (two repos sharing a basename), the verb refuses with the
  disambiguating paths — relay that to the user and use the full path the user intends.
