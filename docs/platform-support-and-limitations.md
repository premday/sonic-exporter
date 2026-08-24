# Platform support and limitations

This page records the SONiC platform combinations tested with `sonic-exporter`. Metric availability depends on the data each SONiC platform publishes to `STATE_DB`.

## Validated platforms

These combinations were tested with SONiC Community releases. Other releases may work, but they are not claimed as validated here.

| Model | SONiC | OS | Distribution | Kernel | Platform | ASIC |
|---|---:|---:|---|---|---|---|
| DellEMC-S5232f-C8D48 | 202012 | 10 | Debian 10.13 | 4.19.0-12-2-amd64 | x86_64-dellemc_s5232f_c3538-r0 | Broadcom |
| DellEMC-S5232f-C32 | 202605 | 13 | Debian 13.5 | 6.12.41+deb13-sonic-amd64 | x86_64-dellemc_s5232f_c3538-r0 | Broadcom |
| MSN2100-CB2FC | 202411 | 12 | Debian 12.12 | 6.1.0-29-2-amd64 | x86_64-mlnx_msn2100-r0 | Mellanox |
| MSN2100-CB2FC | 202605 | 13 | Debian 13.5 | 6.12.41+deb13-sonic-amd64 | x86_64-mlnx_msn2100-r0 | Mellanox |
| SSE-T7132SR | 3.1.3.0-0004 | 11 | Debian 11.8 | 5.10.0-23-2-amd64 | x86_64-supermicro_sse_t7132s-r0 | Innovium |
| SSE-T7132SR | 202505 | 12 | Debian 12.11 | 6.1.0-29-2-amd64 | x86_64-supermicro_sse_t7132s-r0 | Marvell Teralynx |
| SSE-T7132SR | 202605 | 13 | Debian 13.6 | 6.12.41+deb13-sonic-amd64 | x86_64-supermicro_sse_t7132s-r0 | Marvell Teralynx |

## Metric support matrix

This matrix records live metric-family checks with `sonic-exporter` v0.4.0. `Yes` means the metric family was present. `Partial` means some expected series were unavailable. Optics are marked `Data-dependent` because their series depend on installed modules and the SONiC tables populated for those modules.

| Model and SONiC | PSU status | PSU voltage/current/power | PSU temperature | Fan status | Fan RPM | ASIC temperature | Optics | Storage |
|---|---:|---:|---:|---:|---:|---|---|---:|
| DellEMC-S5232f-C32, 202605 | Yes | Yes | Yes | Yes | Yes | 12 sensors and average/max | Data-dependent | Yes |
| MSN2100-CB2FC, 202605 | Yes | No | No | Yes | Yes | 1 sensor and average/max | Data-dependent | Yes |
| MSN2100-CB2FC, 202411 | Yes | No | No | Yes | Yes | Average/max only | Data-dependent | Yes |
| SSE-T7132SR, 202605 | Yes | Yes | No | Yes | Partial: 6 of 8 RPM values | 5 sensors and average/max | Data-dependent | Yes |
| SSE-T7132SR, 3.1.3.0-0004 | Yes | Yes | Yes | Yes | Yes | 5 sensors and average/max | Data-dependent | No |

The HW, transceiver, thermal, and platform-health collectors reported success on all live checks. The transceiver and thermal collectors reported no skipped or truncated entries.

## Platform and SONiC limitations

### Mellanox MSN2100 PSU sensors

The MSN2100 fixed PSUs expose presence and operational status, but not individual voltage, current, power, or temperature readings. SONiC publishes these fields as `N/A`, so the exporter does not emit the numeric PSU metrics.

The platform has internal 12V board-rail sensors. These are not individual PSU measurements and must not be mapped to PSU slots.

### Supermicro fan and PSU sensors

On the Supermicro 202605 combination, all eight fan status records are present but only six contain numeric RPM readings. PSU temperature is also reported as `N/A`.

### Storage monitoring

Storage metrics require SONiC `stormond` to populate `STORAGE_INFO` in `STATE_DB`. The Supermicro `3.1.3.0-0004` image does not include `stormond`, so `sonic_platform_storage_*` metrics are unavailable even though `show platform ssdhealth` works.

### ASIC temperatures

The Mellanox 202411 combination provides ASIC average and maximum temperatures but does not provide per-sensor ASIC temperatures.

### Optics

The transceiver collector works on all live combinations tested. Identity, status, flag, and threshold metric families vary with installed optics and the data available in SONiC `STATE_DB`.

### Process collection limit

The 202605 lab combinations have more than the default limit of 512 process records. The collector remains successful but reports `sonic_platform_entries_truncated 1`. Increase `PLATFORM_HEALTH_MAX_PROCESSES` only after considering the extra metric volume.
