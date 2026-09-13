# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/)
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Prometheus exporter for the NerdQAxe firmware.
- Device query on `/api/system/info` at each scrape.
- Several devices in one exporter, with a repeatable `--target`, queried in
  parallel on each scrape.
- Metrics in base units for hashrate, power, voltage, current, ASIC frequency,
  temperature, fan speed, blocks found, share difficulty, stratum pools,
  uptime, WiFi signal and free heap.
- Labels `pool`, `asic`, `fan`, `sensor` and `memory`.
- Metrics `nerdqaxe_up` and `nerdqaxe_scrape_duration_seconds`.
- Value `nerdqaxe_up 0` on a failed device query.
- Go, process and build-info metrics for the exporter.
- Target check at startup.
- Scrapes bound to the request context.
- Graceful shutdown on SIGINT and SIGTERM.
- Landing page on `/`.
- Linux binaries for amd64 and arm64.
- Image `docker.io/theo01/nerdqaxe-exporter`.
- Grafana dashboard

[Unreleased]: https://github.com/TheoBrigitte/nerdqaxe-exporter/tree/main
