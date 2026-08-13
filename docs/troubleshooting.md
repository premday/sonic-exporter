# Troubleshooting

Start with the smallest failing layer: process, listener, Redis, collector, then metric correctness. Avoid broad permission changes or privileged containers as a first response.

## Fast diagnostic set

For a Docker-on-SONiC deployment:

```bash
sudo systemctl status sonic-exporter.service --no-pager
sudo journalctl -u sonic-exporter.service -n 100 --no-pager
sudo docker ps --filter name='^/sonic-exporter$'
sudo docker inspect sonic-exporter --format \
  'Image={{.Config.Image}} Network={{.HostConfig.NetworkMode}} PID={{.HostConfig.PidMode}} Restart={{.HostConfig.RestartPolicy.Name}} CapAdd={{json .HostConfig.CapAdd}} CapDrop={{json .HostConfig.CapDrop}} Binds={{json .HostConfig.Binds}}'
```

For a direct binary service:

```bash
sudo systemctl status sonic-exporter.service --no-pager
sudo journalctl -u sonic-exporter.service -n 100 --no-pager
sudo systemctl show sonic-exporter.service \
  -p ActiveState -p SubState -p ExecMainStatus -p NRestarts
```

Do not print the complete service or container environment when it may contain `REDIS_PASSWORD`.

## The endpoint is not reachable

### On SONiC

The listener binds to the `mgmt` VRF by default. Check that the device exists and the container uses host networking:

```bash
ip link show mgmt
sudo docker inspect sonic-exporter --format '{{.HostConfig.NetworkMode}}'
```

Expected network mode: `host`.

Test through the switch management address, not remote loopback:

```bash
SWITCH_MGMT_ADDRESS=192.0.2.10
curl -v "http://${SWITCH_MGMT_ADDRESS}:9101/metrics"
```

If access is possible only through SSH:

```bash
ssh -N -L 19101:192.0.2.10:9101 192.0.2.10
```

Then, in another terminal:

```bash
curl -fsS http://127.0.0.1:19101/metrics | head
```

The tunnel target must be the switch management address because the exporter is bound to that VRF, not remote `127.0.0.1`.

### On a regular Linux host

A regular host normally has no `mgmt` device. Start the exporter with an empty VRF value:

```bash
./sonic-exporter --web.vrf=
```

For `systemd`, confirm the unit contains:

```ini
ExecStart=/usr/local/bin/sonic-exporter --web.vrf=
```

An error similar to “no such device” during listener startup usually means VRF binding was left enabled on a host without that device.

## The listener reports a permission error

The SONiC management-VRF listener requires `NET_RAW`.

For Docker, verify the runtime has dropped all capabilities and added only `NET_RAW`:

```bash
sudo docker inspect sonic-exporter --format \
  'CapAdd={{json .HostConfig.CapAdd}} CapDrop={{json .HostConfig.CapDrop}}'
```

For a direct SONiC binary service, the unit needs:

```ini
AmbientCapabilities=CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_RAW
```

Do not solve this with `--privileged` or `CAP_SYS_ADMIN`.

## Redis-backed collectors report failure

Look at collector health metrics first:

```bash
curl -fsS http://192.0.2.10:9101/metrics \
  | grep -E '^sonic_.*collector_success|^sonic_.*scrape_duration_seconds'
```

Test Redis from the same host namespace. Use `REDISCLI_AUTH` rather than putting a password in the command arguments:

```bash
export REDISCLI_AUTH='replace-with-password-if-needed'
redis-cli -h 127.0.0.1 -p 6379 ping
unset REDISCLI_AUTH
```

Expected response: `PONG`.

Check these common causes:

- `REDIS_ADDRESS` does not match the deployment topology.
- A Docker container is not using host networking, so `127.0.0.1` points to the container instead of SONiC Redis.
- Redis requires a password that is missing or incorrect.
- A remote Linux deployment cannot route to the switch Redis service.
- A collector timeout is too low for the platform or dataset size.

Keep timeouts bounded. Increase them only after confirming the source is healthy and the operation is expected to take longer.

## A collector or metric family is missing

Verify the corresponding setting in the protected configuration file:

```bash
sudo grep -E '^(FDB|ROUTING|PLATFORM_HEALTH|SYSTEM|DOCKER|FRR)_ENABLED=' \
  /etc/sonic-exporter/sonic-exporter.env
```

Restart after changes; settings are not reloaded dynamically:

```bash
sudo systemctl restart sonic-exporter.service
```

Also check `SONIC_DISABLED_METRICS`. Matching is case-sensitive and uses full metric-family names. A broad pattern such as `sonic_queue_*` also hides that collector's health metrics.

`SONIC_DISABLED_METRICS` does not filter embedded `node_exporter` or upstream FRR metrics.

## Filesystem metrics describe the container instead of the switch

The recommended Docker runtime must contain all three host-filesystem elements:

```text
--pid=host
--volume /:/hostfs:ro,rslave
./sonic-exporter --path.rootfs=/hostfs
```

Inspect the running container:

```bash
sudo docker inspect sonic-exporter --format \
  'PID={{.HostConfig.PidMode}} Binds={{json .HostConfig.Binds}} Cmd={{json .Config.Cmd}}'
```

Expected:

- `PID=host`
- a bind equivalent to `/:/hostfs:ro,rslave`
- `--path.rootfs=/hostfs` in the exporter command

Confirm the filesystem collector itself succeeds:

```bash
curl -fsS http://192.0.2.10:9101/metrics \
  | grep -x 'node_scrape_collector_success{collector="filesystem"} 1'
```

The internal prefix must not leak into Prometheus mountpoint labels:

```bash
if curl -fsS http://192.0.2.10:9101/metrics \
  | grep -q 'mountpoint="/hostfs'; then
  echo 'hostfs prefix leaked into metric labels' >&2
  exit 1
fi
```

For a specific host mount such as `/host`, compare the host source, type, size, and read-only state with the exported series:

```bash
findmnt -no SOURCE,FSTYPE,OPTIONS /host
stat -f -c 'block_size=%S blocks=%b' /host

curl -fsS http://192.0.2.10:9101/metrics \
  | grep 'node_filesystem_.*mountpoint="/host"'
```

Interpret `node_filesystem_readonly` as follows:

- `0`: the mount is writable from the host mount namespace.
- `1`: the mount is read-only in the host mount namespace.

If the host reports `rw` but the exporter reports `1`, check for a missing `--pid=host`, an old deployment that mounted `/host` read-only directly, or a container that was not recreated after the unit changed.

## Optional System collector data is incomplete

The System collector reads sources in this order:

1. SONiC Redis metadata.
2. Read-only files.
3. Optional allowlisted commands.

For a container deployment, confirm each configured path is present inside the container and mounted read-only. Common optional mounts are:

```text
/etc/sonic:/etc/sonic:ro
/host:/host:ro
/proc:/proc:ro
```

Check the collector-specific health and cache metrics:

```bash
curl -fsS http://192.0.2.10:9101/metrics \
  | grep -E '^sonic_system_(collector_success|cache_age_seconds|scrape_duration_seconds)'
```

Do not enable arbitrary command execution. The collector's command fallback is intentionally allowlisted and bounded.

## FRR metrics are absent

Confirm `FRR_ENABLED=true`, then verify the configured access method.

For the default Unix-socket mode:

```bash
sudo test -d /var/run/frr && echo 'FRR socket directory exists'
sudo docker exec sonic-exporter sh -c 'test -d /var/run/frr && ls -la /var/run/frr'
```

The Docker deployment normally needs:

```text
/var/run/frr:/var/run/frr:ro
```

Use `vtysh` mode only when socket access is unsuitable and after reviewing the additional command and permission implications. Upstream FRR collector defaults and options are listed in [Configuration](configuration.md#frr-collector).

## Scrapes are slow or series counts grow unexpectedly

Inspect duration, cache-age, skipped-entry, and truncation metrics:

```bash
curl -fsS http://192.0.2.10:9101/metrics \
  | grep -E '_(scrape_duration_seconds|cache_age_seconds|entries_skipped|entries_truncated)'
```

Then check:

- whether a previously disabled high-volume collector was enabled;
- whether a maximum such as `FDB_MAX_ENTRIES`, `ROUTING_MAX_ROUTES`, or `PLATFORM_HEALTH_MAX_PROCESSES` was raised;
- whether the refresh interval is shorter than the source operation can reliably complete;
- whether a source-side metric filter could remove an unnecessary family before emission.

Prefer bounded partial results with visible skip/truncation metrics over unbounded scans.

## The service repeatedly restarts

```bash
sudo systemctl show sonic-exporter.service \
  -p Result -p ExecMainCode -p ExecMainStatus -p NRestarts
sudo journalctl -u sonic-exporter.service -b --no-pager
```

Typical causes are an unavailable VRF device, Redis readiness failure, an occupied listen port, an invalid duration or integer environment value, or a missing optional source path after enabling a collector.

Validate unit syntax without restarting:

```bash
sudo systemd-analyze verify /etc/systemd/system/sonic-exporter.service
```

When testing a fix, use a unique canary name and alternate port rather than stopping the production exporter.
