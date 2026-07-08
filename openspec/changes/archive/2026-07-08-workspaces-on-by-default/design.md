## Context

The dash-namespaces feature currently gates on `len(namespaces) > 0`: `assignRepoTabs` returns
`nil` with no configured `[[workspace]]`, and the tab bar, filtering, help entry, and body-height
accounting all short-circuit on the same "no namespaces" condition. Automatic parent-derived
workspaces (the fallback that groups unmatched repos by parent directory) therefore only appear
once at least one namespace is configured, even though they need no per-namespace configuration to
be useful.

## Goals / Non-Goals

**Goals:**
- Make the feature on by default via a `[workspaces].enabled` master switch (default true).
- Form automatic parent-derived workspaces whenever enabled, with zero configured `[[workspace]]`.
- Keep configured `[[workspace]]` entries working exactly as today when present.
- Hide the tab bar when `All` is the only tab, so a user with one parent (or none) sees no chrome.
- Preserve a clean opt-out: `enabled = false` is the old fully-inert behavior.

**Non-Goals:**
- No change to how membership is derived (still path-derived, per-repo, longest-root-wins).
- No new persistence; workspaces remain recomputed each refresh.
- No change to the `]` / `[` bindings or the urgency-dot behavior.

## Decisions

- **Master switch lives in config as a settings table, not an array flag.** `[workspaces]` with
  `enabled` bool mirrors the existing `[dir].use_zoxide` / `[agents.notify]` pattern: the default
  lives in `Default()` so an absent table keeps `enabled = true`, while an explicit `enabled = false`
  overrides. The `[[workspace]]` array is unchanged and independent - a user can declare workspaces
  with the table absent, or set the table without declaring any workspaces.

- **The gate becomes `enabled`, not namespace count.** `assignRepoTabs` takes an `enabled bool`:
  when false it returns `nil` (inert); when true it always assigns tabs, forming parent-derived
  automatic workspaces for repositories that match no configured root - even when the configured set
  is empty. The agents model carries `workspacesEnabled` and the view gates (filter, tab bar height,
  help entry, cycle actions) key off it plus the live tab count.

- **Tab bar hides when `All` is the only tab.** `tabBarLines()` returns 1 only when enabled AND
  `len(tabList()) > 1`. This means enabling the feature on a machine whose repos all share one
  parent directory adds no chrome and no filtering surface - the bar appears only once there is a
  real choice. Cycle actions and the help entry follow the same "more than one tab" test, so `]` /
  `[` are inert (and hidden from help) when there is nothing to cycle to.

- **Signature: pass a bool through `Run`.** Rather than restructure into a `Workspaces{Enabled, Defs}`
  struct, add a single `workspacesEnabled bool` parameter to `agents.Run` alongside the existing
  `namespaces []Namespace`. The two are orthogonal inputs and adding a bool keeps the diff local;
  cli resolves both from config and passes them.

- **The selected tab persists across launches.** The view writes the active tab's key to a small
  `active-workspace.json` under the agent state dir (`StateDir()`) on each explicit switch, mirroring
  the mute-file pattern, and restores it at startup before the first frame is derived. It is
  view-only state, so it is written directly rather than routed through the source (unlike mute,
  which the source also reads). Restoration reuses the existing tab reconciliation: a persisted key
  that no longer names a selectable tab falls back to `All`, so a removed namespace or an emptied
  automatic workspace never blanks the view. The reconciliation reset (an automatic tab emptying
  mid-session) does not rewrite the file, so the user's last explicit choice is re-selected if that
  workspace returns. Persistence is gated on a non-empty `stateDir` field, left empty in tests so
  they touch no real state.

## Risks / Trade-offs

- **Default-on changes behavior on upgrade.** Users who never configured workspaces will, after
  upgrade, get a tab bar auto-grouped by parent directory - but only when they have repos under more
  than one parent (the `All`-only hide covers the single-parent case). The opt-out is one line
  (`[workspaces] enabled = false`), documented in `default.toml`. This is the intended behavior:
  the automatic grouping is useful and discoverable, and the switch makes it reversible.
- **Automatic tab churn.** With many parent directories a user could see several automatic tabs.
  This already exists once any namespace is configured; making it default-on widens exposure but
  does not change the mechanic. Sorting automatic tabs stably by label keeps them from jumping.
