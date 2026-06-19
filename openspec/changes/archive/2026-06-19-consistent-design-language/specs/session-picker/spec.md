## ADDED Requirements

### Requirement: Consistent selection chrome

The picker's selection and accent chrome SHALL use the tool-wide shared accent color and the shared
selection glyph, so that "the selected thing" is presented the same way in the picker as in the
agents view. Specifically, the fzf selection/match-highlight, pointer, prompt, and marker colors
SHALL be drawn from the shared accent token rather than a picker-specific color, and the fzf pointer
glyph SHALL be the shared selection glyph rather than a picker-specific marker. Candidate **type**
colors (for example coral for tmuxp templates) and the per-type icons SHALL be unaffected, since
they convey type rather than selection. All of these colors and the glyph SHALL come from the shared
theme tokens; the picker SHALL NOT hardcode them.

#### Scenario: Picker selection uses the shared accent

- **WHEN** the picker renders its fzf chrome (pointer, prompt, selected-match highlight, marker)
- **THEN** those elements use the shared accent color, matching the agents view's selection accent,
  rather than a picker-specific accent

#### Scenario: Picker pointer uses the shared selection glyph

- **WHEN** the picker shows its selection pointer
- **THEN** the pointer is the shared selection glyph used by the agents-view cursor, not a separate
  marker

#### Scenario: Candidate type colors are unchanged

- **WHEN** candidates of differing types render with their icons and labels
- **THEN** each type keeps its own type color (such as coral for tmuxp), independent of the shared
  selection accent
