# Contributing to sonic-exporter

Contributions are welcome. The highest-value changes keep the exporter read-only, predictable under load, and easy to operate on real SONiC switches.

## Before starting

For a small bug fix, documentation correction, or test improvement, a pull request is usually enough. For a new collector, metric redesign, default change, or deployment behavior change, opening an issue first is recommended so the source, naming, cardinality, compatibility, and rollout impact can be discussed before implementation.

Report security vulnerabilities privately through the process in [SECURITY.md](SECURITY.md), not in a public issue.

## Development setup

Requirements:

- Go 1.25 or newer;
- Docker for image and Compose smoke tests;
- a shell environment capable of running the scripts under `scripts/`.

Build and test:

```bash
go test -race -shuffle=on -count=1 ./cmd/sonic-exporter
go test -race -count=1 ./...
go build ./...
```

Local Redis-fixture environment:

```bash
docker compose up --build -d
curl -fsS http://127.0.0.1:9101/metrics | head
docker compose down
```

The Compose setup is for development only and disables VRF binding because a normal CI or developer host does not provide the SONiC `mgmt` device.

## Run the repository checks

Before submitting a pull request, run the checks relevant to your change:

```bash
go test -race -shuffle=on -count=1 ./cmd/sonic-exporter
go test -race -count=1 ./...
go build ./...
./scripts/validate-dashboard.sh dashboards/sonic-exporter.json
./scripts/smoke-image.sh --dry-run
./scripts/smoke-image.sh
```

The GitHub Actions workflow also runs:

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run
govulncheck ./...
gitleaks detect --no-git --redact --source .
```

Install `govulncheck` and Gitleaks when you want to reproduce those jobs locally:

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
go install github.com/zricethezav/gitleaks/v8@latest
```

## Collector design expectations

A collector should:

- read only from SONiC Redis, documented read-only files, or an explicitly allowlisted command path;
- use stable Prometheus names and bounded, deterministic labels;
- place context deadlines around source work;
- avoid unbounded key scans and series growth;
- expose success and duration metrics, plus cache age for background-refresh collectors;
- expose skipped, truncated, or stale signals where partial results are possible;
- preserve the previous successful snapshot when a background refresh fails;
- use the shared in-repo metric filter for every SONiC metric family it emits;
- remain disabled by default when it is experimental, unusually expensive, high-cardinality, or requires extra host access.

Choose an execution model deliberately:

- The four original core collectors use a short scrape-time cache.
- Most newer SONiC collectors refresh in the background and serve an in-memory snapshot.
- FRR delegates collection to the upstream `frr_exporter` library.

See [docs/architecture.md](docs/architecture.md) before adding a collector.

## Tests and fixtures

For collector changes, add or update:

- focused unit tests;
- Prometheus `CollectAndLint` coverage;
- success-metric assertions;
- representative metric assertions;
- edge cases for malformed or missing source data;
- guardrail assertions for skipped or truncated entries;
- Redis fixtures under `fixtures/test/` when source data is required.

Keep output deterministic so tests and scrapes do not depend on Redis key order.

If Redis fixture keys are added manually during local development, persist them with `SAVE`.

## Documentation changes

Update the documentation closest to the source of truth:

| Change | Documentation to review |
|---|---|
| New collector or changed default | README collector table and `docs/configuration.md` |
| New flag or environment variable | `docs/configuration.md` |
| Changed cache, source, or safety model | `docs/architecture.md` |
| Changed container/runtime requirement | `docs/deployment-docker-sonic.md` and troubleshooting |
| Changed binary service behavior | `docs/deployment-systemd.md` |
| New dashboard panel or variable | `docs/grafana-dashboard.md` and dashboard validation |
| User-visible release or migration impact | GitHub release notes and `docs/releasing.md` |

Do not duplicate a full reference table in the README. Keep the README focused on evaluation and first use, then link to the detailed document.

## Pull requests

A good pull request:

- explains the operational problem being solved;
- identifies the data source and supported SONiC variants;
- calls out metric-name, label, default, or cardinality changes;
- describes failure behavior and rollback;
- includes test evidence;
- updates relevant documentation;
- avoids unrelated formatting or dependency changes.

Keep commits understandable and do not include generated binaries, credentials, production addresses, or proprietary switch data.

## Maintainer releases

Release tags, artifacts, attestations, and rollout expectations are documented in [docs/releasing.md](docs/releasing.md). GitHub Releases are the canonical release history.
