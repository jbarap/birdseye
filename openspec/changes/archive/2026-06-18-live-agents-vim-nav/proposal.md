## Why

`be agents` is built to live in a tmux popup as a triage dashboard, but today it
snapshots agent state once at launch and never updates — a popup left open shows
stale statuses while hooks keep writing fresh state underneath it. Its navigation
is also a fixed, partial set of keys (`j`/`k`, arrows, `enter`, `q`); there is no
jump-to-top/bottom or half-page movement a vim user expects, and no way to rebind
keys the way every other part of bird's-eye is configurable.

The view is also bare and hard to read — an unframed list with a fixed-width status
badge and a faint help line — and it shows only a one-word status per agent, so the
user must actually jump into a session to learn *what* an agent is doing. A triage
view should let you read the room without leaving it.

## What Changes

- The agents view refreshes in place: as Claude Code hooks write new state, the
  list re-renders without the user re-running `be agents`. The cursor stays on the
  same agent across refreshes where possible, and re-sorting (e.g. an agent moving
  to needs-attention) does not yank the selection out from under the user.
- Navigation gains the vim-native motions a triage view needs: `g`/`gg` to top,
  `G` to bottom, `ctrl+d`/`ctrl+u` half-page, in addition to the existing
  `j`/`k`/arrows. `enter` (jump) and `q`/`esc` (quit) are retained.
- Keybindings become configurable via `config.toml` under a new `[agents.keys]`
  table: each action (up, down, top, bottom, half-up, half-down, select, quit)
  maps to one or more keys, with the current bindings as defaults so existing
  muscle memory is unchanged.
- The in-view help line reflects the active (possibly customized) bindings rather
  than a hard-coded string.
- The view is visually refined with lipgloss: a framed layout, a styled title bar,
  status badges that read at a glance (color + glyph, aligned columns), and a clear
  cursor/selection treatment — degrading gracefully on narrow popups and limited
  color terminals.
- A live preview pane shows the recent terminal output of the selected agent's tmux
  session (via `tmux capture-pane`), so the user can see what an agent is doing
  without jumping to it. The preview tracks the cursor and refreshes with the list.
  Preview is purely a convenience view of pane output and does **not** change how
  status is determined — status remains hook-driven, never scraped. The hook records
  the agent's exact tmux **pane id**, so the preview (and the jump) target the pane
  the agent actually runs in, even when its window is split.
- Selecting an agent jumps to its exact window and pane, not just its session.
- One row per tmux location: multiple Claude sessions that map to the same pane (e.g.
  restarting Claude in place) are collapsed to the most recent, instead of stacking
  duplicate rows.
- State files are garbage-collected: ended (done) sessions are removed after a
  retention window, and crashed/abandoned sessions after a longer one, so the list
  and the on-disk state do not grow without bound.
- The `be` picker (`be list`) is given the same visual refresh: per-type icons and
  colors, a clear create-vs-attach marker, and a modern fzf presentation, all
  configurable.

## Capabilities

### New Capabilities

(none — this extends the existing agent view and session picker)

### Modified Capabilities

- `agent-view`: the popup-friendly invocation requirement gains live, in-place
  refresh of agent state and jump-to-exact-pane; the navigation behavior becomes an
  explicit, configurable, vim-native keymap rather than a fixed key set; the view
  gains a refined visual presentation; a new live preview of the selected agent's
  tmux pane output is added (without changing the hook-driven source of status);
  records sharing a tmux location are de-duplicated to one row; and aged-out state
  is garbage-collected.
- `session-picker`: the by-type candidate presentation gains per-type icons and
  colors and a modern fzf appearance, all configurable, without changing matching,
  ordering, or selection behavior.

## Impact

- Code: `internal/agents/view.go` (Bubble Tea model — refresh tick, configurable
  keymap, lipgloss layout/styling, two-pane list+preview with `tea.WindowSizeMsg`
  handling, per-cursor preview fetch), `internal/agents/claude.go` (record the pane
  id, de-duplicate records per tmux location, prune aged-out state files),
  `internal/agents/preview.go` (pane-id-first capture target),
  `internal/agents/store.go`/`agent.go` (carry `TmuxPane`),
  `internal/cli/agents.go` (pass keymap, previewer, and retention config; jump to the
  exact pane), `internal/config/config.go` (new `Agents.Keys`, `Agents.ForgetDone`,
  `Agents.ForgetStale`, and picker `Icons`), `internal/tmux/tmux.go` (`CapturePane`
  and `ConnectPane`), `internal/picker/picker.go` (per-type icons + ANSI colors and
  modern fzf flags).
- Dependencies: none added. The `tea.Tick` refresh, lipgloss styling, and raw-ANSI
  picker output all use what is already vendored; fzf renders the colors via
  `--ansi`.
- Docs: README configuration section gains the `[agents.keys]`, `[icons]`, and
  `forget_done`/`forget_stale` keys, and notes live-refresh, preview, and pruning.
- No breaking changes: defaults preserve today's keys; the preview degrades to
  hidden on narrow popups or when pane capture is unavailable; picker icons are
  overridable (and require a Nerd Font for the defaults to render); status semantics
  are unchanged.
