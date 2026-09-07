---
name: sync-upstream-release
description: Synchronize DevKuAI/devku-sub2api from Wei-Shaw/sub2api main or an exact tag while preserving fork-only behavior. Reserve three-part versions for upstream and use four-part fork releases, mapping upstream vX.Y.Z to fork vX.Y.Z.0. Validate with current CI, and push or publish only when explicitly requested. Use only in this repository for upstream sync, fork release, or post-release verification.
---

# Sync Upstream And Release

Keep the fork current without losing DevKu behavior or reintroducing upstream promotional content. Current code, workflows, remotes, and refs are authoritative; versions, commands, and SHAs from earlier releases are only leads to re-check.

## Route The Request

Resolve one source target before changing the release branch:

- "sync latest" or an unversioned sync targets the fetched `upstream/main`.
- A named three-part upstream version targets the exact namespaced upstream tag `upstream/<upstream-tag>`. Record the source and release versions separately: upstream `v0.2.2` maps to fork baseline `v0.2.2.0`.
- A four-part version names a fork release, not an upstream tag. For a local iteration without a sync request, release the current fork source; do not fetch or merge upstream automatically.
- A release request after a latest-main sync keeps `upstream/main` as the source. Compare it with the upstream version tag and disclose any post-tag commits; do not describe it as an exact-tag sync.
- A post-release verification request is read-only with respect to remote state and checked-out branch or tag refs. Fetching remote-tracking refs and downloading assets into a new temporary directory are allowed. Read [references/release-verification.md](references/release-verification.md) only for publishing or post-release verification.

Authorization is action-specific:

- A sync request permits fetch, local integration, the required local merge commit, and verification. It does not permit push, tag, or release creation.
- `commit` permits only a local commit. `push` without a release request permits only the intended branch push.
- An explicit publish or release request permits the required branch push and creation and push of a new requested tag.
- Force-updating a branch or existing tag, changing package visibility, adding release reactions, or deleting files, worktrees, or branches always requires separate authorization.

Never push to `upstream`. Treat staged, modified, and untracked paths as user work. Fetches may proceed for inspection, but stop before integration when the release branch is dirty or unexpected commits are present.

## Fork Version Policy

- Reserve `vX.Y.Z` for upstream. Synchronizing upstream `v0.2.2` produces fork baseline release `v0.2.2.0`; never publish the customized fork under a new three-part upstream version tag.
- Local iterations increment the fourth component: `v0.2.2.1`, `v0.2.2.2`, and so on. Create a new tag and release for each revision instead of replacing a published one.
- A four-part release tag is not an upstream ref. Its upstream base is the first three parts; never fetch an upstream `v0.2.2.0` or `v0.2.2.1` tag.
- Reset the revision to `0` only when adopting a new upstream base, for example upstream `v0.2.3` maps to fork `v0.2.3.0`.
- Derive a baseline with `sh backend/scripts/release-version.sh --upstream v0.2.2`. It prints `tag=v0.2.2.0` and `version=0.2.2.0` without changing refs. For an unspecified local iteration, inspect both local and origin tags for that base and increment the largest existing fourth component numerically. Do not reuse a pending or published tag.
- If an explicitly requested fork tag already exists, inspect its object and peeled commit before choosing another version or requesting a replacement. Historical three-part fork releases remain compatible; do not rename, delete, or retag them to apply this policy retroactively.
- Update `backend/cmd/server/VERSION` for local source builds. The release workflow uses the selected tag for frontend and backend source, binary version, archives, image tags, and the VERSION sync commit.
- New publication uses `backend/scripts/release-version.sh --fork` and accepts only four-part tags. Three-part versions remain available for historical readback, installation, and rollback.
- Run `bash deploy/tests/release-version-test.sh` and the update-service tests for version changes. Four-part tags use the workflow's existing GoReleaser `--skip=validate` after our version validation; Docker aliases must derive from `.Version`, not `.Major` or `.Minor`.
- Preserve exact versioned artifacts. Moving image aliases (`latest`, major, and major/minor) may advance through the normal release workflow. Changing the version file alone does not authorize publication.

## Establish Current State

Read these surfaces before choosing a strategy:

```bash
git status --short --branch -uall
git rev-parse HEAD
git remote -v
git fetch --prune origin
git fetch --prune upstream
git rev-list --left-right --count main...upstream/main
git log --oneline origin/main..main
git log --oneline upstream/main..main
git log --oneline main..upstream/main
```

Verify by repository owner and name, allowing either SSH or HTTPS transport:

- `origin` is `DevKuAI/devku-sub2api` and `main` tracks `origin/main`.
- `upstream` is `Wei-Shaw/sub2api`.

Also inspect current `backend/go.mod`, root and backend Makefiles, `.github/workflows/backend-ci.yml`, `.github/workflows/security-scan.yml`, `.github/workflows/release.yml`, and `.goreleaser.yaml`. Derive toolchains, checks, artifacts, image tags, and release jobs from these files instead of earlier runs.

When the selected sync source is a named upstream version, synchronize to that exact upstream tag rather than a newer moving `upstream/main`. Keep `<upstream-tag>` (three parts) separate from `<release-tag>` (four parts). Inspect the upstream tag without overwriting the fork tag namespace:

```bash
git ls-remote --tags upstream "refs/tags/<upstream-tag>" "refs/tags/<upstream-tag>^{}"
git fetch upstream "refs/tags/<upstream-tag>:refs/tags/upstream/<upstream-tag>"
git cat-file -t "upstream/<upstream-tag>"
git rev-parse "upstream/<upstream-tag>^{}"
```

If `upstream/<upstream-tag>` already exists locally and differs from the remote, report both tag objects and peeled commits before requesting permission to replace the namespaced ref. If the selected target is `upstream/main`, compare it with the upstream base tag before a fork release:

```bash
git rev-list --count "upstream/<upstream-tag>^{}..upstream/main"
git log --oneline "upstream/<upstream-tag>^{}..upstream/main"
```

Record the selected upstream tag/ref, peeled commit, and separate fork release tag in the final ledger.

## Rehearse And Integrate

Before touching `main`, inventory every fork-only commit and affected path. Rehearse a divergent merge with `git merge-tree --write-tree main <target-ref>` or a temporary worktree. Use the rehearsal to identify conflicts, upstream-only files, generated-code changes, and promotion-only additions.

Choose the smallest correct integration:

- If `main` is an ancestor of the target and has no fork-only commits, use `git merge --ff-only <target-ref>`.
- If both sides contain commits, create a three-way merge into `main`. A fork release tag points to the verified fork release-source commit, which may include the merge and approved cleanup commits, never the upstream commit.
- Stop before changing refs when the target, ancestry, branch ownership, or intended outgoing commits are unclear.

Do not resolve conflicts with blanket `ours` or `theirs`. Read both sides and preserve behavior, not stale text. When upstream provides an equivalent implementation, adapt the fork behavior to the upstream design and retain or update coverage.

Resolve source files before generated files. If Ent schemas, Wire providers, or their consumers change, run `make -C backend generate` after resolving the source and review all regenerated output. Do not hand-maintain generated Ent or Wire code as an independent design surface.

For a non-fast-forward sync, use the repository style:

```text
chore(upstream): sync <tag-or-source>

Merge <selected upstream ref and peeled commit>.
Preserve <concrete fork-only product surfaces>.
Exclude <concrete upstream promotional content or assets>.
```

A fast-forward needs no synthetic sync commit. Before committing a merge, require no unmerged entries, run `git diff --check`, inspect the complete merge against both parents, and confirm every new or removed file is intentional.

## Preserve DevKu Surfaces

Build the preservation inventory from current fork-only commits rather than relying only on this list. Unless the user requests a behavior change, preserve:

- Desktop organizations, members, authentication and refresh flow, managed member API keys, Admin APIs, Vue management UI, feature flags, configuration, migration, documentation, and tests;
- per-user API key creation limits and their migration, service, API, Admin UI, and tests;
- DevKu repository, update, install, container image, and documentation URLs, while retaining the explicit upstream repository attribution link;
- DevKu branding, curated multilingual README content, and blue theme;
- fork-specific release workflow, container registries, and publishing configuration.

Keep `README.md`, `README_CN.md`, and `README_JA.md` aligned. Port useful upstream functional documentation deliberately, including new behavior, configuration, compatibility notes, and security notices. Do not replace the curated README files wholesale.

Exclude upstream sponsor, donation, affiliate, referral, community-recruitment, hosted-service marketing, ranking badge, and similar promotional sections or links unless the user explicitly asks to include them. Exclude promotional-only partner images and other assets that have no functional consumer. Preserve neutral upstream attribution. If filtering requires deleting a tracked path, stop and request the deletion authorization required by the repository instructions.

Audit the merge result and outgoing diff for promotional additions, changed DevKu URLs, and orphaned assets. Do not rely only on conflict files: upstream promotion commits may merge cleanly.

## Verify With Current CI Parity

Run the checks defined by the fetched workflows. At minimum:

- every command in the current shell-check job;
- `env -u OPENAI_API_KEY make -C backend test-unit` and `env -u OPENAI_API_KEY make -C backend test-integration`;
- the workflow's frozen frontend install, lint, typecheck, complete Vitest suite, and production build;
- golangci-lint at the exact workflow version;
- `govulncheck ./...` in `backend` and the repository pnpm audit-exception check;
- `make -C backend generate` followed by a generated-diff review when generator inputs changed;
- a clean committed release build whose `-version` output and `go version -m` revision match the intended fork release source when publishing is planned.

An injected `OPENAI_API_KEY`, wrong toolchain, registry timeout, unavailable Docker daemon, or missing local service is an environment failure until the intended path proves otherwise. Do not change source or lockfiles to hide it. Re-read `git status` after generators and package-manager commands, and account for every change.

## Push Or Publish Only When Authorized

Immediately before any remote write, re-read `HEAD`, full status, tracking refs, `origin/main..HEAD`, and the authenticated GitHub identity. Every outgoing commit must be intended for the current request. Push the branch first, then require successful terminal CI and Security Scan states for the pushed source commit.

For an authorized version release:

1. Compare the local and `origin` four-part release tag independently of the namespaced three-part upstream source tag. For example, release `v0.2.2.0` uses source `upstream/v0.2.2`. Never overwrite an existing fork tag without confirmation.
2. Create a new annotated fork tag at the verified clean release source. Reuse the upstream annotation only when it accurately describes the selected source; otherwise write fork-accurate release text.
3. Push only the requested new tag. Poll tag CI, Security Scan, and every Release workflow job as structured state.
4. Complete [references/release-verification.md](references/release-verification.md), including assets, checksums, binary metadata, registries, and any workflow-created VERSION commit.

Finish with the selected upstream source and peeled commit, fork release-source commit, exact local and origin branch SHAs, tag object and peeled commit, CI state, release URL, artifact and registry evidence, promotion audit result, remaining verification gaps, and worktree status. Never claim that a fork tag is source-identical to the upstream tag when it contains fork commits or post-tag upstream commits.
