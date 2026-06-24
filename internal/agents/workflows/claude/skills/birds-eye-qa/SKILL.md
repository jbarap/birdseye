---
name: birds-eye-qa
description: Run a QA pass on a coding agent's work using bird's-eye's `be agents` verbs — spawn a dedicated reviewer agent on the same branch's worktree (or a fresh one), have it run tests/lint/build and review the diff, and report findings back. Use when the user wants an independent quality check of work an agent produced.
---

# bird's-eye QA workflow

You run an **independent** quality pass on work produced in a git worktree, using a
separate agent so the reviewer's context is clean. Composed entirely from bird's-eye's
stable headless verbs.

## The verbs you compose

Address work by handle: `<repo>`, `<repo>/<worktree>`, or a tmux pane id (`%37`).

```
be agents list --json                       # find the worktree under review and its status
be agents status <handle> --json            # confirm a worker is idle/done before reviewing
be agents spawn <dir> --branch <b> --prompt <text>   # spawn the QA reviewer; prints its handle
be agents send <handle> <input...>          # hand the reviewer follow-up checks
be agents close <handle>                      # retire the reviewer's window, keep the worktree
be agents delete <handle> [--force]          # remove the reviewer's worktree when done
```

## Workflow

1. **Locate the work.** Run `be agents list --json`. Identify the worktree to QA by its
   handle and confirm its `status` is `idle` or `done` (don't review a `working` agent
   mid-turn — wait or ask the user).

2. **Spawn the reviewer.** Create a dedicated QA agent on its own worktree branched off the
   work under review, with a precise checklist prompt:

   ```
   be agents spawn /path/to/repo --branch qa/fix-login --prompt "QA the changes on branch fix-login. Run the full test suite, the linter, and a build. Review the diff against the base branch for correctness, missing edge cases, and style. Report PASS/FAIL with specifics; do not modify code unless asked."
   ```

   Capture the printed handle (e.g. `repo/qa-fix-login`).

3. **Collect the verdict.** Poll `be agents status <qa-handle> --json` until it is
   `idle`/`done`, then read the reviewer's output (jump in with `be agents jump <handle>`
   if you need to read the full transcript, or have the prompt instruct it to write a
   summary). Relay a clear **PASS/FAIL with the specific findings** to the user.

4. **Iterate or close.** If QA found issues, either send them back to the original worker
   (`be agents send <work-handle> "QA found: …"`) or, on the user's instruction, have the
   QA agent fix them. When the pass is complete:
   - `be agents close <qa-handle>` to free the window but keep the worktree, or
   - `be agents delete <qa-handle>` to remove the QA worktree entirely (confirm with the
     user if it reports uncommitted changes before `--force`).

## Rules

- **Independence.** QA runs in its **own** worktree/agent so its review is not colored by
  the author's in-progress context.
- **Read-only by default.** The reviewer reports findings; it changes code only on explicit
  instruction.
- **Never `--force` delete without confirmation.** Surface uncommitted changes to the user
  first.
- **Stable verbs only** — `be agents …` and `be sessions --json`; no direct tmux/git.
