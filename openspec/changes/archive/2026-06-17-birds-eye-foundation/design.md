## Context

Bird's-eye (`be`) is a greenfield Go CLI that sits on top of tmux to give a single,
fuzzy-searchable view over session *candidates* — both running sessions and ways to
create new ones — plus a triage view over AI agents and a git-worktree manager. The user
already runs neovim + ghostty + tmux + tmuxp + Claude Code, loves `fzf`, and values a
lightweight, hackable, extensible tool over a heavyweight framework. tmux is the
deliberate backend: battle-tested and ubiquitous. The hard design questions are how to
keep providers pluggable, how to reconcile "I love fzf" with "Go has great TUI tooling,"
and how to detect agent status reliably without a heavy daemon.

The proposal defines six capabilities: `be-cli`, `session-picker`, `session-providers`,
`tmux-backend`, `agent-view`, `worktree-provider`. This document covers how they fit
together; the spec files define the per-capability requirements.

## Goals / Non-Goals

**Goals:**
- One portable, self-contained binary with no runtime interpreter dependency.
- A small, stable `Provider`/`Candidate` interface that new session sources implement
  without touching picker, tmux, or CLI core code.
- A picker that feels like `fzf` and a separate, richer agent view.
- Graceful degradation: missing `tmuxp`/`zoxide`/`git`/`tmux` disables features with a
  non-fatal warning rather than crashing.
- Agent status triage that works today against Claude Code with room to grow.

**Non-Goals:**
- A long-running daemon or background service (this iteration is invocation-driven).
- Full agent orchestration (spawning/steering agents, multi-agent workflows) — only a
  read-only status/triage view now.
- Replacing tmux, tmuxp, or zoxide; `be` orchestrates them, it does not reimplement them.
- A plugin ABI for out-of-process/third-party binaries — extensions are in-tree Go
  providers for this iteration (see Open Questions).
- Windows-first support; target Linux/macOS where tmux runs.

## Decisions

### Language & layout: Go, standard `cmd/` + `internal/`
`cmd/be` holds the entrypoint and command wiring; `internal/` holds `config`, `provider`
(registry + interface), `providers/*` (tmux, tmuxp, dir/zoxide, worktree), `tmux`
(backend), `picker`, and `agents`. Rationale: matches the user's stated reasons (readable,
portable static builds, strong TUI ecosystem) and keeps capabilities as separate packages
so they can evolve independently. *Alternative considered:* a flat single-package layout —
rejected because it would erode the provider seam the project exists to protect.

### CLI framework: Cobra
Use Cobra for command routing (`be`, `be agents`, `be worktree`, `--version`). Rationale:
ubiquitous, gives subcommands/flags/help for free, and `be` with no args maps cleanly to
the default `list` command. *Alternative considered:* hand-rolled flag parsing — fine for
3 commands but pays off poorly as subcommands grow; the dependency is cheap and static.

### Picker: shell out to `fzf` (hard requirement)
The picker pipes labeled candidates into `fzf` (with a configurable prompt and a preview
command) and reads the selection back. `fzf` is a hard requirement — there is no native
fallback; if `fzf` is absent the picker reports it and exits non-zero. Rationale: the user
explicitly loves `fzf` and wants this kept simple; depending on it directly is the fastest
path to a battle-tested, familiar fuzzy UI and the least code to maintain. *Alternative
considered:* build the picker in Bubble Tea (or keep a Bubble Tea fallback behind a `Picker`
interface) — rejected as unnecessary complexity for a tool whose owner always has `fzf`.
Bubble Tea is reserved for the richer `be agents` view only.

### Agent view: Bubble Tea + Lipgloss
`be agents` renders with Bubble Tea/Lipgloss rather than `fzf`. Rationale: it needs grouped
rows, status colors, ordering by attention, and (later) live updates — beyond what a flat
`fzf` list expresses well. It must run cleanly inside `tmux display-popup -E be agents` and
exit on dismiss. *Alternative considered:* also use `fzf` — rejected because status
rendering and future live refresh are first-class needs here.

### Provider/Candidate model
`Provider.Candidates(ctx) ([]Candidate, error)`; each `Candidate` carries `Name` (the
canonical tmux session name), `Label` (display text), `Type` (source tag for grouping),
and an `Action` that is either *attach existing* or *create* (a closure/struct that, given
the tmux backend, materializes the session). The registry queries all enabled providers,
isolates per-provider failures (skip + warn), and deduplicates candidates that resolve to
the same session `Name` with a deterministic precedence (existing-session beats creator).
Rationale: a tiny surface keeps new providers trivial to write and makes the picker
provider-agnostic.

### tmux backend: wrap the `tmux` CLI, idempotent by name
A `tmux` package shells out to the `tmux` binary: `has-session` to check existence,
`new-session -d` to create, then `attach-session` vs `switch-client` chosen by inspecting
`$TMUX` (set ⇒ inside tmux ⇒ switch). Rationale: shelling out avoids brittle libtmux-style
bindings and matches how tmux is actually driven; `$TMUX` is the canonical inside/outside
signal. Errors bubble up with tmux stderr attached.

### Config: TOML at `~/.config/birds-eye/config.toml` (XDG)
TOML via a standard decoder, located through the platform config dir. Enables/disables
providers, sets type ordering/labels, and per-provider options (tmuxp dir, zoxide on/off,
directory roots, managed-repos root). Built-in defaults apply when absent; malformed
config is a hard, clearly-reported error. Rationale: TOML is comment-friendly and readable
for a hackable tool; XDG keeps it predictable. *Alternative considered:* YAML — heavier
and whitespace-fragile for hand editing.

### Agents: an `Agent` abstraction, Claude implemented via hooks, no daemon
`be` defines an `Agent` abstraction (interface) that yields the agents it knows about,
each with a location (tmux session/window) and a status from a small, well-defined enum
(needs-attention / working / idle / done). The first implementation is Claude Code, and it
gets its status from **Claude Code hooks** rather than from scraping panes: hooks (e.g.
session start/stop, notification/needs-input, post-response) write per-session state to a
known location (e.g. a status file/dir under the config or state dir), and `be agents`
reads and renders that state. Rationale: hooks give authoritative, low-noise status that
pane/process heuristics cannot, and the abstraction keeps new agent types pluggable without
changing the `be agents` contract. `be` ships/loads a documented hook config so wiring it up
is one step. *Alternatives considered:* pane/process-tree heuristics (rejected — noisy,
guesses "done"); a background daemon tracking state (rejected as a Non-Goal — too heavy).

### Worktree manager: wrap `git worktree`, `<repo>/main` + siblings
`be worktree` clones into `<repo>/main` and creates siblings via `git worktree add
<repo>/<name>`. Managed repos live under a configured root and are surfaced through a
worktree provider so each `main`/worktree becomes a session candidate. Rationale: reuses
git's native worktree mechanism (no reinvention) and composes with the provider model so
worktrees are "just another session source."

## Risks / Trade-offs

- **Hard dependency on external `fzf` for the core UX** → Accepted by design (owner always
  has `fzf`); detect it at runtime and fail with a clear, actionable message and install
  hint rather than a confusing crash.
- **Agent status depends on Claude Code hooks being installed/firing** (stale or missing
  state) → Ship a documented hook config and an install step; treat absent/stale state as
  a distinct "unknown" rendering rather than guessing, and keep the status enum small and
  honest behind the `Agent` abstraction so other status sources can be added later.
- **Shelling out to many external tools is fragile across environments** → Centralize tool
  discovery (probe PATH once), degrade gracefully per provider, and attach the underlying
  stderr to every error for diagnosis.
- **Session-name collisions across providers** → Deterministic dedup precedence
  (existing-session wins) plus name-normalization rules defined in `session-providers`.
- **In-tree-only extensions limit third-party hacking now** → Acceptable for a lean first
  cut; revisit an out-of-process plugin protocol once the in-tree interface stabilizes
  (Open Questions).
- **Nested tmux confusion** → Always branch on `$TMUX` to switch-client rather than attach,
  preventing nested clients.

## Open Questions

- Should extensions eventually support out-of-process plugins (e.g. a documented exec/JSON
  protocol so non-Go tools can contribute candidates), or stay in-tree Go providers?
- Which exact Claude Code hook events map to which status (e.g. notification → needs-
  attention, stop → idle/done), and where should the hook write state (config dir vs an
  XDG state dir) so concurrent sessions don't clobber each other?
- Should `be agents` offer any actions beyond jump-to-session (e.g. kill, mark-read) in
  this iteration, or stay strictly read-only?
