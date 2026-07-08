# dash-namespaces Specification

## Purpose
Slice `be dash` by life-context so a mixed set of projects (personal, work, ...) is not one
undifferentiated view. Repositories map to path-derived **namespaces** ("workspaces") surfaced as a
top tab bar; `]` / `[` switch the active tab, filtering both lenses upstream. The feature is on by
default - repositories group automatically by parent directory, configured `[[workspace]]` entries
refine that, and a `[workspaces]` master switch turns it off. Non-active tabs surface hidden urgency
so a blocked agent is never invisible, and the selected tab persists across launches.
## Requirements
### Requirement: Path-derived namespaces from configured roots

A **namespace** SHALL be declared in configuration by a name and a set of roots (directories,
following the tilde/glob conventions of the existing `[repo].roots` and `[dir].roots`). A
recognized repository SHALL belong to the namespace whose configured root contains the
repository's path. When a repository's path lies under the roots of more than one namespace, it
SHALL belong to the namespace with the most specific (longest matching) root, so membership is
deterministic. A repository under no namespace root SHALL fall back to an automatic parent-derived
workspace rather than being confined to `All` (see the fallback requirement). Namespace membership
SHALL be derived fresh from configured roots on each
refresh - it is not persisted per agent or per repository.

#### Scenario: Repository maps to the namespace whose root contains it

- **WHEN** a repository lives under a directory configured as a namespace's root
- **THEN** the repository belongs to that namespace, and its rows are shown when that namespace is
  the active tab

#### Scenario: Most specific root wins

- **WHEN** a repository's path lies under the roots of two configured namespaces
- **THEN** it belongs to the namespace whose matching root path is the longest, so membership does
  not depend on config ordering

#### Scenario: Unrooted repository falls back to a parent-derived workspace

- **WHEN** a repository's path is under no configured namespace root
- **THEN** it belongs to an automatic workspace derived from its parent directory (see the
  parent-derived fallback requirement), rather than being confined to `All` only

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

### Requirement: Cycle the active namespace

The dashboard SHALL bind a rebindable pair of actions to move the active tab to the next and
previous namespace, defaulting to `]` (next) and `[` (previous). Cycling SHALL wrap around the tab
list, which includes `All`. Switching the active tab SHALL re-filter the row set and update the
lenses and preview accordingly.

#### Scenario: Next cycles forward and wraps

- **WHEN** the last tab is active and the next-namespace key is pressed
- **THEN** the active tab wraps to `All` at the start of the list

#### Scenario: Previous cycles backward

- **WHEN** a named namespace is active and the previous-namespace key is pressed
- **THEN** the active tab moves one tab toward `All`

### Requirement: Namespace filters the shared row set upstream

The active namespace SHALL filter the dashboard row set exactly once, upstream of the two lenses,
so the Agents lens and the Projects lens remain consistent projections of the same filtered set.
The `All` tab SHALL pass every row through unfiltered. A named namespace SHALL keep only the rows
whose repository maps to it. Incidental agents - those not inside any recognized git repository -
SHALL appear only under `All`, since they map to no repository and therefore no namespace.

#### Scenario: All passes every row

- **WHEN** the `All` tab is active
- **THEN** every row is shown in both lenses, exactly as when no namespace is selected

#### Scenario: A named namespace keeps only its repositories

- **WHEN** a named namespace is active
- **THEN** both lenses show only rows whose repository maps to that namespace, and the same
  filtered set feeds both, so an agent visible in one lens is visible in the other

#### Scenario: Incidental agents appear only under All

- **WHEN** a named namespace is active and an incidental agent (not in a recognized repository)
  exists
- **THEN** that agent is not shown under the named namespace, and it is shown under `All`

### Requirement: Non-active tabs surface hidden urgency

Each non-active tab SHALL show an urgency indicator when its namespace slice contains an agent in
`needs-attention`, so a blocked agent in a namespace the user is not currently viewing is never
fully hidden. The indicator SHALL be a glyph (not color alone). Consistent with how muting
suppresses a section's urgency badge, a **muted** agent SHALL NOT raise a tab's urgency indicator.

#### Scenario: A blocked agent in a hidden namespace dots its tab

- **WHEN** a namespace that is not the active tab contains an agent in needs-attention
- **THEN** that tab shows an urgency indicator, so the user can see attention is required elsewhere

#### Scenario: A muted agent does not dot its tab

- **WHEN** the only needs-attention agent in a non-active namespace is muted
- **THEN** that tab shows no urgency indicator, so muting suppresses the cross-namespace signal too

### Requirement: Inert when disabled

When the feature is disabled (`[workspaces]` `enabled = false`), the dashboard SHALL NOT render a
tab bar, SHALL NOT filter the row set, and SHALL NOT form automatic workspaces, behaving exactly as
a dashboard with no namespace feature. The feature SHALL add no chrome and consume no body height
while disabled, regardless of any declared `[[workspace]]`.

#### Scenario: Disabling means no change

- **WHEN** the dashboard opens with the feature disabled
- **THEN** no tab bar is shown, no filtering occurs, and the layout is identical to today's, even if
  `[[workspace]]` entries are declared

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

