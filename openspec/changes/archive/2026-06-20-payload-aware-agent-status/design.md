## Context

`HandleHook` (`internal/agents/claude.go`) parses only `session_id` and `cwd` from the hook JSON,
then calls `StatusForEvent(event string)` — a flat switch on the event name. The full payload is
discarded. The one event where the name is genuinely ambiguous is `Notification`: Claude Code fires
it both for **permission/approval prompts** (a real block on the user) and for its periodic idle
nudge (~60s after Claude goes quiet, "Claude is waiting for your input"). Today both become
`needs-attention`, so an agent that is merely idle turns red and sorts to the top — a false alarm
the user has to mentally filter out.

The hook payload distinguishes them: `Notification` carries a `message` string. Everything needed to
fix this is already on stdin; we just aren't reading it.

## Goals / Non-Goals

**Goals:**
- Derive status from the parsed hook payload, not the event name alone.
- Stop the idle "waiting for your input" notification from showing as needs-attention; map it to
  `idle` instead.
- Keep permission prompts — and any notification message we don't recognize — as `needs-attention`.
- Leave every other event's mapping, the store schema, the installed hooks, and the view untouched.

**Non-Goals:**
- No new hook events and no change to `ManagedEvents` (we don't start installing `PreToolUse` etc.).
- No new record/`Agent` field and no UI change to display a "reason"/"detail" string. The payload
  could feed a richer per-agent detail later, but that is a separate change (see Open Questions).
- No config surface for the classification; the message rules are built in.
- No status derived from pane contents — unchanged; the payload is the only new input.

## Decisions

**1. Payload-aware signature: `StatusFor(event string, in hookInput) Status`.**
Replace `StatusForEvent(event string)` with a function that also receives the already-parsed
payload. `HandleHook` parses the JSON once (as it does now) and passes it in. Rationale: keeps a
single, table-driven, unit-testable derivation point; the event name stays the primary key and the
payload only refines the few cases that need it. Alternative — threading raw `[]byte` and decoding
inside the mapper — was rejected: it duplicates JSON parsing and makes the function harder to test.

**2. Classify `Notification` by message, downgrade-only.**
Only a recognized idle phrase downgrades to `idle`; every other message (permission prompts,
anything unfamiliar, empty) keeps `needs-attention`. Concretely: case-insensitive substring match on
`"waiting for your input"` → `idle`; else → `needs-attention`. Rationale: conservative by
construction — a notification we don't understand still reaches the user, so the failure mode is "an
idle agent occasionally shown as needs-attention" (today's behavior), never "a real block silently
hidden". Alternative — enumerating permission-message variants to *promote* to needs-attention — was
rejected as brittle and backwards (it would default unknown messages to idle, hiding real blocks).

**3. Match on a substring, not an exact string.**
Claude's notification copy is not a stable API and may vary ("Claude is waiting for your input",
possibly with a trailing duration). A narrow, lowercase substring (`waiting for your input`) is
resilient to surrounding wording while specific enough not to match permission prompts. The exact
phrase MUST be confirmed against a real payload during implementation (capture one `Notification`
event's stdin) rather than trusted from memory; if the live phrasing differs, the substring is
adjusted to match the observed idle nudge and nothing else.

**4. `hookInput` gains `Message string \`json:"message"\``.**
Smallest possible payload surface — one field, only consumed for `Notification`. No store/schema
change, since status is already persisted and the message itself is not retained.

## Risks / Trade-offs

- **[Wrong idle phrase → no behavior change or, worse, a missed block.]** If the substring is too
  broad it could match a permission message and hide a real block. → Keep the phrase narrow and
  downgrade-only; verify against a captured payload (Decision 3). A too-narrow phrase merely
  preserves today's behavior (idle nudge stays red) — a safe failure.
- **[Claude changes its notification copy later.]** A future Claude release could reword the idle
  nudge, silently reverting to today's behavior. → Acceptable: the regression is cosmetic (idle
  shown as needs-attention), self-evident, and a one-line substring fix. Document the dependency in
  the test so it's discoverable.
- **[Scope temptation.]** Reading the JSON invites parsing tool names, `source`, `reason`, etc. →
  Explicitly out of scope here; this change does one thing.

## Open Questions

- Should the recognized notification `message` (and/or the active tool from `PreToolUse`) be captured
  into the record and surfaced in the agents view as a short "why/what" detail? Deferred to a
  follow-up; it touches the store schema and the view, which this change deliberately leaves alone.
