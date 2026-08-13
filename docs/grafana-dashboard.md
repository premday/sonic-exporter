# SONiC Exporter Grafana Dashboard

This dashboard gives operators a single-switch view of SONiC exporter health, traffic, and service state.
It uses Grafana's `dashboard.grafana.app/v2` resource format and is meant to be portable across Prometheus setups.

## What the dashboard does

The dashboard helps you check whether `sonic-exporter` is collecting data, which collectors are active, and whether the main SONiC subsystems are healthy.
It is focused on drilldown for one switch at a time, not fleet overview.

## Requirements

- A Grafana version that supports the `dashboard.grafana.app/v2` dashboard resource format.
- A Prometheus data source that scrapes `sonic-exporter`.
- Metrics from the default collectors, plus any optional collectors you enable.

## Import in Grafana

Import `dashboards/sonic-exporter.json` using Grafana's dashboard resource API or another tool that supports the `dashboard.grafana.app/v2` format.
Set the `datasource` variable to the Prometheus data source used for `sonic-exporter`.

## Provisioning option

You can also provision the dashboard from `dashboards/sonic-exporter.json`.
Keep the file path stable and point Grafana at the checked-in JSON so the dashboard stays in sync with the repo.

## Variables

The dashboard uses these variables only:

- `datasource`, the Prometheus data source for `sonic-exporter`.
- `job`, the Prometheus job label for the exporter scrape target.
- `instance`, the Prometheus instance label for one switch.

Do not add site, role, or hostname variables unless the dashboard design changes later.

## Row order

The checked-in dashboard uses this final row order:

- `Overview / Exporter Health`
- `Interfaces / Queues`
- `Hardware / CRM / Host`
- `Topology / L2`
- `Optional / FDB`
- `Optional / System`
- `Optional / Docker`
- `Optional / FRR`

The first four rows are expanded by default. They cover exporter health, interface and queue traffic, hardware, CRM, host health, LLDP, VLAN, and LAG signals.

The optional FDB, System, Docker, and FRR rows are collapsed by default. They do not need to return data for the default dashboard to be useful.

## Validation command

Run these commands after changing the dashboard JSON:

```bash
jq empty dashboards/sonic-exporter.json
./scripts/validate-dashboard.sh dashboards/sonic-exporter.json
go test ./...
```

## Known limits and cardinality warnings

- This dashboard is for one switch at a time, not a fleet summary.
- Keep labels bounded. Do not add high-cardinality labels to panels.
- Optional collectors may be absent on some switches, so panels for them should not be required for the dashboard to load.
- The dashboard should stay portable, so avoid hard-coded hostnames, site names, or private IPs in the JSON or docs.
