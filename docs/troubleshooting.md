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
sudo docker exec sonic-exporter bash -c "supervisorctl status"
sudo docker logs --since 30m --tail 100 sonic-exporter
```

For an advanced direct-binary SONiC Community OS service:

```bash
sudo systemctl status sonic-exporter.service --no-pager
sudo journalctl -u sonic-exporter.service -n 100 --no-pager
sudo systemctl show sonic-exporter.service \
  -p ActiveState -p SubState -p ExecMainStatus -p NRestarts
```

Do not print the complete service or container environment when it may contain `REDIS_PASSWORD`.

## Supervisor reports no HTTP authentication

The v0.7.0 image intentionally logs this message at startup:

```text
CRIT Server 'unix_http_server' running without any HTTP authentication checking
```

Despite the severity label, this message does not mean Supervisor failed or that a TCP management service is exposed. The server listens only on `/run/sonic-supervisor/supervisor.sock`. Access is controlled by a `0600` socket inside a `0700` directory, both owned by the container's `sonic` user. Verify those permissions without changing them:

```bash
sudo docker exec sonic-exporter stat -c '%a %U:%G %n' \
  /run/sonic-supervisor /run/sonic-supervisor/supervisor.sock
sudo docker exec sonic-exporter bash -c "supervisorctl status"
```

Processes running as `sonic`, and privileged operators, can control Supervisor. This is the intended trust boundary; no separate password is required for the bundled setup. Do not expose the socket through a host mount, relax its permissions, or enable a TCP interface just to silence the warning. A custom configuration that changes that boundary needs its own authentication review.

`silent=true` prevents a duplicate foreground copy of Supervisor's activity log; logs still go to Docker through stderr. It does not hide warnings or errors.

## Exporter is missing from SONiC monitoring

Check the `sonic-exporter` entry in both live CONFIG_DB and `/etc/sonic/config_db.json`, following [FEATURE registration](deployment-docker-sonic.md#register-the-exporter-as-a-sonic-feature). A running Docker container and a working `supervisorctl` command do not create that entry. A saved JSON entry alone does not activate live monitoring.

The inspected SONiC setup uses the shared Monit `container_checker`; not finding a separate `sonic-exporter` Monit file is normal. Check the shared monitor state and keep container enrollment separate from the image/deployment critical-process policy.

## Container is running but the exporter is not

Supervisor can remain running when the exporter is `BACKOFF`, `EXITED`, or `FATAL`. Check `supervisorctl status`, bounded container logs, and the metrics endpoint together. Fix the startup error before restarting only the exporter container through its service owner. Do not restart Docker or switch networking. The image supplies the process-status interface, but does not configure SONiC's critical-process policy.

If `supervisorctl` cannot connect, check whether the deployment overrides the image's default command with `./sonic-exporter`; that bypasses Supervisor.

## Disk collector cannot read udev data

`Failed to open directory, disabling udev device properties` means `/run/udev/data` is unavailable inside the container. Basic disk counters can still succeed, but additional device properties are disabled. If `stat /run/udev/data` succeeds on the switch, add this read-only Docker mount through the deployment configuration:

```text
--mount type=bind,src=/run/udev/data,dst=/run/udev/data,readonly
```

Recreate only the exporter container under the normal approval process, then check its logs and `node_scrape_collector_success{collector="diskstats"}`. Do not install udev in the container or use privileged mode. If the host has no udev data, omit the mount and accept the metadata limitation.

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

### Stale management-VRF listener after interface initialization

An exporter container can remain running while its listener is bound to an interface index that no longer exists. Check the listener and the current management-VRF index without changing switch state:

```bash
ss -lnt 'sport = :9101'
ip -o link show mgmt
systemctl show sonic-exporter.service -p Requires -p After
systemctl show interfaces-config.service -p Type -p ActiveState -p Result
```

For example, `*%if4:9101` with `mgmt` at index `7` suggests a stale binding; `*%mgmt:9101` identifies the current named interface. Confirm an unresolved listener index no longer exists with `ip -o link show` before drawing that conclusion. `SO_BINDTODEVICE` binds the socket to an interface index; recreating the VRF with the same name does not move the existing listener to the new index.

One possible cause is a boot-time race: the exporter and `interfaces-config.service` both start after `sonic.target`, but the exporter does not wait for interface initialization to finish. The interface script may recreate management networking after the listener binds. A stale index is evidence of a binding problem, not proof of which event recreated the VRF.

For Docker deployments, use the [management-VRF startup ordering](deployment-docker-sonic.md#management-vrf-startup-ordering) dependency to wait for synchronous interface initialization. The ordering rule alone does not repair an existing stale socket. Once management networking is stable, restart only the exporter under the normal change-approval process, then check the listener binding and metrics reachability. Do not restart Docker, switch networking, or the switch as a workaround.

This startup rule adds no polling or fixed delays. It does not cover VRF recreation later during operation, and a running container alone does not establish metrics reachability.

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

Test Redis from the same host namespace. Read the password without echoing it or storing it in shell history:

```bash
read -rsp 'Redis password: ' REDISCLI_AUTH && export REDISCLI_AUTH
printf '\n'
redis-cli -h 127.0.0.1 -p 6379 ping
unset REDISCLI_AUTH
```

Expected response: `PONG`.

Check these common causes:

- `REDIS_ADDRESS` does not match the deployment topology.
- A Docker container is not using host networking, so `127.0.0.1` points to the container instead of SONiC Redis.
- Redis requires a password that is missing or incorrect.
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

The recommended Docker runtime needs host PID visibility and the read-only host-root mount:

```text
--pid=host
--volume /:/hostfs:ro,rslave
```

Inspect the running container:

```bash
sudo docker inspect sonic-exporter --format \
  'PID={{.HostConfig.PidMode}} Binds={{json .HostConfig.Binds}} Cmd={{json .Config.Cmd}}'
sudo docker exec sonic-exporter printenv SONIC_EXPORTER_ROOTFS
```

Expected:

- `PID=host`
- a bind equivalent to `/:/hostfs:ro,rslave`
- Supervisor as the container command and `SONIC_EXPORTER_ROOTFS=/hostfs`, which the bundled configuration passes as `--path.rootfs=/hostfs`

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

For a specific switch mount such as `/host`, compare the source, type, size, and read-only state with the exported series:

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

## Process metrics are missing or PSU slot labels are unexpected

Check the collector health and affected metrics first:

```bash
SWITCH_MGMT_ADDRESS=192.0.2.10
curl -fsS "http://${SWITCH_MGMT_ADDRESS}:9101/metrics" \
  | grep -E '^sonic_(platform_(collector_success|process_(cpu|memory)_percent)|hw_(collector_success|psu_info))'
```

Inspect `STATE_DB` (database `6`) with read-only commands:

```bash
read -rsp 'Redis password: ' REDISCLI_AUTH && export REDISCLI_AUTH
printf '\n'
redis-cli -n 6 --scan --pattern 'PROCESS_STATS|*'
redis-cli -n 6 HGETALL 'PROCESS_STATS|<pid>'
redis-cli -n 6 --scan --pattern 'PSU_INFO|PSU*'
redis-cli -n 6 HGETALL 'PSU_INFO|PSU<slot>'
unset REDISCLI_AUTH
```

For a process entry, the exporter uses numeric `%CPU` and `%MEM` first, then numeric `CPU` and `MEM`. The CPU or memory metric is omitted when neither candidate parses as numeric.

Accepted PSU key forms such as `PSU1`, `PSU 1`, `PSU_1`, and `PSU-1` normalize to `slot="1"` in `sonic_hw_psu_info`. A PSU key with an empty slot is rejected and does not emit a metric with an empty slot.
