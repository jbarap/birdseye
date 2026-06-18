## 1. Keymap config

- [x] 1.1 Add an `Action` enum (Up, Down, Top, Bottom, HalfUp, HalfDown, Select, Quit) and a `Keymap` type (`action → []key`) in the agents package, with a `DefaultKeymap()` matching today's bindings (`up=[k,up]`, `down=[j,down]`, `top=[g,gg]`, `bottom=[G]`, `half_down=[ctrl+d]`, `half_up=[ctrl+u]`, `select=[enter]`, `quit=[q,esc,ctrl+c]`).
- [x] 1.2 Add `Keys map[string][]string` to `config.Agents` (TOML `[agents.keys]`), defaulting absent to no override in `config.Default()`.
- [x] 1.3 Resolve config keys onto the default keymap and validate at load: unknown action name → error, action present with empty key list → error, duplicate key bound to two actions → error; surface a clear message naming the problem.
- [x] 1.4 Unit-test keymap resolution and validation: defaults when absent, override merges, and each error case.

## 2. Configurable, vim-native navigation in the view

- [x] 2.1 Add the keymap to the Bubble Tea `model` and build a reverse key→action lookup once; replace the hard-coded `switch key.String()` in `Update` with resolve-to-action then act.
- [x] 2.2 Implement Top, Bottom, HalfUp, HalfDown cursor motions clamped to list bounds (half-page size derived from a sensible page constant).
- [x] 2.3 Implement the `gg` two-key motion via a pending-`g` flag: lone `g` arms it, second `g` fires Top, any other key clears it.
- [x] 2.4 Generate the help line in `View()` from the active keymap instead of the fixed string.
- [x] 2.5 Unit-test navigation: vim motions move/clamp correctly, `gg`/`G` jump to ends, default bindings reproduce prior behavior, and a rebind drives the new key while the old key no longer does.

## 3. Live refresh

- [x] 3.1 Change `Init()` to emit an initial `tea.Tick`; add a refresh-tick message handled in `Update` that re-reads `src.Agents()` and reschedules the next tick (interval as a named constant).
- [x] 3.2 On refresh, preserve selection by SessionID: capture the current cursor's SessionID, relocate it in the new list, else clamp the old index into the new bounds.
- [x] 3.3 Keep `Source` as the only read path so the view never touches files directly; have `Run` accept the keymap (e.g. `Run(src, keymap)`) and start the tick loop.
- [x] 3.4 Unit-test refresh handling with a fake `Source`: new state re-renders, selection follows its agent across a re-sort, and a vanished agent clamps to a valid row.

## 4. Session preview

- [x] 4.1 Add `CapturePane(target string, lines int) (string, error)` to `tmux.Client` (`capture-pane -p -t <target>`, last-N lines, colors preserved); unit-test it with a fake runner.
- [x] 4.2 Define a `Previewer` interface (`Preview(Agent) (string, error)`) in the agents package and a tmux-backed implementation that targets `TmuxSession[:TmuxWindow]`; provide a no-op previewer for when tmux is absent.
- [x] 4.3 Add the previewer to the `model`; fetch the preview when the selected agent changes and on each refresh tick, caching the last preview so unrelated re-renders don't re-capture.
- [x] 4.4 Render an explicit placeholder when capture returns empty/error so the view never fails on an unavailable preview.
- [x] 4.5 Unit-test preview behavior with a fake `Previewer`: follows the cursor, refreshes with the tick, placeholder on error, and that status is never derived from preview output.

## 5. Visual presentation

- [x] 5.1 Handle `tea.WindowSizeMsg` in `Update` to track width/height in the model.
- [x] 5.2 Build a framed lipgloss layout: styled title bar, aligned columns (replace the ad-hoc `pad` with width styles), per-status badges with color plus a glyph/label, and a clearly marked selected row.
- [x] 5.3 Compose list + preview side by side with `lipgloss.JoinHorizontal` when wide; below a width threshold hide the preview (stack/full-width list) and trim chrome when very short.
- [x] 5.4 Ensure status and selection stay distinguishable without color (glyph/label + selected-row marker).
- [x] 5.5 Add rendering tests at a couple of representative sizes (wide with preview, narrow without) and an empty-state check.

## 6. Wiring and docs

- [x] 6.1 Update `cli/agents.go` to load the keymap from config, construct the tmux-backed previewer (no-op fallback when tmux is absent), and pass both into `agents.Run`.
- [x] 6.2 Update README: document `[agents.keys]` with defaults, and note the live-refresh, preview, and refined-view behavior in the agents section.
- [x] 6.3 Run `gofmt`/`go vet`, `go build ./...`, and `go test ./...`; verify in a tmux popup that `be agents` refreshes live, vim/custom bindings work, the preview tracks the cursor, and the layout adapts to small and large popups.

## 7. Exact-pane targeting

- [x] 7.1 Record the agent's pane id: `resolveTmux` returns `#{pane_id}`; add `TmuxPane` to `record` and `Agent`, store it in `HandleHook`, and map it in `ClaudeSource.Agents`.
- [x] 7.2 Prefer the pane id in the preview target (`previewTarget`), falling back to `session:window` then `session`.
- [x] 7.3 Add `tmux.Client.ConnectPane(session, window, pane)` that best-effort focuses the window/pane before attach/switch; call it from `cli/agents.go` so jump lands on the exact pane.
- [x] 7.4 Unit-test pane-id precedence (`preview_test.go`) and `ConnectPane` focus/fallback (`tmux_test.go`).

## 8. De-duplication

- [x] 8.1 Collapse records sharing a tmux location in `ClaudeSource.Agents` (key by pane, else session:window, else session), keeping the most recent; drop pane-less legacy records superseded by a pane-keyed agent in the same window.
- [x] 8.2 Unit-test dedup: restart-in-pane collapses, legacy supersession, and distinct panes survive (`TestAgentsDedupsSharedLocation`).

## 9. Garbage collection

- [x] 9.1 Add `Agents.ForgetDone`/`Agents.ForgetStale` config (default 24h each, 0 disables) and thread them through `NewClaudeSource`.
- [x] 9.2 Delete aged-out record files during `Agents` (done past `forget_done`, others past `forget_stale`).
- [x] 9.3 Unit-test pruning removes aged records from the list and deletes their files while keeping recent ones (`TestAgentsPrunesExpiredRecords`); document `forget_done`/`forget_stale` in README.

## 10. Picker (`be list`) visual refresh

- [x] 10.1 Add a per-type `Icons` config (`[icons]`, Nerd Font defaults) with a `Config.Icon` accessor.
- [x] 10.2 Render candidates with per-type icon + accent color, bold label, dim type tag, and a colored create-vs-attach marker, emitting raw ANSI.
- [x] 10.3 Add `--ansi` and modern fzf flags (reverse layout, inline info, border, cycle, pointer/prompt/color); verify the installed fzf accepts them.
- [x] 10.4 Update tests for the colorized display and icon-omitted case; document `[icons]` in README.
