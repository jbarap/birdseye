## 1. Config: namespace declaration

- [x] 1.1 Add a `[[workspace]]` table (`name` + `roots`) to the config schema in `internal/config`,
      reusing the tilde/glob expansion used by `[repo].roots` / `[dir].roots`
- [x] 1.2 Validate config: non-empty unique names, at least one root each; surface errors through the
      existing strict config-loading path (`be config --check`)
- [x] 1.3 Document the table in `internal/config/default.toml` with a personal/work example

## 2. Namespace mapping (path-derived)

- [x] 2.1 Add a resolver that maps a repository path to at most one namespace by longest-matching
      configured root (deterministic tie-break), returning none when no root matches
- [x] 2.2 Unit-test the resolver: match, nested-roots longest-match wins, no-match, and that an
      incidental (repo-less) row maps to none

## 3. Upstream row filter

- [x] 3.1 Add a namespace filter applied to the reconciled row set before the lens projections;
      `All` is identity, a named namespace keeps only rows whose repo maps to it, incidental rows
      drop from named namespaces
- [x] 3.2 Ensure both lenses consume the filtered set (single filter point), preserving the
      one-agent-one-row-in-each-lens invariant
- [x] 3.3 Test: named namespace filters both lenses consistently; `All` passes everything; incidental
      agents appear only under `All`

## 3b. Parent-derived automatic workspaces (default-on)

- [x] 3b.1 Assign each recognized repository to a tab: a configured namespace when its path matches
      a root, else an automatic workspace keyed by parent path and labelled by parent base name
- [x] 3b.2 Build the tab list configured-first (fixed) then automatic-sorted-after (live); track the
      active tab by stable key, not index, and fall back to `All` when an active automatic tab empties
- [x] 3b.3 Gate automatic workspaces on at least one configured namespace (inert default preserved);
      incidental agents (no repo) never form one
- [x] 3b.4 Test: siblings share one auto tab, its label/urgency/filtering, and the empty-tab fallback

## 4. Active namespace state + navigation

- [x] 4.1 Add active-tab state to the dashboard model, defaulting to `All`; build the ordered tab
      list (`All` + configured namespaces in config order, then automatic workspaces)
- [x] 4.2 Add rebindable `next-namespace` / `prev-namespace` actions to the agents keymap, default
      `]` / `[`, wrapping across the tab list
- [x] 4.3 On switch, re-filter and refresh lenses + preview; test wrap-around forward and backward

## 5. Tab bar rendering

- [x] 5.1 Render the tab bar as one chrome line at the top; active tab in `theme.Accent` + a
      glyph/bracket (never color alone), no hardcoded colors/glyphs (theme guard)
- [x] 5.2 Account the tab bar in `chromeLines()` so the sidebar + preview never overflow terminal
      height; confirm with the existing height-fit tests
- [x] 5.3 Compute per-tab urgency dots from the unfiltered row set (non-muted `needs-attention`);
      dot non-active tabs only; muted agents do not raise a dot
- [x] 5.4 Render nothing (no tab bar, no height cost) when no namespace is configured

## 6. Rename Workspaces lens → Projects

- [x] 6.1 Rename Workspaces → Projects across `internal/agents` (identifiers, comments, `lensID`
      labels, titles) with no behavior change
- [x] 6.2 Update `default.toml` and `DESIGN.md` prose to say Projects; keep `theme` tokens intact

## 7. Verify end to end

- [x] 7.1 `go test ./...` green
- [x] 7.2 Drive the TUI (use-tty) at wide/narrow/short with a configured namespace: confirm tab bar,
      `]`/`[` switching, urgency dot on a hidden namespace, and inert behavior with none configured
- [x] 7.3 `openspec validate add-dash-namespaces --strict` passes; run repo quality checks
