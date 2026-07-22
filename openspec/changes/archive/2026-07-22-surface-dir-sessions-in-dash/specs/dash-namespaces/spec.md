## ADDED Requirements

### Requirement: Non-git directory sessions participate in namespaces by path

An open non-git directory session (a directory row - see agent-view) SHALL participate in
namespace membership judged by its **own directory path**, the same path-derived rule that
governs repositories: it belongs to the configured namespace whose root most specifically
contains its path, and when it matches no configured root it falls back to an automatic
parent-derived workspace rather than being confined to All. This distinguishes a directory
session, which is a durable location the user opened, from an incidental agent, which has no
directory identity and SHALL continue to appear only under All.

#### Scenario: A directory session under a configured root files under that namespace

- **WHEN** the feature is enabled and an open non-git directory session's path lies under a
  configured namespace's root
- **THEN** its directory row appears when that namespace's tab is active, alongside the
  namespace's repositories, not only under All

#### Scenario: An unmatched directory session forms a parent-derived tab

- **WHEN** an open non-git directory session's path is under no configured namespace root
- **THEN** it belongs to an automatic workspace derived from its parent directory, reachable as
  its own tab, rather than appearing only under All

#### Scenario: Incidental agents remain All-only

- **WHEN** a live agent resolves to no repository and has no directory row (an incidental agent)
- **THEN** it still appears only under All, because it has no directory path to judge membership by
