## Context

Every agent lifecycle operation today lives inside the TUI. `internal/cli/agents.go`
wires an `orchestrator` (tmux + worktree primitives) and a `Workspace` reconciler into a
Bubble Tea model; the operations — `NewSession`, `Spawn`, `Remove` — are in-process Go
methods reachable only by a human pressing keys. There is no way for a script, or another
agent, to spawn work, poll status, or send feedback.

bird's-eye's stated direction is "the CLI is the API": clients are peers, and no surface
holds private powers. An *orchestration agent* — one whose job is to coordinate other
agents — is just another client, so the same lifecycle operations must exist as headless,
composable shell verbs. This change exposes them under `be agents`, moves the TUI to
`be dash`, and makes both faces resolve to one shared set of operations.

It depends on `worktree-sibling-layout`: git-native recognition gives every unit of work
a durable identity (its git worktree, keyed by the shared git dir), which is what makes a
stable text handle possible.

## Goals / Non-Goals

**Goals:**

- Headless `be agents` verbs — `list`, `status`, `spawn`, `send`, `jump`, `close`,
  `delete` — emitting `--json` where they return data.
- A durable, derived **addressing scheme** so any agent (even one bird's-eye did not
  spawn) is nameable without persisted state.
- `spawn` that takes a **directory** (any repo on disk) and is independently usable: it
  ensures-or-reuses the repo's session, makes a sibling worktree, opens a window, and
  starts the agent.
- One shared operation layer behind both the verbs and the TUI (lifecycle parity).
- A headless discovery face — `be sessions --json` — over the *same* provider registry
  fzf already drives.

**Non-Goals:**

- `watch`/streaming. Clients poll `list`/`status`. No long-lived event stream here.
- Solving `send` delivery semantics. tmux `send-keys` has no readiness signal; `send` is
  best-effort and documented as such.
- The `dd`/`dD` key UI — `close`/`delete` define the *operations*; the keys that invoke
  them in the TUI are specified in `dash-teardown-keys`.
- The install/workflow tooling that composes these verbs — `agents-install-workflows`.

## Decisions

### One operation layer, two faces

The lifecycle operations move behind a surface both the verbs and `be dash` call. The
TUI's `orchestrator` methods and the verbs are thin adapters over the same functions, so
a `close` from the CLI and a `dd` in the dash are the *same* operation.

- *Why:* "no face holds private powers." Divergence between what a human can do and what a
  script can do is the exact failure this change exists to remove.
- *Alternative considered:* let the CLI shell out to drive the TUI. Rejected — fragile,
  and inverts the dependency (the API should not route through the UI).

### Addressing by durability: `repo/worktree` handles

A unit of work is addressed by a **derived** text handle, never a stored id:

- `<repo>` alone addresses a repository's **primary worktree** (its anchor): `<repo>` is
  the basename of the primary worktree's directory (the clone dir).
- `<repo>/<worktree>` addresses a linked worktree, where `<worktree>` is that worktree
  directory's basename (the branch-slug under `<repo>.worktrees/`).
- A live/observed agent with **no worktree** (an imported or incidental agent) is
  addressed by its **tmux pane id** (`%id`), which survives renames.

Handles are *computed* from git (`git-common-dir`, `git worktree list`) and tmux on every
invocation, so an agent bird's-eye never spawned is addressable automatically — there is
no `import` verb. When two repos in the active set share a basename, the handle is
ambiguous; the verb errors and lists the disambiguating paths (mirroring the existing
"Candidate name uniqueness for resolution" provider rule) rather than guessing.

- *Why durability layers:* work outlives processes and tmux servers but is pinned by its
  git worktree; live objects (windows/panes) are pinned by tmux's own stable ids; the
  agent process is the most ephemeral and is resolved at call time. Addressing each thing
  at its real durability is what lets handles stay stable across renames, restarts, and
  respawns.
- *Alternative considered:* bird's-eye-assigned UUIDs persisted to disk. Rejected — it
  reintroduces owned state, defeats "import for free," and drifts from reality the moment
  a user renames or moves something.

### `spawn` takes a directory and is self-sufficient

`be agents spawn <dir> [--branch B] [--prompt P]` resolves `<dir>` to its repository
(shared git dir), ensures-or-reuses a tmux session named for the repo, creates a sibling
worktree for the branch, opens a window rooted there, starts the configured agent
(optionally seeding `--prompt`), and prints the new handle.

- *Why a directory, not a picker candidate:* `spawn` must work on any repo on disk
  without pre-existing tmux state, so an orchestrator can point it at a freshly cloned
  repo. It is a primitive, not tied to the session lister.
- *Reuse:* if the repo's session already exists it is reused, not duplicated (the tmux
  backend's idempotent-creation rule).
- *Alternative considered:* `spawn` consumes a `be sessions --json` candidate. Rejected as
  the *only* form — that couples spawn to discovery; a directory arg is the primitive,
  and discovery feeding it is an optional composition.

### Discovery is the same registry, headless

`be sessions --json` lists launch candidates (repos/dirs/templates/worktrees) non-
interactively by serializing the provider registry's candidates. fzf remains the
interactive face of the same registry.

- *Why:* "one observable world" — the interactive and headless faces enumerate identical
  candidates; nothing is duplicated. An orchestrator discovers with `--json`, a human
  with fzf.

### Stable JSON contract

Data-returning verbs emit a JSON array (or object) of records with stable field names:
`handle`, `repo`, `worktree`, `branch`, `path`, `session` (`{id, name}`), `window`,
`pane`, `status`, and `kind` (anchor / worktree / slot / agent). `--json` selects machine
output; without it the verb prints a human-readable table.

- *Why:* a documented, additive-only field set is the actual API contract another agent
  codes against. tmux ids are included so a client can drop to raw tmux when it wants.

### `close` vs `delete` parity

`be agents close <handle>` closes the window and keeps the worktree (≡ `dd`).
`be agents delete <handle>` closes the window *and* removes the git worktree (≡ `dD`),
escalating on a dirty worktree (a `--force` flag replaces the TUI's force confirmation).
A primary-worktree handle refuses `delete` structurally (git won't remove it).

- *Why:* identical vocabulary across surfaces; the headless `--force` is the non-
  interactive analog of the dash's force confirmation.

## Risks / Trade-offs

- **`send` is best-effort.** [tmux `send-keys` has no readiness signal, so a prompt sent
  before the agent's REPL is ready can be lost] → documented sharp edge; `send` reports
  what it dispatched, not what was received. Clients that need confirmation poll
  `status`.
- **Handle ambiguity on basename collision.** [Two repos named `api` under different
  roots collide] → the verb errors with the candidate paths instead of acting on the
  wrong one; never silently picks.
- **BREAKING: bare `be` is dropped.** [Scripts and muscle memory invoking bare `be` break]
  → required; `be dash` is the TUI and a clear "subcommand required" error names it.
  Documented in README and the proposal as breaking.
- **JSON contract becomes load-bearing.** [Clients couple to field names] → mitigated by
  declaring the contract additive-only (new fields may appear; existing ones are stable).

## Migration Plan

1. Extract the lifecycle operations from the TUI `orchestrator` into a shared package both
   the verbs and `be dash` call.
2. Add handle resolution (`repo/worktree` ↔ git worktree / tmux object) on top of the
   `worktree-sibling-layout` git-native recognition.
3. Add the `be agents` verb tree (`list`/`status`/`spawn`/`send`/`jump`/`close`/`delete`)
   with `--json`.
4. Add `--json` to `be sessions` (serialize registry candidates).
5. Restructure the root command: drop bare `be`, add `be dash` (the current `runAgents`),
   repoint `be agents` from the TUI to the verb namespace.
6. Update README/DESIGN: `be dash` for the TUI, the verb reference, the handle scheme.

Rollback is reverting the release; no persisted state.

## Open Questions

- Should `be agents` with no subcommand print verb help, or be a friendly alias that
  suggests `be dash`? (Leaning: print help; bare `be` already errors.)
- `send` input ergonomics: positional text vs `--message` vs stdin (`be agents send
  <handle> -` reading stdin) for multi-line prompts — pick during apply.
- Whether `status` is just `list` filtered to one handle, or a distinct richer record
  (e.g. last-event timestamp). Leaning: `status <handle>` = single-record `list`.
