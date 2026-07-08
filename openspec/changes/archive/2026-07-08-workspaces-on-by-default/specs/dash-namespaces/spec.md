## RENAMED Requirements

- FROM: `### Requirement: Inert when no namespace is configured`
- TO: `### Requirement: Inert when disabled`

## ADDED Requirements

### Requirement: Enabled by default with a master switch

The namespaces feature SHALL be governed by a single master switch in configuration, `enabled`, in
a `[workspaces]` settings table, defaulting to true when the table or key is absent. When enabled,
the feature is active with no further configuration: automatic parent-derived workspaces form from
the live repositories and configured `[[workspace]]` entries layer on top. The master switch and the
`[[workspace]]` declarations SHALL be independent - the feature is on by default with no workspace
declared, and setting the switch does not require declaring any workspace.

#### Scenario: On by default with no configuration

- **WHEN** the dashboard opens with no `[workspaces]` table and no `[[workspace]]` declared
- **THEN** the feature is enabled, and automatic parent-derived workspaces form from the live
  repositories

#### Scenario: Explicit disable turns the feature off

- **WHEN** the configuration sets `[workspaces]` `enabled = false`
- **THEN** the feature is inert regardless of any declared `[[workspace]]` - no tab bar, no
  filtering, no automatic workspaces

### Requirement: Active tab persists across launches

The dashboard SHALL persist the user's selected tab when they switch it, and restore that selection
on the next launch, so a user who works within one workspace is not returned to `All` every time.
The persisted selection SHALL be restored only when it names a currently-selectable tab (a
configured namespace, or an automatic workspace with a live repository); otherwise the dashboard
SHALL fall back to `All`, so a stale selection never blanks the view. Persistence SHALL be
best-effort - a failure to read or write it SHALL NOT disrupt the dashboard, which simply opens on
`All`.

#### Scenario: Last selected tab is restored on relaunch

- **WHEN** the user switches to a tab and later reopens the dashboard while that tab is still
  selectable
- **THEN** the dashboard opens with that tab active rather than `All`

#### Scenario: A stale persisted tab falls back to All

- **WHEN** the persisted tab no longer names a selectable tab (its namespace was removed from
  configuration, or its automatic workspace has no live repository)
- **THEN** the dashboard opens on `All` instead

## MODIFIED Requirements

### Requirement: Unmatched repositories fall back to a parent-derived workspace

A recognized repository that matches no configured namespace root SHALL fall back to an automatic
workspace derived from its parent directory (active whenever the feature is enabled, even with no
`[[workspace]]` declared): the workspace's identity is the repository's parent path and its tab
label is that directory's base name. Repositories sharing a parent directory SHALL collect under the
same automatic workspace. Automatic workspaces are derived fresh from the live rows, so an automatic
workspace SHALL exist only while at least one of its repositories is present. Incidental agents (not
in a recognized repository) have no parent repository path and SHALL NOT form an automatic
workspace - they remain under `All` only.

#### Scenario: An unmatched repository forms a parent-derived tab

- **WHEN** the feature is enabled and a repository under no configured root is present at
  `~/experiments/foo`
- **THEN** an automatic workspace labelled `experiments` (its parent directory's base name) holds
  that repository, appearing as a tab alongside any configured ones

#### Scenario: Sibling repositories share one automatic workspace

- **WHEN** two unmatched repositories live under the same parent directory
- **THEN** they collect under a single automatic workspace for that parent, not one tab each

#### Scenario: An automatic workspace exists only while populated

- **WHEN** the last repository under an automatic workspace's parent leaves the dashboard
- **THEN** that automatic workspace's tab disappears, and a user viewing it falls back to `All`

#### Scenario: Automatic workspaces form without any configured namespace

- **WHEN** the feature is enabled with no `[[workspace]]` declared and repositories under more than
  one parent directory are present
- **THEN** each parent directory forms an automatic workspace tab, so grouping appears with zero
  workspace configuration

### Requirement: Namespace tab bar with an All default

When the feature is enabled and more than one tab exists, the dashboard SHALL render a **tab bar**
at the top of `be dash` listing `All` first, then the configured namespaces in configuration order,
then any automatic parent-derived workspaces sorted stably by label. The configured tabs SHALL keep
a fixed position; only the automatic tail reflects the live rows, so an appearing or disappearing
automatic tab never shifts a configured tab. When `All` is the only tab - no configured namespace
and no recognized repository to form an automatic tab - the dashboard SHALL render no tab bar and
consume no body height, so enabling the feature adds no chrome until there is a real choice to make.
Exactly one tab SHALL be active at a time, and the dashboard SHALL open on the tab the user last
selected (restored per the persistence requirement), falling back to `All` when none was persisted
or the persisted tab is no longer selectable - so by default the view shows every row as it does
today. The active tab SHALL be visually distinct using the theme accent paired with a glyph or
shape, never color alone. The tab bar SHALL occupy one line of dashboard chrome, accounted for in
the body height so the sidebar and preview never overflow.

#### Scenario: Tab bar lists All then configured namespaces

- **WHEN** the dashboard opens enabled with namespaces configured and no tab was previously persisted
- **THEN** a tab bar appears at the top listing `All` first and the configured namespaces after it,
  with `All` active

#### Scenario: No tab bar when All is the only tab

- **WHEN** the feature is enabled but there is only the `All` tab (no configured namespace and no
  recognized repository - e.g. only incidental agents, or an empty dashboard)
- **THEN** no tab bar is shown and the layout is identical to a dashboard with the feature off

#### Scenario: Active tab is distinguished without relying on color

- **WHEN** a tab is active
- **THEN** it is marked with the theme accent and a glyph or shape, so the active tab is legible
  without color

### Requirement: Inert when disabled

When the feature is disabled (`[workspaces]` `enabled = false`), the dashboard SHALL NOT render a
tab bar, SHALL NOT filter the row set, and SHALL NOT form automatic workspaces, behaving exactly as
a dashboard with no namespace feature. The feature SHALL add no chrome and consume no body height
while disabled, regardless of any declared `[[workspace]]`.

#### Scenario: Disabling means no change

- **WHEN** the dashboard opens with the feature disabled
- **THEN** no tab bar is shown, no filtering occurs, and the layout is identical to today's, even if
  `[[workspace]]` entries are declared
