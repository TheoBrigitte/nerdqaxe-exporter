# NerdQAxe exporter

Prometheus exporter for NerdQAxe firmware.

It scrapes `/api/system/info` on the miner at each Prometheus scrape and exposes
the result as metrics in base units (hashes/second, volts, amperes, watts,
seconds, degrees Celsius).

## Usage

```
make build
./build/nerdqaxe-exporter --target http://192.0.2.10
```

| Flag | Default | Description |
| --- | --- | --- |
| `--target`, `-t` | *required* | Base URL of the device, also read from `NERDQAXE_TARGET` |
| `--timeout` | `5s` | Timeout for a device query |
| `--web.listen-address` | `:9800` | Address to listen on for telemetry |
| `--web.telemetry-path` | `/metrics` | Path under which to expose metrics |

Scrape config:

```yaml
scrape_configs:
  - job_name: nerdqaxe
    static_configs:
      - targets: ['localhost:9800']
```

One exporter instance serves one miner.

## Metrics

All metrics are prefixed with `nerdqaxe_`.

| Metric | Description |
| --- | --- |
| `up` | Whether the last scrape of the device succeeded |
| `info` | Device identity: model, ASIC model and count, hostname, IP, MAC, firmware version, SSID |
| `hashrate_hashes_per_second` | Hashrate, by `window` (`current`, `1m`, `10m`, `1h`, `1d`) |
| `power_watts`, `input_voltage_volts`, `input_current_amperes` | Power input |
| `core_voltage_volts` | ASIC core voltage, `kind` `configured` or `measured` |
| `asic_frequency_hertz` | ASIC clock frequency |
| `shutdown` | Whether the device has shut down mining |
| `temperature_celsius` | Temperature by `sensor` (`asic_max`, `voltage_regulator`, `voltage_regulator_internal`) |
| `asic_temperature_celsius` | Temperature per `asic` index, on boards that report it |
| `overheat_temperature_celsius` | Shutdown temperature threshold |
| `fan_speed_rpm`, `fan_speed_ratio` | Fan speed, by `fan` label |
| `shares_accepted_total`, `shares_rejected_total` | Shares across all pools |
| `blocks_found_total`, `session_blocks_found` | Blocks found, lifetime and since restart |
| `best_difficulty`, `best_session_difficulty` | Best share difficulty, lifetime and since restart |
| `duplicate_hw_nonces_total` | Duplicate nonces returned by the hardware |
| `stratum_using_fallback`, `stratum_pool_mode` | Pool selection state |
| `pool_connected`, `pool_difficulty`, `network_difficulty` | Per `pool` index |
| `pool_shares_accepted_total`, `pool_shares_rejected_total`, `pool_best_difficulty` | Per `pool` index |
| `pool_ping_rtt_seconds`, `pool_ping_loss_ratio` | Per `pool` index |
| `uptime_seconds`, `wifi_rssi_dbm`, `free_heap_bytes` | Device health |
| `ping_rtt_seconds`, `ping_loss_ratio` | Connectivity to the active pool |
