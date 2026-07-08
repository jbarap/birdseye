## 1. Config: master switch

- [x] 1.1 Add `WorkspaceSettings` type (`Enabled bool `toml:"enabled"``) and a `Workspaces` settings field (`toml:"workspaces"`) to `Config`; keep the existing `[[workspace]]` array field (rename its Go field if it now collides).
- [x] 1.2 Default `Enabled: true` in `Default()` so an absent table keeps the feature on, and an explicit `enabled = false` overrides.
- [x] 1.3 Update `clone()` if needed and add/extend config tests: default is enabled, explicit false disables, unknown keys in `[workspaces]` still rejected by strict loading.
- [x] 1.4 Document `[workspaces] enabled` and the on-by-default / auto-parent behavior in `default.toml`.

## 2. Agents package: gate on enabled

- [x] 2.1 Change `assignRepoTabs` to take `enabled bool`: return `nil` when disabled; when enabled, always assign tabs (forming parent-derived automatic workspaces) regardless of configured count.
- [x] 2.2 Carry `workspacesEnabled bool` on the model; add it to `Run`'s signature and set it. Update all call sites.
- [x] 2.3 Replace the `len(m.namespaces) == 0` gates (`filteredRows`, `tabBarLines`, help entry, cycle actions) with an `enabled` + live-tab-count test. `tabBarLines` returns 1 only when enabled AND `len(tabList()) > 1`.
- [x] 2.4 Update `internal/cli/agents.go` to read `[workspaces].enabled` from config and pass it into `agents.Run`.
- [x] 2.5 Fix the first-frame delay: re-derive the tab assignment in `Run` after the namespace fields are set, so the bar shows on the first paint, not after the first tick.

## 3. Persist the active tab

- [x] 3.1 Add `readActiveTab` / `writeActiveTab` (a single `active-workspace.json`) to `store.go`, mirroring the mute file.
- [x] 3.2 Add a `stateDir` model field (empty in tests). In `Run`, restore the persisted tab before the first derive; in `switchNamespace`, write it. Restoration reuses the existing fallback-to-All reconciliation.
- [x] 3.3 Test the write/read round-trip for All, a configured name, and an automatic key (NUL byte).

## 4. Tests and docs

- [x] 4.1 Update `namespace_test.go`: rename/replace the "inert without namespaces" test to cover disabled-is-inert and enabled-with-zero-config forms auto tabs; add an `All`-only hide test. Update `nsModel` helper to set `workspacesEnabled`.
- [x] 4.2 Reconcile `DESIGN.md` and `README.md` to the on-by-default framing and note the persisted selection.
- [x] 4.3 `go test ./...`, `gofmt`, and drive the TUI (use-tty) to confirm: no config shows auto tabs (or no bar when single-parent), `enabled = false` shows no bar, and the selected tab survives a relaunch.
