# Releasing

This document is for maintainers. Users should select an immutable version from [GitHub Releases](https://github.com/premday/sonic-exporter/releases) and follow the appropriate deployment guide.

## Release trigger

The release workflow runs when a tag matching `v*` is pushed. Create an annotated semantic-version tag only after the default-branch CI is green:

```bash
git switch master
git pull --ff-only
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

Do not move or reuse a published tag for different content.

## What the workflow checks

Before publishing, `.github/workflows/release.yml` runs:

- the Go test suite;
- `govulncheck`;
- GoReleaser;
- SBOM generation through Syft;
- provenance attestations for checksums and release archives.

The normal pull-request workflow additionally runs race-enabled tests, dashboard validation, `golangci-lint`, Gitleaks, and a Docker image smoke test. A release should not be cut to bypass a failing default-branch check.

The image smoke test must launch the default Supervisor command and verify both `bash -c "supervisorctl status"` and `/metrics`; directly launching the exporter would miss the SONiC FEATURE integration. `Dockerfile` and `supervisord.conf` are the release image sources, and GoReleaser includes the configuration in its build context. No locally prebuilt base image is required.

For container changes, also scan the final image's OS and Python packages with an approved image-vulnerability scanner. `govulncheck` covers Go vulnerabilities, not the Bash/Supervisor runtime dependencies. Record the scanner, image ID, findings, and any validation gaps in the PR; do not treat a successful build as security validation.

## Published artifacts

GoReleaser currently publishes Linux/amd64 artifacts:

| Artifact | Purpose |
|---|---|
| `sonic-exporter_X.Y.Z_linux_amd64.tar.gz` | Advanced standalone binary deployment on SONiC Community OS |
| `checksums.txt` | SHA-256 verification for release files |
| SBOM JSON files | Software-bill-of-materials data for release archives |
| `ghcr.io/premday/sonic-exporter:vX.Y.Z` | Recommended Docker deployment on SONiC Community OS |
| `ghcr.io/premday/sonic-exporter:latest` | Convenience tag for the newest stable release; not recommended for production pinning |

The repository does not publish `.deb` packages.

Do not confuse these three artifact types:

1. The GitHub release archive contains the standalone binary.
2. The GHCR reference names the container image in the registry.
3. A file produced by `docker save` is an offline copy of that container image.

They are not interchangeable.

## Verify a release

Download the archive and `checksums.txt` from the same release. The checksum file also lists the release SBOM, so select the archive entry when the SBOM was not downloaded:

```bash
VERSION=X.Y.Z
ARCHIVE="sonic-exporter_${VERSION}_linux_amd64.tar.gz"

awk -v archive="${ARCHIVE}" \
  '$2 == archive {line = $0; matches++} END {if (matches != 1) exit 1; print line}' \
  checksums.txt \
  | sha256sum -c -
```

When GitHub attestations are present:

```bash
gh attestation verify sonic-exporter_X.Y.Z_linux_amd64.tar.gz \
  --repo premday/sonic-exporter
gh attestation verify checksums.txt \
  --repo premday/sonic-exporter
```

Inspect the published container image before rollout:

```bash
RELEASE_TAG=vX.Y.Z

docker pull ghcr.io/premday/sonic-exporter:${RELEASE_TAG}
docker image inspect ghcr.io/premday/sonic-exporter:${RELEASE_TAG} \
  --format 'ID={{.Id}} RepoDigests={{json .RepoDigests}}'
```

## Release notes

GitHub Releases are the canonical release history. Keep the release description focused on user-visible changes:

- new or changed metric families;
- default changes;
- required deployment changes;
- compatibility or migration notes;
- security fixes;
- known limitations.

A manually maintained `CHANGELOG.md` should be added only if the project commits to updating it for every release or generates it automatically. Maintaining both ad hoc release notes and a manual changelog creates two sources of truth.

### v0.7.0 migration notes

The release description must call out the new Bash/Supervisor runtime, continued non-root execution, and the startup-command change. Users must remove direct exporter command overrides to enable Supervisor; custom listener settings move to the container environment. Include the read-only udev mount, the expected permission-protected Unix-socket authentication warning, and the need to check the exporter process and metrics rather than only container state. Link to the [deployment guide](deployment-docker-sonic.md) and [warning explanation](troubleshooting.md#supervisor-reports-no-http-authentication).

The development image was checked on SONiC lab switches for process status, metrics, and read-only udev access. Recheck the actual release candidate after building through the standard Dockerfile; that earlier evidence does not validate a different image ID. Keep unpublished lab builds separate from the eventual immutable published tag.

## Rollout

Optionally, test the release tag in a parallel canary before editing the persistent service. For a normal update, keep the previous image or binary available until the new version passes endpoint, collector-health, and host-filesystem checks.

- [Docker deployment and rollback](deployment-docker-sonic.md)
- [Binary deployment and rollback](deployment-systemd.md)
- [Troubleshooting and validation](troubleshooting.md)
