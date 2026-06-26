# birdseye design language

This is the visual contract for birdseye. Every surface — the picker, the agents view, CLI
chrome — follows it so the tool feels like one thing. All colors and visual glyphs come from the
`internal/theme` package; nothing hardcodes them (a guard test enforces this).

## Principles

1. **One accent for selection and brand.** A single magenta accent marks "the selected thing" and
   the tool's identity, everywhere. It never doubles as a status or type color.
2. **Legible without color.** Color is reinforcement, never the only signal. Status, selection, and
   grouping each carry a glyph or shape so the tool reads on a monochrome terminal.
3. **Fixed positions over density.** Data points sit at stable columns; the eye learns the layout
   once and scans. Truncate rather than reflow.
4. **One source of truth.** Colors and selection glyphs live in `theme`. Surfaces reference tokens,
   never literals.
5. **Recognize structure; don't own it.** birdseye is a lens over tmux and the filesystem, not a
   workspace that owns them. It lights up when it *recognizes* a convention and otherwise gets out
   of the way — an unrecognized session looks and behaves exactly as it does without the tool. See
   [Recognition over ownership](#recognition-over-ownership).

## Palette and semantics

Defined in `internal/theme`. Each color carries a 24-bit hex and a 256-color fallback.

| Token | Hex | Meaning |
|---|---|---|
| `Accent` | `#c792ea` | **Tool-wide selection + brand accent** (cursor, title, picker pointer/prompt/match). Configurable. |
| `Blue` | `#4ea8ff` | tmux/sessions type color; agents **idle** status |
| `Coral` | `#ff7a6b` | tmuxp template type color (and only that — no longer the primary accent) |
| `Gold` | `#f5c542` | dir type color; agents **working** status |
| `Green` | `#4ec98a` | repo / success; the picker "create" mark |
| `Red` | `#ff6b6b` | agents **needs-attention**; errors |
| `Gray` | `#8a8a8a` | dim / unlisted; agents **done** and **unknown** |
| `Border` | `#3a3a3a` | borders and info chrome |
| `Text` | `#e6e6e6` | primary row text (agent name) |
| `Attach` | `#38bdf8` | picker "attach" mark (`→`) |
| `SessionFg` / `SessionBg` | `#9cc7ff` / `#1d2b3f` | agents session section bars |
| `RowHL` | `#2c2c34` | agents selected-row highlight background |

The same hue can mean different things in different contexts (`Blue` is a session type in the
picker and the idle status in agents). That is intentional: **type** palettes and **status**
palettes are context-scoped. Only the `Accent` is cross-cutting.

## Selection

"The selected thing" looks the same on every surface:

- **Glyph:** `theme.CursorGlyph` — `󰁕` (a nerd-font arrow). It is the agents-view cursor and the
  picker's fzf `--pointer`.
- **Color:** `theme.Accent` (magenta).
- **Emphasis:** where a full row can be highlighted (agents view), the selected row also gets a
  `RowHL` background spanning its width, so the eye can ride from the left edge to the content.
- **Mirror:** in the dual-lens dash, the unfocused lens echoes the focused selection with
  `theme.MirrorGlyph` (`•`, a filled bullet) in the accent, and **without** the `RowHL`
  background. It is deliberately not the `CursorGlyph` arrow: only the lens you are in owns the
  pointer, so the echo reads as a passive locator, not a rival cursor. Focused and mirror stay
  distinct by glyph shape (arrow vs bullet) and the focused row's `RowHL` bar, not by color.

## Panels

Bounded content lives in a **panel**: a minimal rounded border in `theme.Border` with the panel's
title embedded in its **top border**, left-aligned near the corner and rendered in `theme.Accent`.
The title names what the panel holds — it is content-descriptive (`sessions`, `agents`, `preview`),
not branding. The tool's identity comes from the consistent chrome (accent, glyph, panels), not a
textual brand: the picker prompt is a bare chevron (`❯`). There is no separate floating title line
above a panel, and no faint inline title inside it.

The convention is one look with two implementations, because the picker is fzf-driven and fzf owns
its own screen region:

- **lipgloss surfaces** (agents list and preview) reconstruct the frame's top border with the title
  spliced in, reusing the same rounded-border runes the frame already draws.
- **the picker** asks fzf to draw it: `--border-label` (positioned near the top-left) plus the
  `label` `--color` key set to the accent. Where an older fzf lacks the feature, the panel degrades
  to a plain bordered box — title omitted, never an error.

## Status vocabulary (agents)

Each status is a glyph **plus** a short word in a fixed gutter, color-coded — never color alone:

| Status | Glyph + word | Color |
|---|---|---|
| needs-attention | `● attn` | `Red` |
| working | `◐ work` | `Gold` |
| idle | `○ idle` | `Blue` |
| done | `✓ done` | `Gray` |
| unknown | `· unkn` | `Gray` |

Needs-attention sorts first; the most-urgent group floats to the top.

In a [managed repo](#the-orthogonal-row-model) the same pinned gutter also carries two
structural **markers** (not statuses — they mark the absence/role of an agent), in `Gray`:

| Marker | Glyph + word | Meaning |
|---|---|---|
| base | `⌂ base` | the repo's primary worktree (git's main worktree; no agent) |
| slot | `◌ slot` | a worktree with no agent — a spawn target (windowed or windowless) |

A managed repo's section bar additionally carries a source-control indicator (`󱘎`), its
worktree count, and a most-urgent-status badge (a status glyph + count), so recognition and
urgency both read from a glyph and a word, never color. Within the repo the base pins first
and slots sort last, with live worktrees ordered by **name** in between - section and row
position are stable identity, never status, so a row a user is watching never moves when its
agent changes state.

## Two lenses

The dash is two always-visible lenses over the same agents, plus the preview:

- **Agents** (left): a flat triage list in four fixed bands - `NEEDS YOU`, `WORKING`,
  `IDLE`, `DONE` - always rendered in that order, each band drawn as a full-width section
  bar (the same distinct-background bar the Workspaces lens uses for repositories): a
  populated band is bold and carries its count, an empty band keeps the bar but goes faint
  (so an empty `NEEDS YOU` still reads as a section, just a quiet "all clear"). Agents are
  ordered newest-changed first within each band. Here urgency *does* move: a status change
  re-bands an agent, but never reorders its band peers.
- **Workspaces** (right): the repository→worktree→slot tree, ordered by name and held stable.
  Status is a per-row indicator and a section-bar badge, never a position. The incidental
  `(no-repo)` bucket states why it offers no worktrees (e.g. `not a git repo`).

Exactly one lens holds focus; `h`/`l` switch between them and the focused lens's title takes
the accent (the other reads gray). The focused selection drives the preview and shows in the
other lens with an accent **mirror** bullet (`theme.MirrorGlyph`, see Selection), so the same
agent is visibly linked across both without the unfocused lens sprouting a rival cursor. Folding (`tab`) works in **both** lenses and stays consistent: a Workspaces
section collapses its rows, an Agents band collapses its agents - each to a navigable header
stand-in carrying the same `▾`/`▸` glyph and a hidden count. An empty band has nothing to
collapse, so it is never foldable. Below a width threshold only the focused lens shows (still
toggled by `h`/`l`), sharing the preview.

**Action feedback** (a rejected action, an error, a confirmation) is *notable*, never the
faint help line — otherwise a no-op looks like nothing happened. It renders on a fixed line
below the list, bold and color-coded, paired with a glyph so it reads without color: a
rejection or failure is `✗` in `Red`, a neutral confirmation (e.g. *delete cancelled*) is `•`
in the accent. Even an action with nothing to do still speaks — closing an empty slot reports
*nothing to close* rather than swallowing the key. The line is always a single row, so it fits
the same height budget the list and preview share. A notice is *transient*: it auto-dismisses
after a few seconds, and clears the moment the cursor moves — feedback that is acknowledged
should get out of the way, never linger as stale state.

A **destructive confirmation** is not a status line but a centered modal whose frame is tinted
a warning hue, so the caution registers before any text is read: `Gold` for a plain delete, the
more severe `Red` for the dirty-worktree force-remove (which discards uncommitted changes). Its
prompt *wraps* to the modal width (a popup, unlike the single-row status line, so a long
worktree name is never sheared). The `cancel`/`confirm` choices are **selectable buttons**
defaulting to `cancel`: `←/→`/`h`/`l`/`tab` move between them and `⏎` activates the highlighted
one, while the `y`/`n` shortcuts still fire directly. The selected button carries the tool-wide
selection treatment (accent `CursorGlyph`, accent text, `RowHL` background); each button's
hotkey letter stays accented on both sides so the shortcut reads without relying on selection.

## Layout philosophy

- **Fixed columns.** The agents view is `cursor | status gutter | indent | window | name`, each at a
  stable horizontal position; over-long values truncate with `…`.
- **Section bars.** A group header (a recognized repository) is a full-width bar, left-aligned, in a
  distinct background — visually different from the selection highlight so structure and cursor never
  blur. Sections group by repository, not by tmux session.
- **The status gutter is pinned.** Status sits at the same column on every row regardless of
  grouping, so it reads as one vertical stripe.

## Recognition over ownership

birdseye's advanced features (agent orchestration) are built on top of **independently useful
primitives** — git worktrees, tmux session/window/pane management, agent status monitoring — and
never require the user to adopt the whole. Each primitive stands alone: a user who wants only the
worktree tool, or only the agent status view, gets exactly that. The tool layers in two tiers:

- **Primitives.** Standalone, self-contained operations (`be worktree`, session listing/creation,
  agent status from hooks plus the live OSC-title level signal). Usable on their own; they know
  nothing about orchestration.
- **Orchestration.** Pure *composition* of those primitives plus a recognition step. It owns no
  persistent state — every refresh it re-derives what it knows from tmux + the filesystem + hooks,
  the way agent liveness is already re-derived rather than tracked.

The litmus test for any new feature: **if it requires birdseye to own state, it is probably the
wrong design.** Recognize the structure each tick; don't become a session manager.

### The orthogonal row model

In the agents view, two facts about a row are **independent**: whether it has a live **agent**
(a hook record) and whether it is a managed **worktree** (a worktree of a recognized repository). A
row can be either, both, or neither. This orthogonality is what lets a recognized repository and a
user's ordinary, unrecognized sessions coexist in the same view — side by side — without one
reshaping the other.

Recognition is git-native and **repo-first**: the view groups by **repository** (keyed by
`git rev-parse --git-common-dir`), not by tmux session. A repository is recognized when **any** pane's
start path **or any** active agent's working directory resolves (via the git-common-dir /
`git worktree list`) to a worktree of it — not by matching any path shape. There is **no
whole-session purity gate**: a stray non-git pane never suppresses a repository, and panes spanning
two repositories yield two sections. One repository's rows may be drawn from several tmux sessions; a
row whose pane lives outside the repository's home session carries an `[in: <session>]` locator
hint. An agent with no git context renders as an incidental, ungrouped row, identical to a tool-free
terminal.

be authors windows only in the sessions it names - the **`<repo>-<hash>` home is its write domain**, a
deterministic, collision-safe function of the repository's identity (its git-common-dir). The trailing
hash both disambiguates same-named repositories and marks the session as be-derived, without a noisy
prefix; the same name is used by the dash's `n`, `be agents spawn`, and the session picker, so they all
land in one session. Every other session is observed read-only; spawning from a repository whose only
presence is a user session creates its home rather than injecting into the user's session. This is the
relocation of "managed": not a tracked flag, but a rule about *where be is allowed to write*.

**An agent is the row; an agentless worktree is a row.** A worktree is no longer one-to-one with a
row: two agents in one worktree are **two rows** (the worktree label repeating), and a worktree with
no agent is a row even with no open window — the repo's base (its primary worktree) or an empty slot,
enumerated from the git worktree set. The view stays a two-level tree (repository → row) with the
[pinned status gutter](#layout-philosophy) intact; co-located agents add no third indent level. A
recognized repository's section bar carries a recognition indicator (glyph + worktree count, never
color alone); the base and slot rows each take their own pinned-gutter token (glyph + word), defined
in `theme` exactly like every other status — so the view never relies on color to convey "empty slot"
or "base."

## Primitives and clients

birdseye is a *capability* with interchangeable faces, not an app with a CLI bolted on. Agent
management is a headless primitive; a human TUI and an orchestrating agent are peer clients of it.
These principles are how that capability stays composable and un-opinionated as it grows — the
architectural expression of [Recognition over ownership](#recognition-over-ownership). Some of it is
direction the binary has not fully grown into yet; it is written as principle, not as a claim about
today's commands.

- **The CLI is the API.** Every capability is a composable command that speaks `--json`. There is no
  second, bespoke API: anything that can run a shell — a script, *or another agent* — can drive the
  whole fleet. New capability means a new verb, not a new surface.

- **Clients are peers; no face holds private powers.** Anything a human does in the dash is a
  headless command underneath. The shared core is the work lifecycle (list, status, spawn, close,
  delete); only *ergonomics* differ by client — a live preview is for human eyes, `--json` and a
  message-send are for an agent's. **Litmus:** if a UI can do something no command can, the
  capability is in the wrong place.

- **Substrate, not orchestrator.** birdseye supplies primitives; the *workflow* lives in the
  user's agents and prompts. The binary never learns what "QA" or "open a PR" means. An opinionated
  playbook — an orchestrator that spawns workers, reviews their output, opens PRs — is something a
  user's agent composes from the verbs, never logic baked into `be`.

- **Opinions ship as removable artifacts.** The binary stays un-opinionated; birdseye's *own*
  workflows ship as opt-in, installable skills / commands / hooks that compose the primitives from
  the outside (an interactive checklist by default, never an implicit install). Two tiers, each
  independently opt-in: hooks *enable* a capability (status detection); workflow skills are *pure
  opinion*. Remove the skills and nothing core breaks.

- **Address by durability.** A handle is only as stable as the thing it names. A *unit of work* is
  named by its git worktree — it survives renames, tmux restarts, and the agent respawning. A *live
  realization* is named by tmux's own ids (`%pane`, `@window`), never by mutable names or drifting
  window indices. Because addresses are *derived*, not assigned, recognizing ("importing") an agent
  birdseye did not spawn is automatic, not a command.

These are realized, not just aspirational: the work lifecycle (list, status, spawn, send,
jump, close, delete) lives in one operation layer (`internal/fleet`) that **both** faces
call. The `be dash` TUI's actions are thin adapters over it and the headless `be agents`
verbs call it directly, so the dash's `dd` (close) and `dD` (delete) are the *same*
operations as `be agents close` and `be agents delete` — neither face holds a power the
other lacks. Teardown safety is a property of the *key*, not a dialog: `dd` closes a window
only (reversible — the worktree persists as a slot) and acts instantly, while only the
irreversible `dD` (which also runs `git worktree remove`) confirms. That reversibility is
realized, not nominal: `⏎` on a windowless row gives it a window of its own and attaches —
an agent for a slot (a *spawn target*, started in its existing worktree with no re-`add`), a
plain shell for the base (agentless by design) — the inverse of the `dd` that closed it.
(`⏎` on a windowed row jumps to it as usual; a windowless row has no window to jump to, so
the key would otherwise land on an unrelated window in the session.) Addressing follows the durability
tiers literally: a unit of work is named by its git worktree (`<repo>` for the base,
`<repo>/<worktree>` for a linked one), derived from `git-common-dir` + `git worktree list`
on every call; a worktreeless agent is named by its tmux pane id (`%id`); the agent process
is the most ephemeral and is resolved at call time. Because every handle is *derived*, an
agent birdseye never spawned is addressable with no import step, and an ambiguous handle
(two repos in view sharing a basename) is refused with the disambiguating paths rather than
guessed.

- **One observable world.** Every client reads and writes one substrate — tmux, git, hooks — which
  is the single source of truth. Nothing is cached or owned, so concurrent actors (you and your
  orchestrator) never diverge, and the machine's work is visible on the same screen you would drive
  yourself, live.

- **Only wrap what you improve.** A primitive earns a place in birdseye only by adding something
  the raw substrate command lacks — a worktree command that encodes the layout policy earns its
  keep; a bare `git clone` does not, so it is not a birdseye command. When a wrapper stops adding
  value, it is removed.

## Degradation

- Never rely on color alone: pair it with a glyph, word, or shape (status words, the selection
  glyph, section bars).
- Truecolor when the terminal advertises it (`COLORTERM`), otherwise the 256-color fallback; lipgloss
  and the picker each downgrade through `theme`.
- Nerd-font glyphs (cursor, icons) can render as tofu or double-width on some fonts. Judge glyph
  rendering by `cat`-ing a sample in a raw terminal, not inside a TUI that may blank them.

## Customization

The accent is user-configurable via `agents.accent` (a `#rrggbb` hex) in the config; a malformed
value is rejected with a clear error rather than silently ignored. Other tokens are not yet
user-facing.

## Enforcement

`theme` is the single source of truth. A guard test scans non-`theme` Go sources and fails if a
color literal (`#rrggbb`, `lipgloss.Color("#...")`, a color SGR escape) or the raw selection glyph
appears outside `theme`. Add new colors and glyphs to `theme` and reference the token.
