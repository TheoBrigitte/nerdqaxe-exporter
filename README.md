<p align="center">
    <img src="assets/nerdqaxe-exporter-square.png" alt="NerdQAxe exporter logo" height="225px">
</p>

<div align="center">

  [![GitHub Release](https://img.shields.io/github/v/release/TheoBrigitte/nerdqaxe-exporter)](https://github.com/TheoBrigitte/nerdqaxe-exporter/releases/latest)
  [![Go Reference](https://pkg.go.dev/badge/github.com/TheoBrigitte/nerdqaxe-exporter.svg)](https://pkg.go.dev/github.com/TheoBrigitte/nerdqaxe-exporter)
  [![ci](https://github.com/TheoBrigitte/nerdqaxe-exporter/actions/workflows/ci.yaml/badge.svg)](https://github.com/TheoBrigitte/nerdqaxe-exporter/actions/workflows/ci.yaml)
  ![GitHub Downloads](https://img.shields.io/github/downloads/TheoBrigitte/nerdqaxe-exporter/total)
  [![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/TheoBrigitte/nerdqaxe-exporter/badge)](https://scorecard.dev/viewer/?uri=github.com/TheoBrigitte/nerdqaxe-exporter)

</div>

## About

Prometheus exporter for the NerdQAxe firmware.

It queries `/api/system/info` on the miner at each Prometheus scrape and exposes
the result as metrics in base units (hashes/second, volts, amperes, watts,
seconds, degrees Celsius).

## Metrics

Each description names the `/api/system/info` field the metric is built from.

| Metric | Description |
| --- | --- |
| `nerdqaxe_up` | Whether the last scrape of the device succeeded |
| `nerdqaxe_scrape_duration_seconds` | Duration of the last query to the device |
| `nerdqaxe_info` | Device identity, always 1. Labels: `device_model`, `asic_model`, `asic_count`, `hostname`, `ip`, `mac`, `version`, `ssid` |
| `nerdqaxe_hashrate_hashes_per_second` | Current hashrate |
| `nerdqaxe_power_watts` | Power drawn by the device |
| `nerdqaxe_input_voltage_volts` | Input voltage |
| `nerdqaxe_input_current_amperes` | Input current |
| `nerdqaxe_core_voltage_volts` | Measured ASIC core voltage |
| `nerdqaxe_core_voltage_configured_volts` | Configured ASIC core voltage |
| `nerdqaxe_asic_frequency_hertz` | Configured ASIC clock frequency |
| `nerdqaxe_shutdown` | Whether the device has shut down mining |
| `nerdqaxe_temperature_celsius` | Temperature per `sensor`: `asic_max`, `voltage_regulator`, `voltage_regulator_internal` |
| `nerdqaxe_asic_temperature_celsius` | Temperature per `asic`, reported as 0 on boards without per-ASIC sensors |
| `nerdqaxe_overheat_temperature_celsius` | Temperature at which the device shuts down |
| `nerdqaxe_fan_speed_rpm` | Fan speed, per `fan` |
| `nerdqaxe_fan_speed_ratio` | Fan speed as a fraction of maximum, per `fan` |
| `nerdqaxe_blocks_found_total` | Blocks found over the lifetime of the device |
| `nerdqaxe_session_blocks_found_total` | Blocks found since the last restart |
| `nerdqaxe_best_difficulty` | Best share difficulty over the lifetime of the device |
| `nerdqaxe_duplicate_hw_nonces_total` | Duplicate nonces returned by the hardware |
| `nerdqaxe_stratum_using_fallback` | Whether the device is mining on the fallback pool |
| `nerdqaxe_stratum_pool_mode` | Active pool mode, 1 for the current `mode`: `failover` or `dual` |
| `nerdqaxe_pool_connected` | Whether the stratum connection is established, per `pool` |
| `nerdqaxe_pool_difficulty` | Share difficulty set by the pool, per `pool` |
| `nerdqaxe_network_difficulty` | Bitcoin network difficulty reported by the pool, per `pool` |
| `nerdqaxe_pool_shares_accepted_total` | Shares accepted by the pool, per `pool` |
| `nerdqaxe_pool_shares_rejected_total` | Shares rejected by the pool, per `pool` |
| `nerdqaxe_pool_best_difficulty` | Best share difficulty submitted to the pool this session, per `pool` |
| `nerdqaxe_pool_ping_rtt_seconds` | Round trip time to the pool, per `pool` |
| `nerdqaxe_pool_ping_loss_ratio` | Fraction of ping packets lost to the pool, per `pool` |
| `nerdqaxe_uptime_seconds` | Time since the last restart |
| `nerdqaxe_wifi_rssi_dbm` | WiFi signal strength |
| `nerdqaxe_free_heap_bytes` | Free heap per `memory` area: `spiram`, `internal` |

A failed device query is reported as `nerdqaxe_up 0` rather than an HTTP error,
so the exporter's own process metrics stay available.

Shares and best session difficulty are per pool. Sum them for a device total:
`sum(rate(nerdqaxe_pool_shares_accepted_total[5m]))`. The device also reports
1m, 10m, 1h and 1d hashrate averages; those are deliberately not exported, since
Prometheus averages more accurately with `avg_over_time()`.

The pool label is the pool's index in the device's pool list. In failover mode the device reports only the pool it is currently using, so pool="0" is the primary pool or the fallback
depending on nerdqaxe_stratum_using_fallback.

All of the above are defined in
[`internal/collector/collector.go`](internal/collector/collector.go).
The fields are built from the NerdQAxe firmware v1.0.37.2-LTS API, which is implemented in
[`main/http_server/handler_system.cpp`](https://github.com/shufps/ESP-Miner-NerdQAxePlus/blob/V1.0.37.2-LTS/main/http_server/handler_system.cpp).
Support for the newer v2 API will be added later.

## Install

Download a binary from the [releases page](https://github.com/TheoBrigitte/nerdqaxe-exporter/releases), or use the
Docker image:

```
$ docker run --rm -p 10055:10055 docker.io/theo01/nerdqaxe-exporter --target http://nerdqaxe.local
{"level":"info","device_model":"NerdQAxe++","asic_model":"BM1370","hostname":"nerdqaxe","version":"V1.0.37.2-LTS","time":"2026-09-13T12:09:14Z","message":"device reachable"}
{"level":"info","address":":10055","path":"/metrics","target":"http://nerdqaxe.local","time":"2026-09-13T12:09:14Z","message":"listening"}
```

## Usage

```
$ nerdqaxe-exporter --log.format console --target http://nerdqaxe.local
INF device reachable asic_model=BM1370 device_model=NerdQAxe++ hostname=nerdqaxe version=V1.0.37.2-LTS
INF listening address=:10055 path=/metrics target=http://nerdqaxe.local
```

| Flag | Default | Description |
| --- | --- | --- |
| `--target`, `-t` | *required* | Base URL of the device, also read from `NERDQAXE_TARGET` |
| `--timeout` | `5s` | Timeout for a device query |
| `--web.listen-address` | `:10055` | Address to listen on for telemetry |
| `--web.metrics-path` | `/metrics` | Path under which to expose metrics |
| `--log.level` | `info` | Only log at or above this level: `debug`, `info`, `warn`, `error` |
| `--log.format` | `json` | Log output format: `json`, `console` |
| `--version`, `-V` | | Print the version and exit |

Scrape config:

```yaml
scrape_configs:
  - job_name: nerdqaxe
    static_configs:
      - targets: ["localhost:10055"]
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
