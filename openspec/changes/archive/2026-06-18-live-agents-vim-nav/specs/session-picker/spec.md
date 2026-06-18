## MODIFIED Requirements

### Requirement: Candidates grouped and labeled by type

The system SHALL distinguish existing sessions from creatable candidates and label each
candidate with its source type, so the user can tell at a glance whether selecting it
attaches to a running session or creates a new one. The presentation SHALL be visually
refined — a per-type icon and color and a clear create-vs-attach marker — and the per-type
icons SHALL be configurable, including the ability to disable an icon. Matching, ordering,
and selection behavior SHALL be unchanged by the presentation.

#### Scenario: Existing vs creatable distinction

- **WHEN** the candidate list contains both running tmux sessions and tmuxp templates
- **THEN** each entry is labeled with its type and whether it attaches or creates, with a
  visually distinct create-vs-attach marker

#### Scenario: Configurable ordering

- **WHEN** config specifies an ordering or labels for candidate types
- **THEN** the picker presents types in that order with those labels

#### Scenario: Per-type icons

- **WHEN** the picker renders candidates
- **THEN** each candidate shows its type's configured icon and accent color; an icon
  configured as empty is omitted while the rest of the entry still renders

#### Scenario: Presentation does not affect matching

- **WHEN** the user types a query to fuzzy-filter candidates
- **THEN** matches are computed against the candidate text irrespective of the icons and
  color styling applied for display
