# Advanced direct-binary deployment on SONiC Community OS

This is an advanced direct-binary installation for a SONiC Community OS switch. For most switches, use the [Docker deployment](deployment-docker-sonic.md), which is the recommended path and includes the switch integration it needs.

> This path has not been validated as broadly as Docker on SONiC Community OS. Test it in a lab or canary environment before production use.

## Contents

- [Before you begin](#before-you-begin)
- [Create the service account](#create-the-service-account)
- [Verify and install the binary](#verify-and-install-the-binary)
- [Create a protected environment file](#create-a-protected-environment-file)
- [Install the systemd unit](#install-the-systemd-unit)
- [Start and validate](#start-and-validate)
- [Update or roll back](#update-or-roll-back)
- [Optional: test with a parallel canary](#optional-test-with-a-parallel-canary)

## Before you begin

The published archive is a static **Linux/amd64 binary archive**. It is not a Docker image and cannot be loaded with `docker load`.

Install this archive directly on the SONiC Community OS switch. It must be a Linux/amd64 target that can run the published binary.

## Create the service account

```bash
sudo useradd --system --user-group --no-create-home --shell /usr/sbin/nologin sonic-exporter
```

Use `/sbin/nologin` instead when that is the valid path on the switch.

## Verify and install the binary

Download the release archive and `checksums.txt` from the same GitHub release, then verify the archive before extraction. The checksum file also lists the release SBOM, so select the archive entry when the SBOM was not downloaded:

```bash
VERSION=X.Y.Z
ARCHIVE="sonic-exporter_${VERSION}_linux_amd64.tar.gz"

awk -v archive="${ARCHIVE}" \
  '$2 == archive {line = $0; matches++} END {if (matches != 1) exit 1; print line}' \
  checksums.txt \
  | sha256sum -c - \
  && rm -f ./sonic-exporter \
  && tar -xzf "${ARCHIVE}" \
  && sudo install -m 0755 ./sonic-exporter /usr/local/bin/sonic-exporter
```

Confirm the binary starts and display the available flags:

```bash
/usr/local/bin/sonic-exporter --help
```

## Create a protected environment file

Create a directory that the service group can read, but other local users cannot browse:

```bash
sudo install -d -m 0750 -o root -g sonic-exporter /etc/sonic-exporter
sudo install -m 0640 -o root -g sonic-exporter /dev/null \
  /etc/sonic-exporter/sonic-exporter.env
sudo tee /etc/sonic-exporter/sonic-exporter.env >/dev/null <<'EOF_ENV'
REDIS_ADDRESS=localhost:6379
REDIS_PASSWORD=
REDIS_NETWORK=tcp
SONIC_DISABLED_METRICS=

LLDP_ENABLED=true
VLAN_ENABLED=true
LAG_ENABLED=true
SWITCH_ENABLED=true
THERMAL_ENABLED=true
TRANSCEIVER_ENABLED=true
FDB_ENABLED=false
ROUTING_ENABLED=false
PLATFORM_HEALTH_ENABLED=false
SYSTEM_ENABLED=false
DOCKER_ENABLED=false
FRR_ENABLED=false
EOF_ENV
```

Do not use diagnostics that print the complete service environment when `REDIS_PASSWORD` is populated. See the [configuration reference](configuration.md) for all settings.

## Install the systemd unit

Create `/etc/systemd/system/sonic-exporter.service`:

```ini
[Unit]
Description=SONiC Prometheus Exporter
Documentation=https://github.com/premday/sonic-exporter
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=sonic-exporter
Group=sonic-exporter
EnvironmentFile=/etc/sonic-exporter/sonic-exporter.env
ExecStart=/usr/local/bin/sonic-exporter
Restart=on-failure
RestartSec=5s
UMask=0027

StandardOutput=journal
StandardError=journal

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
LockPersonality=true
MemoryDenyWriteExecute=true
RestrictSUIDSGID=true
RestrictRealtime=true
SystemCallArchitectures=native
AmbientCapabilities=CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_RAW

[Install]
WantedBy=multi-user.target
```

This SONiC Community OS unit retains the default `mgmt` VRF binding and grants only `CAP_NET_RAW` for `SO_BINDTODEVICE`. `ProtectSystem=strict` makes the filesystem read-only to the service while still permitting reads from `/proc`, `/etc`, and other switch sources. Do not add `ReadOnlyPaths` entries for optional paths that may not exist on the switch; such entries can make service startup platform-dependent.

Validate the unit before starting it:

```bash
sudo systemd-analyze verify /etc/systemd/system/sonic-exporter.service
```

## Start and validate

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now sonic-exporter.service
sudo systemctl status sonic-exporter.service --no-pager
```

Check service state without printing its environment:

```bash
sudo systemctl show sonic-exporter.service \
  -p ActiveState -p SubState -p ExecMainStatus -p NRestarts
sudo journalctl -u sonic-exporter.service -n 100 --no-pager
```

Check the endpoint:

```bash
curl -fsS http://192.0.2.10:9101/metrics | head
curl -fsS http://192.0.2.10:9101/metrics \
  | grep -E 'sonic_.*collector_success|node_scrape_collector_success'
```

Useful metric prefixes include:

| Collector | Prefix |
|---|---|
| LLDP | `sonic_lldp_` |
| VLAN | `sonic_vlan_` |
| LAG | `sonic_lag_` |
| FDB | `sonic_fdb_` |
| System | `sonic_system_` |
| Docker | `sonic_docker_` |
| FRR | `frr_` |

If the service is running but a collector reports failure, use the [troubleshooting guide](troubleshooting.md) before weakening the unit hardening.

The direct-binary path sees the switch root at `/`, so keep the default `--path.rootfs=/`. It retains the default `mgmt` VRF listener, which is why the unit includes `CAP_NET_RAW`. Treat it as an advanced installation; the container deployment remains the recommended SONiC Community OS installation.

The `v0.5.0` direct binary was tested in the LAB with the default `mgmt` VRF listener and explicit non-VRF mode using `--web.vrf=`.

## Update or roll back

Download the replacement archive and `checksums.txt` into a clean working directory. Then verify, extract, and install the replacement as one failure-gated sequence. Keep the current binary until the replacement has passed validation:

```bash
VERSION=X.Y.Z
ARCHIVE="sonic-exporter_${VERSION}_linux_amd64.tar.gz"

sudo cp /usr/local/bin/sonic-exporter \
  /usr/local/bin/sonic-exporter.previous \
  && rm -f ./sonic-exporter \
  && awk -v archive="${ARCHIVE}" \
    '$2 == archive {line = $0; matches++} END {if (matches != 1) exit 1; print line}' \
    checksums.txt \
  | sha256sum -c - \
  && tar -xzf "${ARCHIVE}" \
  && sudo install -m 0755 ./sonic-exporter /usr/local/bin/sonic-exporter.new \
  && sudo mv /usr/local/bin/sonic-exporter.new /usr/local/bin/sonic-exporter \
  && sudo systemctl restart sonic-exporter.service \
  && sudo systemctl status sonic-exporter.service --no-pager
```

Rollback is the same atomic replacement in reverse:

```bash
sudo install -m 0755 /usr/local/bin/sonic-exporter.previous \
  /usr/local/bin/sonic-exporter.new \
  && sudo mv /usr/local/bin/sonic-exporter.new /usr/local/bin/sonic-exporter \
  && sudo systemctl restart sonic-exporter.service
```

After an update or rollback, repeat the service-state, endpoint, and collector checks in [Start and validate](#start-and-validate). If filesystem metrics are important to the rollout, also run the [host-filesystem checks](troubleshooting.md#filesystem-metrics-describe-the-container-instead-of-the-switch).

Change collector settings in `/etc/sonic-exporter/sonic-exporter.env`, then restart the service. For local unit customization, use `systemctl edit` rather than modifying a package-managed unit directly.

## Optional: test with a parallel canary

A canary is a second temporary exporter used to test a new binary without stopping the existing service. New installations and normal updates do not require this procedure.

The canary must use a different unit name, port, and binary path. Download the candidate archive and `checksums.txt` into a clean working directory, then verify, extract, and install it as one failure-gated sequence before copying the unit:

```bash
VERSION=X.Y.Z
ARCHIVE="sonic-exporter_${VERSION}_linux_amd64.tar.gz"

rm -f ./sonic-exporter \
  && awk -v archive="${ARCHIVE}" \
    '$2 == archive {line = $0; matches++} END {if (matches != 1) exit 1; print line}' \
    checksums.txt \
  | sha256sum -c - \
  && tar -xzf "${ARCHIVE}" \
  && sudo install -m 0755 ./sonic-exporter /usr/local/bin/sonic-exporter-canary \
  && sudo cp /etc/systemd/system/sonic-exporter.service \
    /etc/systemd/system/sonic-exporter-canary.service \
  && sudoedit /etc/systemd/system/sonic-exporter-canary.service
```

Change at least these fields:

```ini
[Unit]
Description=SONiC Prometheus Exporter canary

[Service]
ExecStart=
ExecStart=/usr/local/bin/sonic-exporter-canary --web.listen-address=:19101
```

Start only the canary and verify it:

```bash
sudo systemd-analyze verify /etc/systemd/system/sonic-exporter-canary.service
sudo systemctl daemon-reload
sudo systemctl start sonic-exporter-canary.service
sudo systemctl status sonic-exporter-canary.service --no-pager
curl -fsS http://192.0.2.10:19101/metrics >/dev/null && echo OK
sudo journalctl -u sonic-exporter-canary.service -n 100 --no-pager
```

For a management-VRF canary on SONiC, keep VRF binding enabled, retain `CAP_NET_RAW`, and verify `http://<switch-management-address>:19101/metrics`.

To test listener operation without VRF binding, use a separate canary port and set `--web.vrf=` explicitly:

```ini
[Service]
ExecStart=
ExecStart=/usr/local/bin/sonic-exporter-canary --web.listen-address=127.0.0.1:19102 --web.vrf=
AmbientCapabilities=
CapabilityBoundingSet=
```

After reloading and restarting the canary, test this mode through the default routing table. Loopback is a simple local check:

```bash
sudo systemctl daemon-reload
sudo systemctl restart sonic-exporter-canary.service
curl -fsS http://127.0.0.1:19102/metrics >/dev/null && echo OK
```

Non-VRF mode can work locally even when remote management access requires the `mgmt` VRF. Test the management-VRF and non-VRF modes on separate ports so listener reachability is not confused with collector behavior.

Remove the canary after both listener tests without touching the production service:

```bash
sudo systemctl stop sonic-exporter-canary.service \
  && sudo rm -f /etc/systemd/system/sonic-exporter-canary.service \
  && sudo rm -f /usr/local/bin/sonic-exporter-canary \
  && sudo systemctl daemon-reload
```
