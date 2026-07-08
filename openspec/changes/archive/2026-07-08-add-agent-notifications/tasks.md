## 1. Notifier seam and delivery

- [x] 1.1 Add `internal/agents/notify.go` with a `Notification` struct (`Agent`, `Status`, `Repo`,
  `CWD`, `Message`) and a `Notifier` interface (`Notify(Notification) error`).
- [x] 1.2 Implement the auto-detect notifier: pick the first available of `notify-send` (Linux) and
  `osascript` (macOS) from `PATH`; fall back to a terminal bell, then no-op, when none is present.
  Detection/resolution failures return without erroring so callers can ignore them.
- [x] 1.3 Implement the command-template notifier: run the configured `command` with `BE_AGENT`,
  `BE_STATUS`, `BE_REPO`, `BE_CWD`, `BE_MESSAGE` exported in its environment.
- [x] 1.4 Add a constructor that returns the command notifier when `command != ""`, else the
  auto-detect notifier.

## 2. Config: `[agents.notify]`

- [x] 2.1 Add a `Notify` struct (`Command string`, `NeedsAttention bool`, `Finished bool`) nested on
  the `Agents` config struct with `toml:"notify"` / `command` / `needs_attention` / `finished` tags.
- [x] 2.2 Seed defaults where the other `Agents` defaults are applied: absent `[agents.notify]` and
  absent booleans resolve to `needs_attention = true`, `finished = true`, empty `command`. Ensure an
  explicit `false` in user TOML still wins over the seeded true.
- [x] 2.3 Confirm strict loading rejects an unknown key under `[agents.notify]` (no code change if
  the strict loader already covers nested tables; add a test asserting it).
- [x] 2.4 Document the `[agents.notify]` keys in `internal/config/default.toml` under the `[agents]`
  section (commented, showing the defaults), matching the file's existing reference style.

## 3. Emit from the hook path

- [x] 3.1 In `HandleHook` (`internal/agents/claude.go`), capture `old := rec.Status` before
  `rec.Status = StatusFor(event, in)` overwrites it (guard for the `SessionEnd` branch that writes no
  status).
- [x] 3.2 Add an edge classifier: `into needs-attention` = `new == StatusNeedsAttention && old !=
  StatusNeedsAttention`; `finished` = `old == StatusWorking && new == StatusIdle`. Everything else is
  no edge.
- [x] 3.3 After the record is written, and only for a qualifying edge whose config key is enabled,
  build the `Notification` (title from `rec.Title`, `Status` = new, `CWD` = `rec.CWD`, `Repo`
  best-effort/empty, a human `Message`) and dispatch it. Wrap all of it so any error is discarded and
  the hook still returns nil.
- [x] 3.4 Skip emission when the agent's location is muted: compute `locationKey(rec.TmuxPane,
  rec.TmuxSession, rec.TmuxWindow, rec.SessionID)` and check `readMutes(dir)`; a muted location emits
  nothing.
- [x] 3.5 Load the effective config inside the hook (best-effort; on load error, fall back to the
  built-in defaults so notifications still work).

## 4. Tests

- [x] 4.1 Edge classifier + emission: table test over (old, new) pairs asserting only `into
  needs-attention` and `working -> idle` emit, and that already-blocked re-fires and start-of-work do
  not, using a fake `Notifier` that records calls.
- [x] 4.2 Mute suppression: a muted location emits nothing on either edge; unmuting re-enables.
- [x] 4.3 Config gating: `needs_attention = false` / `finished = false` suppress only their edge;
  absent table notifies on both.
- [x] 4.4 Best-effort: a `Notifier` that returns an error (and the no-notifier fallback path) leaves
  `HandleHook` returning nil and the record written.
- [x] 4.5 Command notifier: assert the env-var contract (`BE_AGENT`/`BE_STATUS`/`BE_REPO`/`BE_CWD`/
  `BE_MESSAGE`) is passed, using a stub command that captures its environment.
- [x] 4.6 Strict-config test: an unknown key under `[agents.notify]` is rejected.

## 5. Verify

- [x] 5.1 `go build ./...` and `go test ./...` pass.
- [x] 5.2 Drive it end to end via the use-tty workflow: install hooks, trigger a real
  needs-attention and a real Stop in a `_birdseye_dev` agent, and confirm a notification fires with
  the dash closed and is suppressed when the agent is muted.
