## MODIFIED Requirements

### Requirement: Consistent selection chrome

The picker's selection and accent chrome SHALL use the tool-wide shared accent color and the shared
selection glyph, so that "the selected thing" is presented the same way in the picker as in the
agents view. Specifically, the fzf selection/match-highlight, pointer, prompt, and marker colors
SHALL be drawn from the shared accent token rather than a picker-specific color, and the fzf pointer
glyph SHALL be the shared selection glyph rather than a picker-specific marker. The picker SHALL
also present fzf inside a **titled panel** matching the agents view: a minimal panel border with the
picker's title (`sessions`) embedded in the **top border** and rendered in the shared accent, rather
than an unstyled, untitled fzf border. The panel border color SHALL be the shared border token. The
picker SHALL NOT carry a textual brand in its prompt or panel title (the prompt is a bare chevron);
identity comes from the shared chrome. Candidate
**type** colors (for example coral for tmuxp templates) and the per-type icons SHALL be unaffected,
since they convey type rather than selection. All of these colors and the glyph SHALL come from the
shared theme tokens; the picker SHALL NOT hardcode them. Where the embedded-title border depends on
an fzf feature not present in every supported fzf version, the picker SHALL degrade gracefully
(omitting only the title) rather than failing.

#### Scenario: Picker selection uses the shared accent

- **WHEN** the picker renders its fzf chrome (pointer, prompt, selected-match highlight, marker)
- **THEN** those elements use the shared accent color, matching the agents view's selection accent,
  rather than a picker-specific accent

#### Scenario: Picker pointer uses the shared selection glyph

- **WHEN** the picker shows its selection pointer
- **THEN** the pointer is the shared selection glyph used by the agents-view cursor, not a separate
  marker

#### Scenario: Picker renders inside a titled panel

- **WHEN** the picker opens
- **THEN** fzf renders inside a bordered panel whose title (`sessions`) is embedded in the top
  border and colored with the shared accent, matching the agents view's titled panels, rather than
  an untitled raw-fzf border

#### Scenario: Border title degrades on older fzf

- **WHEN** the installed fzf does not support the embedded border-title feature
- **THEN** the picker still opens with its bordered panel and its accent chrome, omitting only the
  embedded title rather than erroring

#### Scenario: Candidate type colors are unchanged

- **WHEN** candidates of differing types render with their icons and labels
- **THEN** each type keeps its own type color (such as coral for tmuxp), independent of the shared
  selection accent
