# Configuration reference

`sonic-exporter` uses CLI flags for its SONiC Community OS HTTP listener and switch paths, and environment variables for Redis and collector settings.

> Settings are read at process startup. Restart the exporter after changing a flag or environment variable.

## Contents

- [Command-line flags](#command-line-flags)
- [Core settings](#core-settings)
- [Source-side metric filtering](#source-side-metric-filtering)
- [Collector settings](#lldp-collector)
- [System collector](#system-collector-experimental)
- [Docker collector](#docker-collector-experimental)
- [FRR collector](#frr-collector)

## Command-line flags

### HTTP listener

| Flag | Description | Default |
|---|---|---|
| `--web.listen-address` | TCP address used by the HTTP server | `:9101` |
| `--web.telemetry-path` | Metrics endpoint path | `/metrics` |
| `--web.vrf` | VRF device for HTTP listeners; an empty value disables VRF binding | `mgmt` |

VRF mode uses `SO_BINDTODEVICE`. It cannot be combined with exporter-toolkit systemd socket activation or `vsock://` listeners. Use `--web.vrf=` for those modes.

### Host filesystem path

| Flag | Description | Default |
|---|---|---|
| `--path.rootfs` | Host root filesystem prefix used by the embedded filesystem collector | `/` |

For an advanced direct-binary SONiC installation, keep the default `/`. Only containers that bind the switch root at `/hostfs` should pass `--path.rootfs=/hostfs`.

The default `mgmt` value is intended for SONiC Community OS and startup fails when the selected VRF device is unavailable. In the recommended SONiC container deployment, use host networking and retain `NET_RAW`; see the [Docker deployment guide](deployment-docker-sonic.md).

## Core settings

| Variable | Description | Default |
|---|---|---|
| `REDIS_ADDRESS` | Redis address (`host:port` for TCP) | `localhost:6379` |
| `REDIS_PASSWORD` | Password for Redis | empty |
| `REDIS_NETWORK` | Redis network type (`tcp` or `unix`) | `tcp` |
| `SONIC_DISABLED_METRICS` | Comma-separated full metric names or wildcard patterns to suppress for in-repo SONiC collectors only | empty |

Protect files containing `REDIS_PASSWORD` with restrictive permissions. Avoid diagnostic commands that print the complete process environment. A safe baseline is available at [`examples/sonic-exporter.env`](../examples/sonic-exporter.env).

## Source-side metric filtering

Use `SONIC_DISABLED_METRICS` to suppress metric families from the in-repo SONiC collectors at exporter startup.

- Matching uses full Prometheus metric names only.
- Matching is case-sensitive.
- Tokens are comma-separated and surrounding whitespace is ignored.
- You must restart the exporter after changing this setting. There is no runtime reload.
- This applies only to the in-repo SONiC collectors in this repo. It does not apply to upstream `node_exporter` metrics or FRR wrapper metrics.

Exact-name example:

```bash
SONIC_DISABLED_METRICS=sonic_queue_watermark_bytes_total,sonic_interface_mtu_bytes
```

Wildcard example:

```bash
SONIC_DISABLED_METRICS='sonic_queue_*'
```

Be careful with broad patterns. A wide match can also hide health metrics such as `sonic_queue_collector_success`, `sonic_queue_scrape_duration_seconds`, `sonic_system_collector_success`, or `sonic_system_scrape_duration_seconds` if the full metric names match.

## LLDP collector

| Variable | Description | Default |
|---|---|---|
| `LLDP_ENABLED` | Enable LLDP collector | `true` |
| `LLDP_INCLUDE_MGMT` | Include management interfaces like `eth0` | `true` |
| `LLDP_REFRESH_INTERVAL` | Cache refresh interval | `30s` |
| `LLDP_TIMEOUT` | Timeout for one refresh cycle | `2s` |
| `LLDP_MAX_NEIGHBORS` | Max neighbors exported per refresh | `512` |

## VLAN collector

| Variable | Description | Default |
|---|---|---|
| `VLAN_ENABLED` | Enable VLAN collector | `true` |
| `VLAN_REFRESH_INTERVAL` | Cache refresh interval | `30s` |
| `VLAN_TIMEOUT` | Timeout for one refresh cycle | `2s` |
| `VLAN_MAX_VLANS` | Max VLANs exported per refresh | `1024` |
| `VLAN_MAX_MEMBERS` | Max VLAN members exported per refresh | `8192` |

## LAG collector

| Variable | Description | Default |
|---|---|---|
| `LAG_ENABLED` | Enable LAG collector | `true` |
| `LAG_REFRESH_INTERVAL` | Cache refresh interval | `30s` |
| `LAG_TIMEOUT` | Timeout for one refresh cycle | `2s` |
| `LAG_MAX_LAGS` | Max LAGs exported per refresh | `512` |
| `LAG_MAX_MEMBERS` | Max LAG members exported per refresh | `4096` |

## FDB collector

| Variable | Description | Default |
|---|---|---|
| `FDB_ENABLED` | Enable FDB collector | `false` |
| `FDB_REFRESH_INTERVAL` | Cache refresh interval | `60s` |
| `FDB_TIMEOUT` | Timeout for one refresh cycle | `2s` |
| `FDB_MAX_ENTRIES` | Max ASIC FDB entries processed per refresh | `50000` |
| `FDB_MAX_PORTS` | Max per-port FDB series exported | `1024` |
| `FDB_MAX_VLANS` | Max per-VLAN FDB series exported | `4096` |

## Switch collector

| Variable | Description | Default |
|---|---|---|
| `SWITCH_ENABLED` | Enable switch collector | `true` |
| `SWITCH_REFRESH_INTERVAL` | Cache refresh interval | `60s` |
| `SWITCH_TIMEOUT` | Timeout for one refresh cycle | `2s` |
| `SWITCH_MAX_ENTRIES` | Max switch table entries exported per refresh | `16` |

## Hardware collector

Hardware PSU metrics read `STATE_DB` keys matching `PSU_INFO|PSU*`. `PSU1`, `PSU 1`, `PSU_1`, and `PSU-1` all export the Prometheus label `slot="1"`. Malformed keys with an empty slot are rejected; no metric is emitted with an empty `slot` label.

## Thermal collector

| Variable | Description | Default |
|---|---|---|
| `THERMAL_ENABLED` | Enable thermal collector | `true` |
| `THERMAL_REFRESH_INTERVAL` | Cache refresh interval | `60s` |
| `THERMAL_TIMEOUT` | Timeout for one refresh cycle | `2s` |

## Transceiver collector

The collector reads hardware identity from `STATE_DB` `TRANSCEIVER_INFO` and exports one bounded `sonic_transceiver_identity_info` series per present port. Identity labels are trimmed to remove fixed-width EEPROM padding.

| Variable | Description | Default |
|---|---|---|
| `TRANSCEIVER_ENABLED` | Enable transceiver collector | `true` |
| `TRANSCEIVER_REFRESH_INTERVAL` | Cache refresh interval | `60s` |
| `TRANSCEIVER_TIMEOUT` | Timeout for one refresh cycle | `2s` |
| `TRANSCEIVER_MAX_PORTS` | Max transceiver ports exported per refresh | `1024` |

## Routing collector

| Variable | Description | Default |
|---|---|---|
| `ROUTING_ENABLED` | Enable routing collector | `false` |
| `ROUTING_REFRESH_INTERVAL` | Cache refresh interval | `60s` |
| `ROUTING_TIMEOUT` | Timeout for one refresh cycle | `2s` |
| `ROUTING_MAX_NEIGHBORS` | Max neighbor entries exported per refresh | `50000` |
| `ROUTING_MAX_ROUTES` | Max route entries exported per refresh | `200000` |

## Platform health collector

| Variable | Description | Default |
|---|---|---|
| `PLATFORM_HEALTH_ENABLED` | Enable platform health collector | `false` |
| `PLATFORM_HEALTH_REFRESH_INTERVAL` | Cache refresh interval | `60s` |
| `PLATFORM_HEALTH_TIMEOUT` | Timeout for one refresh cycle | `2s` |
| `PLATFORM_HEALTH_MAX_PROCESSES` | Max process entries exported per refresh | `512` |
| `PLATFORM_HEALTH_MAX_STORAGE_DEVICES` | Max storage devices exported per refresh | `128` |

Process metrics read `STATE_DB` `PROCESS_STATS|<pid>` entries. The collector prefers numeric `%CPU` and `%MEM` values, then falls back to numeric `CPU` and `MEM` values when the preferred fields are absent, empty, or nonnumeric. It omits `sonic_platform_process_cpu_percent` or `sonic_platform_process_memory_percent` when neither candidate for that metric parses.

## System collector (experimental)

| Variable | Description | Default |
|---|---|---|
| `SYSTEM_ENABLED` | Enable system collector | `false` |
| `SYSTEM_REFRESH_INTERVAL` | Cache refresh interval | `60s` |
| `SYSTEM_TIMEOUT` | Timeout for one refresh cycle | `4s` |
| `SYSTEM_COMMAND_ENABLED` | Enable allowlisted read-only command fallback | `true` |
| `SYSTEM_COMMAND_TIMEOUT` | Timeout per command | `2s` |
| `SYSTEM_COMMAND_MAX_OUTPUT_BYTES` | Max bytes read per command | `262144` |
| `SYSTEM_VERSION_FILE` | SONiC version metadata path | `/etc/sonic/sonic_version.yml` |
| `SYSTEM_MACHINE_CONF_FILE` | Machine config path | `/host/machine.conf` |
| `SYSTEM_HOSTNAME_FILE` | Hostname path | `/etc/hostname` |
| `SYSTEM_UPTIME_FILE` | Uptime path | `/proc/uptime` |

Enable in the process environment:

```env
SYSTEM_ENABLED=true
```

System collector exports:

- `sonic_system_identity_info`
- `sonic_system_software_info`
- `sonic_system_uptime_seconds`
- `sonic_system_collector_success`
- `sonic_system_scrape_duration_seconds`
- `sonic_system_cache_age_seconds`

Data source order:

1. Redis (`DEVICE_METADATA|localhost`, `CHASSIS_INFO|chassis 1`)
2. Read-only files (`/etc/sonic/sonic_version.yml`, `/host/machine.conf`, `/etc/hostname`, `/proc/uptime`)
3. Optional allowlisted command fallback (`show platform summary --json`, `show version`, `show platform syseeprom`)

## Docker collector (experimental)

| Variable | Description | Default |
|---|---|---|
| `DOCKER_ENABLED` | Enable docker collector | `false` |
| `DOCKER_REFRESH_INTERVAL` | Cache refresh interval | `60s` |
| `DOCKER_TIMEOUT` | Timeout for one refresh cycle | `2s` |
| `DOCKER_MAX_CONTAINERS` | Max container entries exported per refresh | `128` |
| `DOCKER_SOURCE_STALE_THRESHOLD` | Source age threshold for stale signal | `5m` |

Enable in the process environment:

```env
DOCKER_ENABLED=true
```

Docker collector behavior:

- Reads `STATE_DB` keys `DOCKER_STATS|*` and `DOCKER_STATS|LastUpdateTime`.
- No Docker socket access.
- No writes.
- Controlled label cardinality (`container` only).

## FRR collector

| Variable | Description | Default |
|---|---|---|
| `FRR_ENABLED` | Enable FRR collector wrapper | `false` |
| `FRR_SOCKET_DIR_PATH` | FRR Unix socket directory | `/var/run/frr` |
| `FRR_SOCKET_TIMEOUT` | Timeout for FRR socket access | `20s` |
| `FRR_VTYSH_ENABLED` | Use `vtysh` instead of Unix sockets | `false` |
| `FRR_VTYSH_PATH` | Path to `vtysh` | `/usr/bin/vtysh` |
| `FRR_VTYSH_TIMEOUT` | Timeout for `vtysh` commands | `20s` |
| `FRR_VTYSH_SUDO` | Run `vtysh` through `sudo` | `false` |
| `FRR_VTYSH_OPTIONS` | Extra options passed to `vtysh` | empty |
| `FRR_BGP_ENABLED` | Enable upstream `bgp` collector | `true` |
| `FRR_BGP6_ENABLED` | Enable upstream `bgp6` collector | `false` |
| `FRR_BGPL2VPN_ENABLED` | Enable upstream `bgpl2vpn` collector | `false` |
| `FRR_OSPF_ENABLED` | Enable upstream `ospf` collector | `true` |
| `FRR_OSPF_INSTANCES` | Comma-separated OSPF instance IDs | empty |
| `FRR_BFD_ENABLED` | Enable upstream `bfd` collector | `true` |
| `FRR_ROUTE_ENABLED` | Enable upstream `route` collector | `true` |
| `FRR_ROUTE_DETAILED_ENABLED` | Enable detailed route metrics | `false` |
| `FRR_RPKI_ENABLED` | Enable upstream `rpki` collector | `false` |
| `FRR_VRRP_ENABLED` | Enable upstream `vrrp` collector | `false` |
| `FRR_PIM_ENABLED` | Enable upstream `pim` collector | `false` |
| `FRR_STATUS_ENABLED` | Enable upstream `status` collector | `true` |
| `FRR_BGP_PEER_TYPES_ENABLED` | Enable peer-type aggregate metric | `false` |
| `FRR_BGP_PEER_TYPES_KEYS` | Comma-separated BGP peer-type keys | `type` |
| `FRR_BGP_PEER_DESCRIPTIONS_ENABLED` | Add structured peer descriptions as labels | `false` |
| `FRR_BGP_PEER_DESCRIPTIONS_PLAIN_TEXT` | Use plain-text peer descriptions | `false` |
| `FRR_BGP_PEER_GROUPS_ENABLED` | Add peer group labels | `false` |
| `FRR_BGP_PEER_HOSTNAMES_ENABLED` | Add peer hostname labels | `false` |
| `FRR_BGP_ADVERTISED_PREFIXES_ENABLED` | Query advertised prefix counts for older FRR | `false` |
| `FRR_BGP_ACCEPTED_FILTERED_PREFIXES_ENABLED` | Export accepted and filtered BGP prefix counts | `false` |
| `FRR_BGP_NEXT_HOP_INTERFACE_ENABLED` | Add next-hop interface label | `false` |
| `FRR_BGP_MONITORED_PREFIXES_FILE` | Prefix file for per-peer presence monitoring | empty |

Enable in the process environment:

```env
FRR_ENABLED=true
```

FRR collector behavior:

- Delegates to the upstream `github.com/tynany/frr_exporter` collectors and keeps upstream metric names under the `frr_` namespace.
- Uses current upstream defaults when enabled: `bgp`, `ospf`, `bfd`, `route`, and `status` are on by default; `bgp6`, `bgpl2vpn`, `rpki`, `vrrp`, and `pim` stay opt-in.
- Uses Unix sockets by default and supports `vtysh`, but upstream also recommends leaving `vtysh` disabled unless you need it.
- `RPKI` requires FRR built with `--enable-rpki` upstream.

## Complete startup example

Advanced direct-binary SONiC Community OS installation with TCP Redis:

```bash
REDIS_ADDRESS=127.0.0.1:6379 \
REDIS_NETWORK=tcp \
ROUTING_ENABLED=false \
PLATFORM_HEALTH_ENABLED=false \
SYSTEM_ENABLED=false \
DOCKER_ENABLED=false \
FRR_ENABLED=false \
./sonic-exporter
```

For all exporter-toolkit logging, TLS, and authentication options supported by the current build, run:

```bash
./sonic-exporter --help
```
