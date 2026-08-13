# SONiC Exporter Grafana Dashboard

This dashboard gives operators a detailed view of one SONiC switch. It covers switch identity, host health, interfaces, queues, hardware, optics, ASIC capacity, topology, containers, FDB state, platform health, storage, and exporter diagnostics.

The checked-in file uses Grafana's `dashboard.grafana.app/v2` resource format. It contains 109 panels arranged in 15 rows.

## Requirements

- A Grafana version that supports `dashboard.grafana.app/v2` resources.
- A Prometheus data source that scrapes `sonic-exporter`.
- The default SONiC and embedded node exporter collectors.
- Optional collectors for the panels that depend on them.

## Import in Grafana

The file is a Grafana resource, not a classic dashboard JSON export. Apply `dashboards/sonic-exporter.json` with Grafana's resource API or a compatible tool such as `grafanactl`.

For example, after configuring a `grafanactl` context, place the dashboard in a resource directory and push that directory:

```bash
grafanactl config use-context YOUR_CONTEXT
grafanactl resources push -p ./resources/
```

The exact setup depends on your Grafana deployment. Do not send this file to the legacy dashboard import endpoint without converting it to that endpoint's expected format.

## Variables

The dashboard has three single-value variables:

- `datasource` selects the Prometheus data source.
- `job` lists `job` labels found on `sonic_interface_collector_success`.
- `instance` lists `instance` labels for the selected job.

The variables refresh when the dashboard loads and are sorted alphabetically. Select them from left to right: data source, job, then instance.

## Dashboard layout

The dashboard uses this row order:

| Row | Default | Main content |
|---|---|---|
| Overview | Expanded | Switch identity, SONiC version, uptime, collector health, host health, links, hardware, optics, FDB, platform health, and storage summaries |
| Interfaces / Traffic | Expanded | Interface state, throughput, utilization, packet mix, errors, discards, pause frames, and frame sizes |
| Queues / QoS (platform-dependent) | Collapsed | Queue throughput, packet rates, drops, and persistent watermarks |
| Hardware / Environment (platform-dependent) | Collapsed | PSU inventory and state, fan speed and state, and ASIC temperature |
| Hardware / PSU Telemetry (platform-dependent) | Collapsed | PSU voltage, current, power, and temperature |
| Platform Health / Storage (platform-dependent) | Collapsed | Platform health issues, collector completeness, storage inventory, health, temperature, and I/O |
| Optics / Transceivers (platform-dependent) | Expanded | Transceiver state, alarms, identity, module metadata, optical readings, and DOM thresholds |
| Capacity / ASIC (CRM) (platform-dependent) | Collapsed | CRM and ACL resource use and availability |
| Topology / L2 | Expanded | LLDP neighbors, VLAN state, LAG state, and LAG members |
| Topology / Membership Inventory | Collapsed | VLAN membership inventory |
| Host / OS Detail | Expanded | CPU, load, memory, filesystem, disk throughput, IOPS, busy time, interrupts, and context switches |
| Containers / Docker | Collapsed | Container inventory, source state, CPU, memory, PIDs, and block I/O |
| FDB | Expanded | FDB totals, skipped or truncated data, breakdowns, and comparison with CRM |
| Switch Settings | Collapsed | FDB aging, ordered ECMP, hash seeds, and hash offsets |
| Exporter Diagnostics | Collapsed | Collector state, scrape duration, cache age, skipped or truncated data, Docker source state, and node exporter collector state |

The default time range is the last 12 hours. The dashboard uses the browser timezone and refreshes every minute.

## Optional and platform-dependent data

Some panels can show no data without indicating a dashboard problem:

- System collector metrics provide switch identity, SONiC version, and SONiC uptime.
- FDB collector metrics provide the FDB summary and FDB row.
- Docker collector metrics provide the Containers / Docker row.
- Platform health collector metrics provide platform health and storage panels.
- Queue, hardware, thermal, transceiver, CRM, LLDP, VLAN, and LAG data can vary by SONiC platform and release.
- Host filesystem panels expect node exporter filesystem metrics with `mountpoint="/host"`.

The Overview collector health panel expects 14 SONiC collector families. It reports `Degraded` when optional collector metrics are absent, even when the enabled collectors are healthy. Use the Exporter Diagnostics row to inspect individual collector results.

## Validation

Run the dashboard validator after changing the JSON:

```bash
./scripts/validate-dashboard.sh dashboards/sonic-exporter.json
```

The script checks that the JSON parses, the title and required variables are present, the dashboard contains elements, data source references are portable, and obvious private target values are absent.

## Known limits

- This dashboard is for one switch at a time, not a fleet summary.
- Panel queries filter on the selected `job` and `instance` values.
- Top lists intentionally limit the number of displayed interfaces, queues, disks, containers, VLANs, and FDB groups.
- Platform-dependent or disabled collectors can leave panels empty.
- Keep the dashboard portable. Do not add hard-coded hostnames, site names, private IP addresses, or fixed data source identifiers.
