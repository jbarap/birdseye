## Why

`be dash` laid its three surfaces out in a single horizontal row: the Agents lens, the Workspaces
lens, and the preview all shared the terminal width. That split the width three ways between
surfaces that do not scale the same. The two lenses are narrow, near-fixed-width lists (a status
gutter plus a label and title column) - they gain nothing from extra width. The preview is a
capture of a real pane, and it is the only surface whose usefulness grows with both the width and
the height it gets. Dividing the width evenly gave the lenses room they could not use while
starving the one surface that wanted it, and forced machinery whose only job was to shrink the
lens columns so the preview kept a horizontal minimum.

A first attempt stacked the two lenses above a full-width preview. That widened the preview but
wasted the top: the two narrow lenses sat side by side and left roughly half the width empty to
their right. The fix is the conventional master-detail arrangement: pin the fixed-size lists in a
left **sidebar** and let the one flexible surface, the preview, absorb all the slack. Stacking the
lenses vertically shrinks the column they need (one lens wide, not two), so the preview gets more
width *and* keeps full height, and there is no dead gap.

## What Changes

- Recompose the dashboard body into a **sidebar + preview**: the Agents lens stacked **above** the
  Workspaces lens in a fixed-width left column, and the **preview filling the rest of the width at
  full height** on the right.
- **Move lens focus-switching to a vertical key pair.** The lenses are stacked, so left/right no
  longer describes moving between them. `ctrl+k` focuses up (Agents), `ctrl+j` focuses down
  (Workspaces), matching the existing `ctrl+u`/`ctrl+d` "bigger vertical movement" convention.
  `h`/`l` and the arrows stay bound as aliases so muscle memory does not break.
- **`agents.split` returns to a width ratio**: the lens sidebar's share of the terminal width, the
  preview taking the rest. (This restores its original meaning.) The key, default, and validation
  are unchanged.
- **Responsive fallbacks by axis**: a terminal too narrow to seat a usable preview beside the
  sidebar drops the preview and gives the sidebar the full width; a terminal too short to stack
  both lens panels shows only the focused lens at full height (still switchable with the focus
  keys). Both lenses stack whenever the height allows - there is no width-based single-lens mode.

## Capabilities

### Modified Capabilities

- `dash-lenses`: the two lenses are stacked in a left sidebar (Agents above Workspaces) with the
  preview filling the remaining width at full height, rather than sharing one horizontal row;
  lens focus switches on a vertical key pair; `agents.split` is the sidebar's width share; the
  preview drops on a too-narrow terminal and the sidebar collapses to the focused lens on a
  too-short one.

## Impact

- `internal/agents/view.go`: `View` joins the sidebar beside a full-height preview; `renderLenses`
  stacks the two lens panels (each forced to its share of the height) with a focused-lens fallback
  when the terminal is too short; new width helpers (`sidebarWidth`/`sidebarContentWidth`) and
  height helpers (`sidebarBodyRows`/`agentsPaneRows`/`workspacePaneRows`/`stackLenses`) replace the
  side-by-side lens-fit machinery (`dualLens`, `agentsReserve`/`mainWidth` as preview reservation,
  `listCap`, `agentsContentWidth`); `renderPreview` sizes to the leftover width and full height;
  `showPreview` gates on width.
- `internal/agents/keymap.go`: `ctrl+k`/`ctrl+j` become the primary focus keys (`h`/`l`/arrows kept
  as aliases); the help hint renders them as `^k`/`^j`.
- `internal/config/config.go`, `internal/config/default.toml`: re-document `agents.split` as the
  sidebar's share of the width. No key/default/validation change.
- No new config key, no new CLI verb, no theme/design-token change.

## Non-Goals

- No change to what the lenses contain, how rows are sourced/ordered/folded, or how focus and
  linked selection work - only where the surfaces sit and how focus moves between the stacked lenses.
- No configurable preview placement (side vs bottom) and no configurable sidebar-internal split
  (Agents vs Workspaces height is derived from content). The layout is the single sidebar
  arrangement.
- No change to preview capture, the placeholder text, or the status/help/notice chrome.
