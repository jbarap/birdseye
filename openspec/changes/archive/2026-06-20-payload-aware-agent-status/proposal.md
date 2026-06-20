## Why

`be agents` derives an agent's status purely from the *name* of the Claude Code hook event
(`StatusForEvent(event string)`), discarding the rest of the hook JSON. The biggest cost of that
is the `Notification` event: it always becomes **needs-attention**, even when it is just Claude's
periodic "waiting for your input" idle nudge rather than a real permission block. The result is an
idle agent wrongly turning red and floating to the top of the view. The hook payload already
carries the information needed to tell these apart (the notification `message`), so we can map more
accurately by reading it.

## What Changes

- Status derivation becomes **payload-aware**: the mapping reads the parsed hook JSON, not just the
  event name. The event name remains the primary signal; the payload only refines it where it adds
  real accuracy.
- `Notification` is classified by its `message`: Claude's idle "waiting for your input" nudge maps
  to **idle**, while permission/approval prompts (and any message we don't recognize) stay
  **needs-attention**. This is conservative — only the known idle phrase downgrades; everything
  unrecognized still surfaces to the user.
- All other events keep their current status mapping; the change is additive and introduces no new
  hook events, no new managed events, and no config.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `agent-view`: the "Claude Code status via hooks" requirement is refined so status MAY be derived
  from the hook payload (not the event name alone), with the `Notification` idle-vs-attention
  distinction as the defined behavior.

## Impact

- `internal/agents/claude.go`: `StatusForEvent(event)` becomes payload-aware (new signature taking
  the parsed payload); `hookInput` gains the `message` field; `HandleHook` passes the payload to the
  derivation.
- `internal/agents/claude_test.go`: tests for the new classification (idle nudge → idle, permission
  prompt → needs-attention, unrecognized → needs-attention, non-Notification events unchanged).
- `README.md`: the event→status table gains a note that `Notification` splits by message.
- No change to the store schema, the installed hooks, `ManagedEvents`, the agents view, or config.
