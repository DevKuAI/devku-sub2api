# Release Verification

Read this reference only after the user explicitly requests publishing or post-release verification. Derive names, target platforms, registries, and version behavior from the current `.github/workflows/release.yml` and `.goreleaser.yaml` before running the checklist.

## Workflow And Version State

Poll GitHub Actions with structured output such as `gh run view <id> --json status,conclusion,jobs`; do not treat streamed watch output or a successful tag push as release completion.

Require successful terminal states for:

- branch CI and Security Scan at the fork sync commit;
- tag CI and Security Scan;
- every Release workflow job, including `sync-version-file` when present.

The release workflow may commit `backend/cmd/server/VERSION` back to the default branch with `[skip ci]`. After the workflow completes:

1. Fetch `origin` and inspect the new commit.
2. Confirm it is a direct successor of the pushed sync commit and changes only the expected version file.
3. Fast-forward the local release branch with `git merge --ff-only origin/main`.
4. Report that no new checks are expected when `[skip ci]` suppresses them; verify the actual check-run list instead of assuming.

## Release Metadata And Assets

Read the published release back with `gh release view`. Confirm the exact tag, title, stable/prerelease state, publication time, URL, target, and release body. The public tag object must peel to the fork sync commit even when the release reports the default branch as `targetCommitish`.

Download assets into a new `mktemp -d` directory. Do not delete that directory without authorization. Validate beyond names and sizes:

- expected archive count and checksum file from the current GoReleaser config;
- every checksum with `shasum -a 256 -c` or the platform equivalent;
- archive entries include the binary and all configured README, LICENSE, and deploy files;
- no cache, log, screenshot, or other unintended packaging noise;
- binary format matches the archive OS and architecture;
- `go version -m` reports the intended GOOS, GOARCH, and fork sync revision;
- each binary contains the requested embedded version, and every locally runnable binary returns that version from `-version`.

Cross-compiled binaries that cannot run on the host still need format, build metadata, revision, and embedded-version checks. Do not call file-size inspection a binary smoke test.

## Container Registries

Inspect every version and moving tag that the current GoReleaser config publishes. For this repository that normally includes versioned and `latest` manifests on GHCR and Docker Hub, but the config remains authoritative.

Use `docker buildx imagetools inspect` or an equivalent registry API to confirm:

- the version tag exists;
- `latest` points to the intended release when the config updates it;
- all configured platforms exist, normally `linux/amd64` and `linux/arm64`;
- version and moving tags use the expected child digests and manifest digest.

Registry publication and package visibility are separate. Do not change visibility during a release unless the user explicitly requests it.

If direct registry readback fails, distinguish a registry publication failure from local DNS, TLS, authentication, or network failure. Retry only when the failure is plausibly transient. Completed GoReleaser logs showing image and manifest digests are useful fallback evidence, but report the missing anonymous readback as a verification gap.

## Final Ledger

Re-read all of the following before declaring the release complete:

```bash
git status --short --branch -uall
git rev-parse HEAD origin/main
git ls-remote origin refs/heads/main "refs/tags/<tag>" "refs/tags/<tag>^{}"
```

The completion report must separate:

- synchronized source and fork customizations;
- branch and tag refs;
- CI and Security Scan;
- GitHub Release metadata and downloaded assets;
- GHCR and Docker Hub state;
- any environment-only verification gap.

Do not add GitHub release reactions or perform unrelated public follow-through unless the current request asks for it.
