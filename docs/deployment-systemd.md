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
- [Run a parallel canary](#run-a-parallel-canary)
- [Update or roll back](#update-or-roll-back)

## Before you begin

The published archive is a static **Linux/amd64 binary archive**. It is not a Docker image and cannot be loaded with `docker load`.

Install this archive directly on the SONiC Community OS switch. It must be a Linux/amd64 target that can run the published binary.

## Create the service account

```bash
sudo useradd --system --user-group --no-create-home --shell /usr/sbin/nologin sonic-exporter
```

Use `/sbin/nologin` instead when that is the valid path on the switch.

## Verify and install the binary

Download the release archive and `checksums.txt` from the same GitHub release, then verify before extraction:

```bash
VERSION=X.Y.Z

sha256sum -c checksums.txt
tar -xzf sonic-exporter_${VERSION}_linux_amd64.tar.gz
sudo install -m 0755 ./sonic-exporter /usr/local/bin/sonic-exporter
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

The direct-binary path sees the switch root at `/`, so keep the default `--path.rootfs=/`. It retains the default `mgmt` VRF listener, which is why the unit includes `CAP_NET_RAW`. Treat it as an advanced canary path; the container deployment remains the recommended SONiC Community OS installation.

## Run a parallel canary

A canary must use a different unit name, port, and binary path. Install the candidate binary separately, then copy the unit:

```bash
sudo install -m 0755 ./sonic-exporter /usr/local/bin/sonic-exporter-canary
sudo cp /etc/systemd/system/sonic-exporter.service \
  /etc/systemd/system/sonic-exporter-canary.service
sudoedit /etc/systemd/system/sonic-exporter-canary.service
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

Remove it without touching the production service:

```bash
sudo systemctl disable --now sonic-exporter-canary.service 2>/dev/null || true
sudo rm -f /etc/systemd/system/sonic-exporter-canary.service
sudo rm -f /usr/local/bin/sonic-exporter-canary
sudo systemctl daemon-reload
```

For a management-VRF canary on SONiC, keep VRF binding enabled, retain `CAP_NET_RAW`, and verify `http://<switch-management-address>:19101/metrics`.

## Update or roll back

Keep the previous binary until the replacement has passed validation:

```bash
sudo cp /usr/local/bin/sonic-exporter \
  /usr/local/bin/sonic-exporter.previous
sudo install -m 0755 ./sonic-exporter /usr/local/bin/sonic-exporter.new
sudo mv /usr/local/bin/sonic-exporter.new /usr/local/bin/sonic-exporter
sudo systemctl restart sonic-exporter.service
sudo systemctl status sonic-exporter.service --no-pager
```

Rollback is the same atomic replacement in reverse:

```bash
sudo install -m 0755 /usr/local/bin/sonic-exporter.previous \
  /usr/local/bin/sonic-exporter.new
sudo mv /usr/local/bin/sonic-exporter.new /usr/local/bin/sonic-exporter
sudo systemctl restart sonic-exporter.service
```

Change collector settings in `/etc/sonic-exporter/sonic-exporter.env`, then restart the service. For local unit customization, use `systemctl edit` rather than modifying a package-managed unit directly.
