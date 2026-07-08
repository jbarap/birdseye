# dash-namespaces Specification

## Purpose
TBD - created by archiving change add-dash-namespaces. Update Purpose after archive.
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
workspace derived from its parent directory (active whenever at least one namespace is configured):
the workspace's identity is the repository's parent path and its tab label is that directory's base
name. Repositories sharing a parent directory SHALL collect under the same automatic workspace.
Automatic workspaces are derived fresh from the live rows, so an automatic workspace SHALL exist
only while at least one of its repositories is present. This fallback SHALL be active by default
whenever namespaces are configured; when none are configured the feature stays inert and no
automatic workspace is formed. Incidental agents (not in a recognized repository) have no parent
repository path and SHALL NOT form an automatic workspace - they remain under `All` only.

#### Scenario: An unmatched repository forms a parent-derived tab

- **WHEN** a namespace is configured and a repository under no configured root is present at
  `~/experiments/foo`
- **THEN** an automatic workspace labelled `experiments` (its parent directory's base name) holds
  that repository, appearing as a tab alongside the configured ones

#### Scenario: Sibling repositories share one automatic workspace

- **WHEN** two unmatched repositories live under the same parent directory
- **THEN** they collect under a single automatic workspace for that parent, not one tab each

#### Scenario: An automatic workspace exists only while populated

- **WHEN** the last repository under an automatic workspace's parent leaves the dashboard
- **THEN** that automatic workspace's tab disappears, and a user viewing it falls back to `All`

#### Scenario: No automatic workspaces without configured namespaces

- **WHEN** no namespace is configured
- **THEN** no tab bar and no automatic workspace are formed - the dashboard is unchanged

### Requirement: Namespace tab bar with an All default

When at least one namespace is configured, the dashboard SHALL render a **tab bar** at the top of
`be dash` listing `All` first, then the configured namespaces in configuration order, then any
automatic parent-derived workspaces sorted stably by label. The configured tabs SHALL keep a fixed
position; only the automatic tail reflects the live rows, so an appearing or disappearing
automatic tab never shifts a configured tab. Exactly one tab SHALL be active at a time, and the
dashboard SHALL open with `All` active, so the
default view shows every row as it does today. The active tab SHALL be visually distinct using the
theme accent paired with a glyph or shape, never color alone. The tab bar SHALL occupy one line of
dashboard chrome, accounted for in the body height so the sidebar and preview never overflow.

#### Scenario: Tab bar lists All then configured namespaces

- **WHEN** the dashboard opens with namespaces configured
- **THEN** a tab bar appears at the top listing `All` first and the configured namespaces after it,
  with `All` active

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

### Requirement: Inert when no namespace is configured

When no namespace is configured, the dashboard SHALL NOT render a tab bar, SHALL NOT filter the row
set, and SHALL behave exactly as a dashboard with no namespace feature. The feature SHALL add no
chrome and consume no body height until at least one namespace is declared.

#### Scenario: No configuration means no change

- **WHEN** the dashboard opens with no namespace configured
- **THEN** no tab bar is shown, no filtering occurs, and the layout is identical to today's

