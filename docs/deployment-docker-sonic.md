# Docker deployment for SONiC

This is the recommended production path for running `sonic-exporter` on a SONiC switch. It covers online and offline image delivery, a disposable canary, reboot persistence with `systemd`, validation, upgrades, rollback, and removal.

> Use an immutable release tag such as `vX.Y.Z`. Do not use `latest` in production.

## Contents

- [Runtime requirements](#runtime-requirements)
- [Online image pull](#online-image-pull)
- [Offline image handoff](#offline-image-handoff)
- [Baseline environment](#baseline-environment)
- [Safe manual canary](#safe-manual-canary)
- [Persistent service with systemd](#persistent-service-with-systemd)
- [Validate the deployment](#validate-the-deployment)
- [Upgrade](#upgrade)
- [Rollback](#rollback)
- [Stop or uninstall](#stop-or-uninstall)
- [Optional collector mounts](#optional-collector-mounts)

## Runtime requirements

The production container needs a small but specific host integration:

| Setting | Why it is needed |
|---|---|
| `--network host` | Reaches local SONiC Redis and exposes the HTTP listener through the host `mgmt` VRF |
| `--pid=host` | Lets the filesystem collector read the host mount table from `/proc/1/mountinfo` |
| `--volume /:/hostfs:ro,rslave` | Provides read-only host filesystem data and propagates host mount changes |
| `--path.rootfs=/hostfs` | Makes filesystem capacity and inode calls operate on the host root |
| `--cap-drop ALL --cap-add NET_RAW` | Retains only the capability required for the VRF-bound listener |

The image runs as the non-root `sonic` user. Do **not** add privileged mode, `CAP_SYS_ADMIN`, a writable host-root bind, or access to `/var/run/docker.sock`.

Optional collectors remain opt-in. Start with `FDB`, Routing, Platform Health, System, Docker, and FRR disabled unless you have a specific need and have reviewed their data sources and cardinality.

## Online image pull

On a switch that can reach GHCR:

```bash
RELEASE_TAG=vX.Y.Z

sudo docker pull ghcr.io/premday/sonic-exporter:${RELEASE_TAG}
sudo docker image inspect ghcr.io/premday/sonic-exporter:${RELEASE_TAG} \
  --format 'ID={{.Id}} RepoDigests={{json .RepoDigests}}'
```

Keep the old image until the new release has passed validation.

## Offline image handoff

A Docker image tarball created with `docker save` is different from the release archive that contains the standalone binary.

On a connected Linux host:

```bash
RELEASE_TAG=vX.Y.Z

sudo docker pull ghcr.io/premday/sonic-exporter:${RELEASE_TAG}
sudo docker save ghcr.io/premday/sonic-exporter:${RELEASE_TAG} \
  -o sonic-exporter-${RELEASE_TAG}.docker.tar
sha256sum sonic-exporter-${RELEASE_TAG}.docker.tar \
  > sonic-exporter-${RELEASE_TAG}.docker.tar.sha256
scp sonic-exporter-${RELEASE_TAG}.docker.tar \
  sonic-exporter-${RELEASE_TAG}.docker.tar.sha256 \
  admin@192.0.2.10:/home/admin/
```

On the SONiC switch:

```bash
cd /home/admin
RELEASE_TAG=vX.Y.Z

sha256sum -c sonic-exporter-${RELEASE_TAG}.docker.tar.sha256
sudo docker load -i sonic-exporter-${RELEASE_TAG}.docker.tar
sudo docker image inspect ghcr.io/premday/sonic-exporter:${RELEASE_TAG} \
  --format 'ID={{.Id}} RepoDigests={{json .RepoDigests}}'
```

`docker load` restores the repository and tag metadata, so the run command remains `ghcr.io/premday/sonic-exporter:vX.Y.Z`.

## Baseline environment

Store runtime settings outside the unit file. The restrictive mode matters if `REDIS_PASSWORD` is not empty.

```bash
sudo install -d -m 0750 -o root -g root /etc/sonic-exporter
sudo install -m 0600 -o root -g root /dev/null \
  /etc/sonic-exporter/sonic-exporter.env
sudo tee /etc/sonic-exporter/sonic-exporter.env >/dev/null <<'EOF_ENV'
REDIS_ADDRESS=127.0.0.1:6379
REDIS_NETWORK=tcp
REDIS_PASSWORD=
SONIC_DISABLED_METRICS=

FDB_ENABLED=false
ROUTING_ENABLED=false
PLATFORM_HEALTH_ENABLED=false
SYSTEM_ENABLED=false
DOCKER_ENABLED=false
FRR_ENABLED=false
EOF_ENV
```

See the [configuration reference](configuration.md) before enabling optional collectors or changing refresh intervals and limits.

## Safe manual canary

The canary uses a unique name and alternate port. It does not stop or replace an existing exporter service.

Set the switch management address and release tag first:

```bash
RELEASE_TAG=vX.Y.Z
SWITCH_MGMT_ADDRESS=192.0.2.10
CANARY_SUFFIX=$(date +%s)
CANARY_NAME="sonic-exporter-canary-${CANARY_SUFFIX}"
CANARY_PORT=$((19000 + CANARY_SUFFIX % 1000))
```

Check that the selected port is free, then start the canary:

```bash
if sudo ss -ltn "sport = :${CANARY_PORT}" | grep -q LISTEN; then
  echo "canary port ${CANARY_PORT} is already in use" >&2
  exit 1
fi

sudo docker run -d \
  --name "${CANARY_NAME}" \
  --label app=sonic-exporter-canary \
  --network host \
  --pid=host \
  --volume /:/hostfs:ro,rslave \
  --cap-drop ALL \
  --cap-add NET_RAW \
  --restart no \
  --env-file /etc/sonic-exporter/sonic-exporter.env \
  ghcr.io/premday/sonic-exporter:${RELEASE_TAG} \
  ./sonic-exporter \
  --web.listen-address=":${CANARY_PORT}" \
  --path.rootfs=/hostfs
```

Validate the endpoint through the switch management address, not remote loopback:

```bash
METRICS_URL="http://${SWITCH_MGMT_ADDRESS}:${CANARY_PORT}/metrics"

curl -fsS "${METRICS_URL}" \
  | grep -E 'sonic_.*collector_success|node_scrape_collector_success|sonic_lldp_neighbors'
```

Then run the [host-filesystem validation](troubleshooting.md#filesystem-metrics-describe-the-container-instead-of-the-switch). Remove only the uniquely named canary when finished:

```bash
sudo docker rm -f "${CANARY_NAME}"
```

The image intentionally has no embedded Docker `HEALTHCHECK`: a fixed command cannot safely infer a custom VRF or listener address. Use Prometheus or another external HTTP probe for readiness; container exit plus the `systemd` restart policy provide liveness handling.

## Persistent service with systemd

SONiC uses `systemd` services tied to `sonic.target` for its local containers. The following unit manages only the `sonic-exporter` container.

Create `/etc/systemd/system/sonic-exporter.service` and replace `vX.Y.Z` with the tested release tag:

```ini
[Unit]
Description=SONiC Exporter container
Documentation=https://github.com/premday/sonic-exporter
Requires=docker.service database.service
After=docker.service database.service sonic.target
BindsTo=sonic.target
StartLimitIntervalSec=1200
StartLimitBurst=3

[Service]
User=root
EnvironmentFile=/etc/sonic-exporter/sonic-exporter.env
Restart=always
RestartSec=30
ExecStartPre=/bin/sh -c 'i=0; while [ "$$i" -lt 60 ]; do REDISCLI_AUTH="$$REDIS_PASSWORD" /usr/bin/redis-cli -h 127.0.0.1 -p 6379 ping 2>/dev/null | /bin/grep -q PONG && exit 0; i=$$((i + 1)); sleep 2; done; exit 1'
ExecStartPre=-/usr/bin/docker rm -f sonic-exporter
ExecStart=/usr/bin/docker run --name sonic-exporter --label app=sonic-exporter --label managed-by=systemd --restart no --network host --pid=host --volume /:/hostfs:ro,rslave --cap-drop ALL --cap-add NET_RAW --env-file /etc/sonic-exporter/sonic-exporter.env ghcr.io/premday/sonic-exporter:vX.Y.Z ./sonic-exporter --path.rootfs=/hostfs
ExecStop=-/usr/bin/docker stop sonic-exporter
ExecStopPost=-/usr/bin/docker rm -f sonic-exporter

[Install]
WantedBy=sonic.target
```

This readiness command assumes SONiC Redis is on `127.0.0.1:6379`. Adjust it deliberately if `REDIS_ADDRESS` points elsewhere. `REDISCLI_AUTH` avoids placing a non-empty password in the process arguments. The doubled dollar signs are required in a systemd command line so the shell, rather than systemd, expands the loop and password variables.

Verify, enable, and start the unit:

```bash
sudo systemd-analyze verify /etc/systemd/system/sonic-exporter.service
sudo systemctl daemon-reload
sudo systemctl enable --now sonic-exporter.service
sudo systemctl status sonic-exporter.service --no-pager
```

Docker's restart policy should remain `no` because `systemd` is the single restart owner.

## Validate the deployment

Confirm the runtime contract:

```bash
sudo docker inspect sonic-exporter --format \
  'Image={{.Config.Image}} Status={{.State.Status}} Network={{.HostConfig.NetworkMode}} PID={{.HostConfig.PidMode}} Restart={{.HostConfig.RestartPolicy.Name}} CapAdd={{json .HostConfig.CapAdd}} CapDrop={{json .HostConfig.CapDrop}} Binds={{json .HostConfig.Binds}}'
```

Expected characteristics include:

- `Network=host`
- `PID=host`
- `Restart=no`
- `NET_RAW` added and all other capabilities dropped
- `/:/hostfs:ro,rslave` in the bind list

Validate the metrics endpoint through the management address:

```bash
SWITCH_MGMT_ADDRESS=192.0.2.10
curl -fsS "http://${SWITCH_MGMT_ADDRESS}:9101/metrics" \
  | grep -E 'sonic_.*collector_success|node_scrape_collector_success|sonic_lldp_neighbors'
```

For a workstation that can reach the switch only over SSH:

```bash
ssh -N -L 19101:192.0.2.10:9101 192.0.2.10
```

Then, in another terminal:

```bash
curl -fsS http://127.0.0.1:19101/metrics | head
```

Replace both occurrences of `192.0.2.10` with the switch management address. The tunnel targets the management-VRF listener, not remote `127.0.0.1`.

For deeper checks, see [Troubleshooting](troubleshooting.md), especially collector health, Redis reachability, and host-filesystem identity.

## Upgrade

Pull or load the new immutable image first. Keep the previous image tag until validation is complete.

```bash
NEW_RELEASE_TAG=vX.Y.Z
sudo docker pull ghcr.io/premday/sonic-exporter:${NEW_RELEASE_TAG}
sudo docker image inspect ghcr.io/premday/sonic-exporter:${NEW_RELEASE_TAG} \
  --format '{{.Id}}'
```

For an offline switch, repeat the [offline handoff](#offline-image-handoff) with the new tag.

Back up the unit, change only the image tag, and restart only the exporter:

```bash
sudo cp /etc/systemd/system/sonic-exporter.service \
  /etc/systemd/system/sonic-exporter.service.bak
sudoedit /etc/systemd/system/sonic-exporter.service
sudo systemd-analyze verify /etc/systemd/system/sonic-exporter.service
sudo systemctl daemon-reload
sudo systemctl restart sonic-exporter.service
sudo systemctl status sonic-exporter.service --no-pager
```

Repeat the validation checks above before deleting the old image.

## Rollback

Confirm the previous image is still present, restore its tag in the unit, and restart only this service:

```bash
PREVIOUS_RELEASE_TAG=vX.Y.Z
sudo docker image inspect ghcr.io/premday/sonic-exporter:${PREVIOUS_RELEASE_TAG} \
  --format '{{.Id}}'

sudoedit /etc/systemd/system/sonic-exporter.service
sudo systemd-analyze verify /etc/systemd/system/sonic-exporter.service
sudo systemctl daemon-reload
sudo systemctl restart sonic-exporter.service
sudo systemctl status sonic-exporter.service --no-pager
```

Do not restart SONiC core services for an exporter rollback.

## Stop or uninstall

Stop until the next boot:

```bash
sudo systemctl stop sonic-exporter.service
```

Disable at boot:

```bash
sudo systemctl disable --now sonic-exporter.service
```

Remove the unit, container, and protected environment file:

```bash
sudo systemctl disable --now sonic-exporter.service
sudo systemctl reset-failed sonic-exporter.service 2>/dev/null || true
sudo rm -f /etc/systemd/system/sonic-exporter.service
sudo systemctl daemon-reload
sudo docker rm -f sonic-exporter 2>/dev/null || true
sudo rm -rf /etc/sonic-exporter
```

Remove only explicitly named exporter images after confirming they are no longer needed:

```bash
sudo docker image rm ghcr.io/premday/sonic-exporter:vX.Y.Z
```

Never use broad `docker rm`, `docker container prune`, or `docker system prune` commands on a SONiC switch; they can affect unrelated SONiC containers.

## Optional collector mounts

The base `/:/hostfs:ro,rslave` mount is required for correct embedded filesystem metrics and is independent of optional collectors.

Add the following read-only mounts only when the corresponding collector needs them:

| Collector | Optional host access |
|---|---|
| System | `/etc/sonic:/etc/sonic:ro`, `/host:/host:ro`, and `/proc:/proc:ro` as required by the configured file paths |
| FRR | `/var/run/frr:/var/run/frr:ro` for Unix-socket access |

The Docker collector reads SONiC `STATE_DB`; it does not require the Docker socket.

Deployment automation may perform image transfer, `docker load`, unit installation, and rollout checks, but it should retain the safety properties documented here and manage only resources labelled or named for `sonic-exporter`.
