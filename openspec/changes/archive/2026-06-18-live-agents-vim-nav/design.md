## Context

`be agents` is a Bubble Tea program (`internal/agents/view.go`). `Run(src Source)`
calls `src.Agents()` exactly once, stuffs the slice into the model, and renders;
`Init()` returns `nil`, so there is no command stream and nothing ever re-reads
state. Status is sourced from per-session JSON files under
`$XDG_STATE_HOME/birds-eye/agents/` written atomically by Claude Code hooks
(`internal/agents/store.go`, `claude.go`). The list is sorted by status rank then
title in `ClaudeSource.Agents()`.

Navigation lives in `model.Update`: a `switch key.String()` with hard-coded cases
(`up`/`k`, `down`/`j`, `enter`, `q`/`esc`/`ctrl+c`). The help line in `View()` is a
fixed string. Everything else in bird's-eye is configured through `config.toml`
(`internal/config/config.go`), and `Agents` already has a `stale_after` key — but
the view receives no config beyond the stale duration, which `cli/agents.go` turns
into the source.

The view renders as a plain `strings.Builder` list: a title, one line per agent
(`cursor + fixed-width badge + padded title + faint location`), and a fixed help
string. There is no frame, no layout engine, and no preview — `lipgloss` is imported
but used only for a handful of foreground colors. The tmux backend
(`internal/tmux/tmux.go`) wraps an injectable `Runner` but exposes no pane capture.

This change makes the open view track state changes live, turns the keymap into
configurable vim-native data with defaults equal to today's behavior, refines the
visual presentation, and adds a live preview of the selected agent's pane.

## Goals / Non-Goals

**Goals:**

- The open view reflects current agent state without the user re-running the
  command, and preserves the selected agent across refreshes and re-sorts.
- A complete, configurable keymap (up, down, top, bottom, half-up, half-down,
  select, quit) with vim-native defaults including `gg`/`G` and `ctrl+d`/`ctrl+u`.
- Defaults change no existing behavior; the help line reflects active bindings.
- Config errors (unknown action, empty binding) fail loudly at load time.
- A refined, framed lipgloss layout that stays readable and degrades on narrow,
  color-limited terminals.
- A live preview of the selected agent's tmux pane that follows the cursor and
  refreshes with the list, without becoming a source of status.

**Non-Goals:**

- No change to how status is produced (hooks, status mapping, stale logic).
- No new agent types or richer status workflow.
- No scrolling/viewport/pagination redesign — half-page motions move the cursor; a
  full viewport is out of scope unless the list already needs one (it does not).
- No mouse support or general-purpose action framework beyond these actions.
- The preview is read-only pane output; no scrollback navigation, no interaction
  with the previewed session, and no use of pane content to infer status.
- No theming system or user-configurable colors; "prettier" means a fixed, tasteful
  default that respects terminal capabilities.

## Decisions

### Decision: Refresh via periodic `tea.Tick`, not `fsnotify`

Drive refresh with a Bubble Tea `tea.Tick` (e.g. every ~1s): `Init()` returns the
first tick; the tick handler re-runs `src.Agents()`, diffs into the model, and
schedules the next tick. `Source` stays the single read path, so the view never
learns about files.

- *Why over fsnotify:* the data is a handful of small JSON files re-read in
  microseconds; a 1s poll is imperceptible for a triage popup and needs no new
  dependency or platform-specific watcher lifecycle. fsnotify would add a dependency
  and edge cases (watch the dir, handle create/rename/atomic-replace events) for no
  felt benefit at this scale.
- *Trade-off:* up to ~1s latency and a steady wakeup while open. Acceptable for a
  transient popup; the interval can be a named constant (and later a config key if
  asked). Re-reading inside the tick keeps `Run` unchanged for callers.
- *Alternative considered:* push from the hook process — far more coupling for no
  gain; hooks are short-lived separate processes.

### Decision: Refresh re-reads through `Source`; selection tracked by SessionID

The tick command calls `src.Agents()` again (the existing sorted read). On result,
the model records the current cursor's `SessionID` before swapping the slice, then
relocates that SessionID in the new slice. If gone, clamp the old index into the new
bounds (nearest valid row). This keeps the cursor on "the agent I was looking at"
even when re-sorting moves it (e.g. it flips to needs-attention and jumps to top).

- *Why SessionID:* it is the stable identity (filename key); title and index are
  not stable across refreshes.
- *Trade-off:* `Source` is read on every tick. Cost is trivial; if it ever matters,
  caching/mtime checks can be added behind the same interface.

### Decision: Keymap as a config-driven `action → []key` table with a typed default

Define an `Action` enum (Up, Down, Top, Bottom, HalfUp, HalfDown, Select, Quit) and
a `Keymap` that maps each action to a set of key strings (Bubble Tea
`key.String()` values like `"j"`, `"ctrl+d"`, `"G"`). The view's `Update` resolves
the pressed key to an action via a reverse lookup built once.

Multi-key motions (`gg`) are handled with a tiny pending-`g` state in the model: a
lone `g` arms it, a second `g` within the same keypress sequence fires Top, any
other key clears it. `G` is a single key. This avoids a general chord engine.

- *Config shape:* a new `[agents.keys]` TOML table, `map[string][]string` action →
  keys, merged onto defaults so users override only what they want. Validate at load:
  unknown action name → error; an action present with an empty list → error. Absent
  table → defaults verbatim (`up = ["k","up"]`, `down = ["j","down"]`,
  `top = ["g","gg"]`, `bottom = ["G"]`, `half_down = ["ctrl+d"]`,
  `half_up = ["ctrl+u"]`, `select = ["enter"]`, `quit = ["q","esc","ctrl+c"]`).
- *Why a flat action→keys map:* it is the minimal shape that supports rebinding and
  multiple keys per action, mirrors the existing config style (maps with defaults),
  and keeps `Update` a single resolve-then-switch.
- *Alternative considered:* key→action map. Rejected — users think in "what key do I
  press for X", and one action commonly has several keys; action→keys reads better in
  TOML and validates naturally.

### Decision: Help line generated from the active keymap

`View()` builds the help string from the keymap (e.g. join the first/representative
key per action) instead of a literal. Keeps the displayed hints honest after
rebinding and removes the duplicated-truth bug where help and bindings drift.

### Decision: Capture preview via `tmux capture-pane` behind a small interface

Add `CapturePane(target string, lines int) (string, error)` to `tmux.Client`,
implemented as `capture-pane -p -t <target>` (optionally `-S -<lines>` for the last
N lines, `-e` to keep colors). The view depends not on `*tmux.Client` but on a tiny
`Previewer` interface (`Preview(agent Agent) (string, error)`) so it stays testable
with a fake, consistent with the project's injectable-runner philosophy. The target
is the agent's `TmuxSession` (plus `:TmuxWindow` when set); capture reads the active
pane of that window.

- *Why capture-pane:* it is the standard, dependency-free way to read a session's
  current screen, exactly what a preview wants. It is read-only and cheap.
- *Status separation:* this is the one place the tool reads pane content, and it is
  display-only — `ClaudeSource` and status logic never consult it, preserving the
  "status from hooks, not scraping" invariant. The spec calls this out explicitly.
- *Failure handling:* capture errors (no session, no server, not in tmux) return an
  empty string / error that the view renders as an explicit placeholder; the view
  never fails because a preview is unavailable.
- *Alternative considered:* spawn a real tmux pane in the popup. Rejected — far more
  complex, fights the single-program TUI model, and is unnecessary for a glanceable
  preview.

### Decision: Two-pane responsive layout composed with lipgloss

Handle `tea.WindowSizeMsg` to track width/height. When width is comfortably wide,
render list (left) and preview (right) side by side with `lipgloss.JoinHorizontal`,
each in a bordered box sized to a share of the width; below a threshold, drop the
preview and render the list full-width (and, if very short, trim the help line).
The title becomes a styled bar; rows get a selected-row background/marker; badges
get per-status color plus a short glyph/label so they remain distinguishable without
color. A `pad`/align pass keeps columns aligned (replacing the ad-hoc `pad` helper
with lipgloss width styles).

- *Why responsive + lipgloss:* the view targets tmux popups of varying sizes, so a
  fixed two-column layout would overflow small popups; lipgloss already ships and
  gives borders, alignment, and width-aware joins without a new dependency.
- *Color-limited terminals:* pair color with a glyph/label and use a marker for the
  selected row so meaning survives on monochrome terminals, satisfying the spec's
  degradation scenario.
- *Trade-off:* layout math adds code; kept contained in `View()` helpers and covered
  by rendering tests at a couple of representative sizes.
- *Alternative considered:* preview below the list (stacked). Viable and used as the
  narrow-width fallback, but side-by-side reads better when width allows, so width
  selects between them.

### Decision: Preview fetched on selection change and on refresh, not every key

Fetch the preview when the selected agent changes and on each refresh tick, caching
the last preview string in the model so unrelated re-renders (e.g. a help toggle)
don't re-shell out. The same SessionID-based selection tracking decides when the
"selected agent changed". This bounds `capture-pane` calls to roughly one per cursor
move plus one per tick.

- *Why:* `capture-pane` is cheap but not free; tying it to selection/refresh avoids
  invoking it on every keypress while keeping the preview current.
- *Trade-off:* preview is at most one tick stale, matching the list's own latency.

### Decision: Thread the keymap and previewer from config through `cli/agents.go` into `Run`

`Run` gains the keymap and a previewer (e.g. `Run(src Source, prev Previewer, keys
Keymap)` or an options struct); `cli/agents.go` loads the keymap from config,
constructs the tmux-backed previewer (falling back to a no-op previewer when tmux is
absent), and passes them in. The tick interval is an internal constant. This keeps
config resolution and backend wiring at the CLI boundary, consistent with how
`stale_after` is already handled.

### Decision: Record the agent's pane id and target it for preview and jump

The hook runs inside the agent's pane, so `$TMUX_PANE` is exactly that pane.
`resolveTmux` now also returns `#{pane_id}` (e.g. `%5`), persisted as
`record.tmux_pane` and carried on `Agent.TmuxPane`. Both the preview capturer and
the jump prefer the pane id, falling back to `session:window` then `session` for
records written before this field existed.

- *Why:* `capture-pane`/`select-pane` against a window target act on whichever pane
  is *active*; in a split window that is often not the agent's pane. A globally
  unique pane id pins the right one. Verified empirically that window-capture grabs
  the active pane while pane-id capture grabs the agent's.
- *Jump:* `tmux.Client.ConnectPane(session, window, pane)` best-effort
  `select-window`/`select-pane` by pane id before attach/switch, so `enter` lands on
  the exact pane; a since-closed pane falls back to connecting to the session.
- *Trade-off:* pre-upgrade records lack a pane id until their hooks fire again; the
  fallbacks keep them working in the meantime.

### Decision: De-duplicate records by tmux location, keyed by pane

Multiple Claude sessions can map to one pane (restarting Claude in place writes a new
`session_id` file each time). `ClaudeSource.Agents` collapses records to one per
location — pane id when present, else `session:window`, else `session` — keeping the
most-recently-updated. A second pass drops pane-less legacy records superseded by a
pane-keyed agent in the same window, so the upgrade transition does not show a stale
duplicate.

- *Why key by pane, not window:* two agents in two panes of one window are distinct
  and must both survive; only the pane id distinguishes them. Window-keying would
  wrongly merge them.
- *Trade-off:* identity is the pane id; if tmux ever reuses a closed pane's id for an
  unrelated pane, an old record could merge with a new one — unlikely (ids
  increment), and newest-wins shows the live one.

### Decision: Garbage-collect aged-out state during the read

`Agents` deletes a record file when it is past its retention window: done records
after `forget_done`, others after `forget_stale` (both default 24h; zero disables).
Pruning piggybacks on the read the live view already performs each refresh, so it
needs no scheduler or daemon and self-heals on every `be agents` launch.

- *Why in the read path:* the view is the only reader; folding prune into it keeps
  list and disk consistent with no extra machinery. With 24h windows, deletions are
  rare, so the per-tick cost is negligible.
- *Alternative considered:* prune in the hook record path — runs far more often (every
  tool use) for no benefit; rejected.
- *Trade-off:* state is only collected while `be agents` runs. Between runs, ended
  files linger on disk (harmless) and are cleaned on next open.

### Decision: Style the fzf picker with raw ANSI, not lipgloss

`be list` shells out to fzf. The picker emits per-type icons and colors as raw ANSI
escapes and passes `--ansi` so fzf renders them; modern flags (`--layout=reverse`,
`--info=inline`, `--border`, `--cycle`, pointer/prompt/`--color`) give the snappy
look. Icons are a configurable `[icons]` per-type table (Nerd Font defaults).

- *Why raw ANSI over lipgloss:* picker output is piped to fzf, not a TTY; lipgloss
  (via termenv) strips color when stdout is not a terminal, so its styling would
  vanish. Emitting escapes directly guarantees they reach fzf, which renders them on
  the real TTY and strips them for match-scoring.
- *Why configurable icons:* defaults assume a Nerd Font; the `[icons]` table lets
  users substitute text/emoji or disable an icon, mirroring the existing `[labels]`.
- *Compatibility:* the flag set was verified against the installed fzf; matching is
  unaffected because fzf scores against the ANSI-stripped text.

## Risks / Trade-offs

- **Polling wakes the program ~1×/s while open** → interval is a single constant,
  cheap reads, and the popup is short-lived; promote to a config key only if asked.
- **`gg` pending-state could swallow a lone `g` meant for something else** → `g` is
  only bound to Top by default and any non-`g` key clears the pending state on the
  same render, so a stray `g` is a no-op, not a stuck mode.
- **Refresh could move the cursor under an actively-navigating user** → selection is
  tracked by SessionID and only clamped when the agent disappears, so normal
  navigation is undisturbed; re-sorts follow the agent the user was on.
- **Config typos silently disabling keys** → validation rejects unknown actions and
  empty key lists at load with a named error, consistent with the existing
  malformed-config behavior.
- **Multiple keys binding the same physical key to two actions** → last-writer or
  documented precedence; resolve by building the reverse map deterministically and,
  if a collision exists, erroring at load so it is caught early.
- **`capture-pane` shells out on each tick/selection** → calls are cheap, bounded to
  one per cursor move plus one per tick, and cached between unrelated re-renders; a
  failed capture degrades to a placeholder, never an error.
- **Preview could be mistaken for a status source** → it is display-only behind a
  separate `Previewer`; status code never reads it, and the spec encodes this as a
  requirement so the invariant is testable.
- **Layout breaks on tiny popups** → `tea.WindowSizeMsg`-driven thresholds hide the
  preview and trim chrome below set widths/heights; rendering tests cover small and
  wide sizes.
- **Colors washed out on limited terminals** → status conveyed by glyph/label plus
  color and a selected-row marker, so meaning does not rely on color alone.

## Open Questions

- Refresh interval value (proposed ~1s) and whether to expose it as config now or
  later. Default: keep internal for this change.
- Whether to support a configurable refresh-paused/manual-refresh key. Out of scope
  unless requested.
- Preview detail: number of captured lines, whether to keep ANSI colors (`-e`), and
  the width threshold for showing the preview. Proposed defaults: last ~full screen
  of the pane, colors preserved, preview shown above a reasonable width; tunable as
  constants now, promoted to config (`[agents]` preview keys) only if asked.
