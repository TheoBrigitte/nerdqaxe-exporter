# NerdQAxe exporter

Prometheus exporter for NerdQAxe firmware.

It scrapes `/api/system/info` on the miner at each Prometheus scrape and exposes
the result as metrics in base units (hashes/second, volts, amperes, watts,
seconds, degrees Celsius), following the Prometheus
[guidelines for writing exporters](https://prometheus.io/docs/instrumenting/writing_exporters/).

## Usage

```
make build
./build/nerdqaxe-exporter --target http://192.0.2.10
```

| Flag | Default | Description |
| --- | --- | --- |
| `--target`, `-t` | *required* | Base URL of the device, also read from `NERDQAXE_TARGET` |
| `--timeout` | `5s` | Timeout for a device query |
| `--web.listen-address` | `:10055` | Address to listen on for telemetry |
| `--web.telemetry-path` | `/metrics` | Path under which to expose metrics |

Scrape config:

```yaml
scrape_configs:
  - job_name: nerdqaxe
    static_configs:
      - targets: ['localhost:10055']
```

One exporter instance serves one miner, as the guidelines prescribe: run one
process per device and let Prometheus do service discovery. Port 10055 is the
next free port in the Prometheus [default port
allocations](https://github.com/prometheus/prometheus/wiki/Default-port-allocations);
claim it there before announcing this exporter publicly.

A failed device query is reported as `nerdqaxe_up 0` rather than an HTTP error,
so the exporter's own process metrics stay available.

## Metrics

All metrics are prefixed with `nerdqaxe_`.

| Metric | Description |
| --- | --- |
| `up` | Whether the last scrape of the device succeeded |
| `scrape_duration_seconds` | How long the device query took |
| `info` | Device identity: model, ASIC model and count, hostname, IP, MAC, firmware version, SSID |
| `hashrate_hashes_per_second` | Current hashrate |
| `power_watts`, `input_voltage_volts`, `input_current_amperes` | Power input |
| `core_voltage_volts`, `core_voltage_configured_volts` | ASIC core voltage, measured and configured |
| `asic_frequency_hertz` | Configured ASIC clock frequency |
| `shutdown` | Whether the device has shut down mining |
| `temperature_celsius` | Temperature by `sensor` (`asic_max`, `voltage_regulator`, `voltage_regulator_internal`) |
| `asic_temperature_celsius` | Temperature per `asic` index, on boards that report it |
| `overheat_temperature_celsius` | Shutdown temperature threshold |
| `fan_speed_rpm`, `fan_speed_ratio` | Fan speed, by `fan` label |
| `blocks_found_total`, `session_blocks_found` | Blocks found, lifetime and since restart |
| `best_difficulty` | Best share difficulty over the lifetime of the device |
| `duplicate_hw_nonces_total` | Duplicate nonces returned by the hardware |
| `stratum_using_fallback`, `stratum_pool_mode` | Pool selection state, `mode` is `failover` or `dual` |
| `pool_connected`, `pool_difficulty`, `network_difficulty` | Per `pool` index |
| `pool_shares_accepted_total`, `pool_shares_rejected_total`, `pool_best_difficulty` | Per `pool` index |
| `pool_ping_rtt_seconds`, `pool_ping_loss_ratio` | Per `pool` index |
| `uptime_seconds`, `wifi_rssi_dbm`, `free_heap_bytes` | Device health |

Shares and best session difficulty are per pool. Sum them for a device total:
`sum(rate(nerdqaxe_pool_shares_accepted_total[5m]))`. The device also reports
1m, 10m, 1h and 1d hashrate averages; those are deliberately not exported, since
Prometheus averages more accurately with `avg_over_time()`.
