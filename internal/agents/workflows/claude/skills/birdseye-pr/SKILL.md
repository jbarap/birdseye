---
name: birdseye-pr
description: Take a coding agent's finished worktree to a pull request using birdseye's `be agents` verbs — confirm the work is done, have the agent commit and push its branch and open a PR, then close or delete the worktree. Use when an agent's work is ready to ship and the user wants it turned into a PR.
---

# birdseye PR workflow

You take finished work in a git worktree through to an opened pull request, then tear the
worktree down cleanly. Composed from birdseye's stable headless verbs; the actual
commit/push/PR is performed by the agent in the worktree (which has the repo, git, and
`gh` in its shell), driven by the prompts you send.

## The verbs you compose

Address work by handle: `<repo>`, `<repo>/<worktree>`, or a tmux pane id (`%37`).

```
be agents list --json                       # find the finished worktree and its branch
be agents status <handle> --json            # confirm the worker is idle/done (not mid-turn)
be agents send <handle> <input...>          # drive commit / push / gh pr create
be agents jump <handle>                      # attach to read its output or handle a prompt
be agents close <handle>                      # close the window, keep the worktree
be agents delete <handle> [--force]          # remove the worktree once the PR is up and merged/abandoned
```

## Workflow

1. **Confirm done.** `be agents status <handle> --json` — the work must be `idle`/`done`,
   not `working`. Note its `branch` from the record; that is the PR's head branch.

2. **Stage and commit.** Send the agent through committing its work:

   ```
   be agents send repo/fix-login "Stage all changes, write a clear commit message describing the fix, and commit."
   ```

   Poll `status` until `idle`/`done` to confirm the commit landed before moving on (`send`
   has no read receipt).

3. **Push and open the PR.** Have the agent push its branch and open the PR with `gh`:

   ```
   be agents send repo/fix-login "Push this branch to origin and open a pull request against the default branch with gh. Summarize what changed and why in the PR body. Print the PR URL."
   ```

   If the agent hits an auth or permission prompt (its status goes to `attn`), surface it
   to the user or `be agents jump <handle>` to handle it interactively.

4. **Report.** Read back the PR URL from the agent's output and give it to the user.

5. **Tear down.** Once the PR is open:
   - `be agents close <handle>` to free the tmux window while keeping the branch's worktree
     on disk (reversible — re-open later if the PR needs changes), **or**
   - `be agents delete <handle>` to remove the worktree entirely. Only do this when the
     user confirms the branch is fully pushed; if it reports uncommitted changes, relay
     that and pass `--force` only on explicit confirmation. The base (`<repo>`) is never
     deletable.

## Rules

- **The branch must be pushed before deleting its worktree.** Closing is always safe;
  deleting is not — never drop a worktree with unpushed commits without the user's
  explicit go-ahead.
- **Drive, don't assume.** After every `send`, confirm via `status` that the step
  completed before issuing the next.
- **Stable verbs only** — `be agents …` and `be sessions --json`; no direct tmux/git.
