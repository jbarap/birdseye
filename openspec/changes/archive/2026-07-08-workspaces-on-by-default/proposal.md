## Why

Dash namespaces ("workspaces") only do anything once a `[[workspace]]` block is declared - with
none configured the feature is inert, so a fresh install shows no tab bar and no grouping. But the
parent-derived automatic workspaces already give useful structure with zero configuration: repos
group by their parent directory on their own. Gating that behind an explicit `[[workspace]]` means
the common case (a user with a mix of projects under a couple of parent directories) sees nothing
until they hand-write config. The feature should be on by default, with configured workspaces as an
optional refinement and a single master switch to turn it off.

## What Changes

- Add a `[workspaces]` settings table with `enabled = true` by default - the master switch for the
  whole feature (tab bar, filtering, automatic parent-derived workspaces).
- When enabled (the default), automatic parent-derived workspaces form even with zero `[[workspace]]`
  declared: repositories group by parent directory into tabs. Configured `[[workspace]]` entries
  layer on top as before (fixed tabs first, automatic tabs after).
- The tab bar renders only when there is more than one tab to show; when `All` is the only tab
  (e.g. every repository shares one parent, or there are none) no chrome appears.
- `enabled = false` restores the fully inert behavior: no tab bar, no filtering, no automatic
  workspaces.
- The selected tab is persisted (best-effort, under the agent state dir) and restored on the next
  launch, falling back to `All` when the persisted tab is no longer selectable.

## Capabilities

### Modified Capabilities
- `dash-namespaces`: the feature is on by default, gated by a `[workspaces].enabled` master switch
  rather than by the presence of a configured `[[workspace]]`; automatic parent-derived workspaces
  form whenever enabled, and the tab bar hides when `All` is the only tab.

## Impact

- `internal/config`: new `WorkspaceSettings` type / `Workspaces` settings field (`enabled` bool,
  default true in `Default()`); `default.toml` documents the switch.
- `internal/agents`: `assignRepoTabs` forms automatic tabs when enabled regardless of configured
  count; `Run` signature carries the enabled flag; tab-bar / help / filtering gates key off enabled
  and tab count rather than `len(namespaces) > 0`.
- `internal/cli/agents.go`: read `[workspaces].enabled` and pass it through.
- `internal/agents/store.go`: a small `active-workspace.json` under the agent state dir persists the
  selected tab; the view restores it at startup and writes it on switch.
- Docs: `DESIGN.md`, `README.md` reconciled to on-by-default framing.
