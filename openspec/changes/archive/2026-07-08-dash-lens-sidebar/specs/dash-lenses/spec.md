## MODIFIED Requirements

### Requirement: Two always-visible lenses over one row set

The agents dashboard SHALL present two lenses over the same agent row set, stacked in a left
**sidebar**: an **Agents** lens above a **Workspaces** lens, with the preview pane beside them.
The two lenses SHALL be projections (ordering and grouping) of one underlying row set, not two
independently sourced lists, so a given agent is the same row in both. Both lenses SHALL be shown
whenever the terminal is tall enough to stack them; when it is too short to stack both, the
dashboard SHALL fall back to showing one lens at a time - the focused one - with a rebindable
focus toggle between them. The always-visible two-lens sidebar is the primary mode whenever height
allows, at any width.

#### Scenario: Both lenses stack in the sidebar

- **WHEN** the dashboard opens on a terminal tall enough to stack both lens panels
- **THEN** the Agents lens and the Workspaces lens are both visible, the Agents lens above the
  Workspaces lens in the left sidebar

#### Scenario: One agent, one row in each lens

- **WHEN** an agent appears in both lenses
- **THEN** it is the same underlying row in each, so an action or selection resolves to the
  same agent regardless of which lens it was reached through

#### Scenario: Short terminal falls back to a single lens

- **WHEN** the terminal is too short to stack both lens panels
- **THEN** the dashboard shows one lens at a time - the focused one - with a rebindable toggle
  between them, rather than cramming both

## ADDED Requirements

### Requirement: Lenses stack in a sidebar beside a full-height preview

The dashboard body SHALL be composed as a fixed-width left sidebar holding the stacked lenses and a
preview pane filling the remaining width at full height. The preview SHALL NOT share the sidebar's
column, and the lenses SHALL NOT share the preview's rows. Within the sidebar, the Agents pane and
the Workspaces pane SHALL divide the available height so that, together, they stand as tall as the
preview beside them and never overflow the terminal height.

#### Scenario: Preview fills the width beside the sidebar

- **WHEN** the preview is shown
- **THEN** it is rendered to the right of the sidebar, spanning the width the sidebar leaves and the
  full height of the body

#### Scenario: Sidebar height splits between the two lenses

- **WHEN** both lenses are stacked
- **THEN** the Agents pane and the Workspaces pane each take a share of the sidebar height, each
  keeping a minimum, and the stacked panels do not exceed the terminal height

### Requirement: Sidebar width and preview visibility are governed by width

The lens sidebar's share of the terminal width SHALL be governed by the `agents.split` ratio, with
the preview taking the remaining width. The preview SHALL be shown only when the terminal is wide
enough to seat the minimum sidebar and a usable preview side by side; when it is not, the preview
SHALL be dropped and the sidebar SHALL take the full width. Both lens panels SHALL render at the
sidebar's width so the column reads as one.

#### Scenario: Split divides width, not height

- **WHEN** `agents.split` is set to a fraction
- **THEN** that fraction of the width goes to the lens sidebar and the remainder goes to the preview
  beside it

#### Scenario: Narrow terminal drops the preview

- **WHEN** the terminal is too narrow to seat the minimum sidebar plus a usable preview side by side
- **THEN** the preview is not shown and the sidebar takes the full width

### Requirement: Lens focus switches on a vertical key pair

Because the lenses are stacked vertically, the dashboard SHALL bind the lens focus-switch to a
vertical key pair by default: one key focuses the Agents lens (above) and another focuses the
Workspaces lens (below). The bindings SHALL be rebindable, and the previous horizontal keys SHALL
remain bound as aliases so existing muscle memory keeps working.

#### Scenario: Focus moves down to the Workspaces lens

- **WHEN** the Agents lens is focused and the user presses the focus-down key
- **THEN** the Workspaces lens becomes focused, driving the preview and the linked selection

#### Scenario: Focus moves up to the Agents lens

- **WHEN** the Workspaces lens is focused and the user presses the focus-up key
- **THEN** the Agents lens becomes focused
