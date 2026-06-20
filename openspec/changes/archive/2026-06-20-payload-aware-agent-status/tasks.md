## 1. Verify the live payload

- [x] 1.1 Capture a real Claude Code `Notification` hook payload from stdin for both the idle
      "waiting for your input" nudge and a permission prompt; record the exact `message` strings so
      the substring match targets the real idle phrasing and nothing else (design Decision 3).
      Confirmed by the user against a live payload: the `waiting for your input` substring matches
      the idle nudge and not the permission prompt.

## 2. Payload-aware status derivation

- [x] 2.1 Add `Message string \`json:"message"\`` to `hookInput` in `internal/agents/claude.go`.
- [x] 2.2 Replace `StatusForEvent(event string)` with `StatusFor(event string, in hookInput) Status`;
      keep the existing event→status switch and add the `Notification` branch that downgrades the
      idle message (case-insensitive substring on the verified phrase) to `StatusIdle`, leaving all
      other notification messages — including unrecognized/empty — as `StatusNeedsAttention`.
- [x] 2.3 Update `HandleHook` to call `StatusFor(event, in)` with the parsed payload; update the
      doc comment to describe the payload-aware mapping.

## 3. Tests

- [x] 3.1 Add `claude_test.go` cases: idle message → idle, permission message → needs-attention,
      unrecognized/empty message → needs-attention, and a non-`Notification` event (e.g. `Stop`,
      `SessionStart`) unchanged. Note the idle-phrase dependency in the test so a future Claude
      wording change is discoverable.
- [x] 3.2 Run `just check` (vet + `go test ./...`) and confirm green.

## 4. Docs

- [x] 4.1 In `README.md`, annotate the `Notification` row of the event→status table to note it
      splits by message (idle nudge → idle; permission/unrecognized → needs-attention).
