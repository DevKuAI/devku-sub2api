---
name: sync-upstream-release
description: Synchronize this sub2api fork with Wei-Shaw/sub2api while preserving DevKu customizations, validate against the current repository CI, and publish the matching upstream version only when explicitly requested. Use only in this repository for upstream sync, same-version release, or post-release verification.
---

# Sync Upstream And Release

Keep the fork current without losing its product-specific behavior. Treat the current repository, workflows, and remote refs as source of truth; versions and SHAs from previous runs are historical only.

## Boundaries

- Verify that `origin` is the DevKu fork and `upstream` is the Wei-Shaw source before any fetch, merge, or push. Never push to `upstream`.
- A sync request authorizes fetching, merging, and local verification. It does not authorize pushing. An explicit publish request covers the branch and new tag pushes required by this repository's release workflow.
- Force-updating a branch or same-name tag, changing package visibility, adding release reactions, or deleting worktrees/branches always needs separate authorization.
- Treat staged, modified, and untracked files as user work. Do not stash, clean, reset, switch away from, or overwrite them. Stop before a mutating sync when the release branch is not clean.

## Establish Current State

Read these surfaces before choosing a strategy:

```bash
git status --short --branch -uall
git remote -v
git fetch --prune origin
git fetch --prune upstream
git rev-list --left-right --count main...upstream/main
git log --oneline upstream/main..main
git log --oneline main..upstream/main
```

Also read the current `backend/go.mod`, root and backend Makefiles, `.github/workflows/backend-ci.yml`, `.github/workflows/security-scan.yml`, `.github/workflows/release.yml`, and `.goreleaser.yaml`. Do not reuse old Go, Node.js, pnpm, golangci-lint, archive, or image settings.

When the user names a release version, synchronize to that exact upstream tag rather than a newer moving `upstream/main`. Inspect the remote tag without overwriting the fork tag namespace:

```bash
git ls-remote --tags upstream "refs/tags/<tag>" "refs/tags/<tag>^{}"
git fetch upstream "refs/tags/<tag>:refs/tags/upstream/<tag>"
git cat-file -t "upstream/<tag>"
git rev-parse "upstream/<tag>^{}"
```

If the namespaced upstream tag already exists but differs from the remote, report both tag objects and peeled commits before requesting permission for any forced ref update.

## Choose The Integration Strategy

- If the fork release branch is an ancestor of the selected upstream ref and has no fork-only commits, use `git merge --ff-only`.
- If both sides contain commits, use a three-way merge into the fork release branch. A same-version fork release tag must point to the resulting fork commit, not the upstream commit.
- If ancestry, target version, or branch ownership is unclear, stop before modifying refs and report the graph.

Before resolving a divergent merge, inventory fork-only commits and their affected files. Preserve the current behavior of these known fork surfaces unless the user requests otherwise:

- per-user API key creation limits and their migration, API, admin UI, and tests;
- DevKu repository, update, install, container image, and documentation URLs;
- DevKu branding, curated README content, and blue theme;
- fork-specific release workflow and container publishing configuration.

Do not resolve conflicts with blanket `ours` or `theirs`. Read both implementations and preserve behavior rather than stale lines. If upstream now implements equivalent behavior, adapt the fork customization to the new design and keep coverage. After resolving, inspect `git diff --check`, unmerged entries, and the merge result before committing.

For a non-fast-forward sync, use the repository commit style:

```text
chore(upstream): sync <tag-or-version>

Merge the selected upstream release while preserving the fork-specific behavior that remains necessary.
```

Name the concrete preserved surfaces in the body. A fast-forward needs no extra commit.

## Verify With Current CI Parity

Run the checks defined by the current workflows, not remembered commands. At minimum:

- all shell checks from the `shell` CI job;
- `env -u OPENAI_API_KEY make -C backend test-unit` and `env -u OPENAI_API_KEY make -C backend test-integration`;
- frontend frozen install using the workflow's pnpm version, then lint, typecheck, full Vitest, and production build;
- golangci-lint using the exact version in the current workflow;
- `govulncheck ./...` in `backend` and the repository pnpm audit-exception check;
- a tagged/local build whose `-version` output matches the requested release version when a release is planned.

An injected `OPENAI_API_KEY`, wrong Go toolchain, wrong pnpm major, registry timeout, or missing local service is an environment failure until the intended test path proves otherwise. Do not change application code or lockfiles to mask it. Re-read `git status` after any package-manager command and account for every change.

## Push Or Publish Only When Authorized

Immediately before a push, re-read `HEAD`, status, tracking refs, and the authenticated GitHub identity. Push the synchronized branch first and wait for its CI and Security Scan to reach successful terminal states.

For a requested same-version release:

1. Compare local, `origin`, and namespaced `upstream` tag objects and peeled commits.
2. Create a new annotated fork tag at the verified fork sync commit, preserving the upstream annotation content. Never overwrite an existing same-name ref without confirmation.
3. Push only that tag and monitor the tag CI, Security Scan, and Release workflow as structured state.
4. Read and complete [references/release-verification.md](references/release-verification.md).

Finish with exact local/origin branch SHAs, tag object and peeled commit, CI state, release URL, artifact and registry evidence, remaining verification gaps, and clean-worktree status.
