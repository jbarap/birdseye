---
name: use-tty
description: Render and inspect birdseye's TUI (be dash, the picker) in a real terminal by driving a dedicated detached tmux session and capturing its screen. Use whenever a layout/rendering change needs to be seen, not reasoned about - column widths, the agents/preview split, glyph rendering, truncation, colors.
metadata:
  author: birdseye
  version: "1.0"
---

Inspect birdseye's TUI by rendering it in a real terminal. Bubble Tea uses the alternate
screen and reads the real pty size, so layout bugs (dead gaps, clipped columns, the
agents/preview split) only show up in an actual terminal - never by reasoning about the code.
Drive a **dedicated, detached** tmux session and read its screen with `capture-pane`.

**Never touch the user's own sessions.** Use a throwaway session named `_birdseye_dev` and
nothing else. Do not attach to, resize, send keys to, or kill any other session.

## Workflow

1. **Build the binary** (so you render the current code, not a stale install):
   ```bash
   go build -o ./bin/be ./cmd/be
   ```

2. **Create a fresh detached session at a chosen size.** Recreate it each time rather than
   reusing - the relaunch-in-place cycle (`C-c` then re-run) is racy and often captures the
   shell instead of the app. The size is the pty the TUI lays out against; pick the case you
   want to test (a wide terminal to see the split, a narrow one to see truncation):
   ```bash
   tmux kill-session -t _birdseye_dev 2>/dev/null
   tmux new-session -d -s _birdseye_dev -x 200 -y 45 -c "$(pwd)"
   tmux send-keys -t _birdseye_dev "./bin/be dash" Enter
   ```

3. **Capture the rendered screen in a *separate* tool call** (the app needs a moment to paint;
   `direnv`/shell startup adds latency, so do not capture in the same call that launches it):
   ```bash
   tmux capture-pane -p -t _birdseye_dev
   ```
   `-p` prints to stdout. The captured frame is exactly what the user sees. Read it as the
   ground truth - if a column is clipped or a gap is dead space, you will see it here.

4. **Test other sizes** by recreating the session with different `-x`/`-y` (recreating is more
   reliable than `resize-window` for a detached session, which may not deliver `SIGWINCH`).
   Confirm the rendered width with `tmux display -t _birdseye_dev -p "#{window_width}"`.

5. **Iterate**: edit code, `go build`, recreate the session, capture. Repeat until it looks
   right at the sizes that matter.

6. **Clean up** when done so no stray session lingers:
   ```bash
   tmux kill-session -t _birdseye_dev 2>/dev/null
   ```

## Notes

- The dash renders real state - live tmux panes, recognized repos, and agent hook records
  under `$XDG_STATE_HOME/birdseye/agents`. On a machine with active agents this gives a rich,
  realistic frame for free. With no agents/repos it shows the empty state.
- Nerd-font glyphs (`󱘎`, `⌂`, `◌`, the cursor `󰁕`) render by the terminal's font. Per
  DESIGN.md, judge glyph rendering by `cat`-ing a sample in a raw terminal, not inside the TUI.
- Colors survive capture only with `tmux capture-pane -e` (include escape sequences); the plain
  `-p` form is best for reading layout (widths, alignment, truncation).
- To exercise an interaction (fold, navigate, open a modal), `tmux send-keys -t _birdseye_dev
  <keys>` then capture again - e.g. `send-keys -t _birdseye_dev j` to move down, `Tab` to fold.
