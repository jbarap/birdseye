# bird's-eye design language

This is the visual contract for bird's-eye. Every surface — the picker, the agents view, CLI
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
5. **Recognize structure; don't own it.** bird's-eye is a lens over tmux and the filesystem, not a
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
| `Green` | `#4ec98a` | worktree / success; the picker "create" mark |
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

## Layout philosophy

- **Fixed columns.** The agents view is `cursor | status gutter | indent | window | name`, each at a
  stable horizontal position; over-long values truncate with `…`.
- **Section bars.** A group header (a tmux session) is a full-width bar, left-aligned, in a distinct
  background — visually different from the selection highlight so structure and cursor never blur.
- **The status gutter is pinned.** Status sits at the same column on every row regardless of
  grouping, so it reads as one vertical stripe.

## Recognition over ownership

bird's-eye's advanced features (agent orchestration) are built on top of **independently useful
primitives** — git worktrees, tmux session/window/pane management, agent status monitoring — and
never require the user to adopt the whole. Each primitive stands alone: a user who wants only the
worktree tool, or only the agent status view, gets exactly that. The tool layers in two tiers:

- **Primitives.** Standalone, self-contained operations (`be worktree`, session listing/creation,
  hook-driven agent status). Usable on their own; they know nothing about orchestration.
- **Orchestration.** Pure *composition* of those primitives plus a recognition step. It owns no
  persistent state — every refresh it re-derives what it knows from tmux + the filesystem + hooks,
  the way agent liveness is already re-derived rather than tracked.

The litmus test for any new feature: **if it requires bird's-eye to own state, it is probably the
wrong design.** Recognize the structure each tick; don't become a session manager.

### The orthogonal row model

In the agents view, two facts about a row are **independent**: whether it has a live **agent**
(a hook record) and whether it is a managed **worktree** (its window started inside a recognized
repo's worktree). A row can be either, both, or neither. This orthogonality is what lets a managed
repo and a user's ordinary, unrecognized session coexist in the same view — and in the same tmux
session — without one reshaping the other. A window surfaces only when it has an agent, is a managed
worktree, or is a repo's anchor; anything else stays invisible, so an unrecognized session renders
identically to a tool-free terminal.

The **worktree is the row** — not a new indent level. In the common case (one agent per worktree),
the worktree row and the agent row are the same line, so the view stays a two-level tree
(session → row) with the [pinned status gutter](#layout-philosophy) intact. A managed repo's
session bar carries a recognition indicator (glyph + worktree count, never color alone); rows that
are worktrees without an agent, and the repo's default-branch anchor, each take their own
pinned-gutter token (glyph + word), defined in `theme` exactly like every other status — so the
managed view never relies on color to convey "empty slot" or "anchor."

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
