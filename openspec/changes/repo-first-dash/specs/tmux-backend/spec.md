## MODIFIED Requirements

### Requirement: Idempotent session creation

The system SHALL create a tmux session for a given name and working directory only if a session with
that name does not already exist, reusing the existing session otherwise. For be-created (managed)
sessions, the session **name SHALL be a deterministic function of the repository's identity** (its
`git-common-dir`), using the repository basename as the friendly part and **disambiguating on
collision** so that two repositories sharing a basename receive distinct session names. Determinism
SHALL be such that ensuring the session for a given repository always resolves to the same name, so
spawning into a repository's home is reliably idempotent.

#### Scenario: Create when absent

- **WHEN** the backend is asked to ensure a session named `X` and no such session exists
- **THEN** it creates a new detached session `X` with the requested working directory

#### Scenario: Reuse when present

- **WHEN** the backend is asked to ensure a session named `X` and session `X` already exists
- **THEN** it does not create a duplicate and targets the existing session

#### Scenario: Repository session name is deterministic and collision-safe

- **WHEN** two distinct repositories share a basename and each is ensured a home session
- **THEN** each resolves to a distinct, stable session name derived from its repository identity, so
  they do not collide into one shared session

#### Scenario: Ensuring the same repository twice resolves to one session

- **WHEN** the same repository's home session is ensured on two separate occasions
- **THEN** both resolve to the same session name, reusing the existing session on the second
