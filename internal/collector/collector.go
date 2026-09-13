// Package collector exposes NerdQAxe device state as Prometheus metrics.
package collector

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/TheoBrigitte/nerdqaxe-exporter/internal/nerdqaxe"
)

const namespace = "nerdqaxe"

// Unit conversions from what the device reports to Prometheus base units.
const (
	gigahashPerHash      = 1e9
	hertzPerMegahertz    = 1e6
	millivoltPerVolt     = 1e3
	millisecondPerSecond = 1e3
	percentPerRatio      = 1e2
)

// poolModes maps the device pool mode to a label value. The modes are
// FAILOVER and DUAL, see StratumManager::PoolMode in the firmware.
var poolModes = map[int]string{0: "failover", 1: "dual"}

func desc(name, help string, labels ...string) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName(namespace, "", name), help, labels, nil)
}

// Metric descriptions name the field of /api/system/info they are built from,
// so that users can trace a metric back to the device API.
var (
	up             = desc("up", "Whether the last scrape of the device succeeded.")
	scrapeDuration = desc("scrape_duration_seconds", "Duration of the last query to the device.")
	info           = desc("info", "Device identity, always 1 (deviceModel, ASICModel, asicCount, hostname, hostip, macAddr, version, ssid).",
		"device_model", "asic_model", "asic_count", "hostname", "ip", "mac", "version", "ssid")

	hashRate = desc("hashrate_hashes_per_second", "Current hashrate (hashRate).")

	power                 = desc("power_watts", "Power drawn by the device (power).")
	inputVoltage          = desc("input_voltage_volts", "Input voltage (voltage).")
	inputCurrent          = desc("input_current_amperes", "Input current (currentA).")
	coreVoltage           = desc("core_voltage_volts", "Measured ASIC core voltage (coreVoltageActual).")
	coreVoltageConfigured = desc("core_voltage_configured_volts", "Configured ASIC core voltage (coreVoltage).")
	frequency             = desc("asic_frequency_hertz", "Configured ASIC clock frequency (frequency).")
	shutdown              = desc("shutdown", "Whether the device has shut down mining (shutdown).")

	temperature  = desc("temperature_celsius", "Temperature per sensor (temp, vrTemp, vrTempInt).", "sensor")
	asicTemp     = desc("asic_temperature_celsius", "Temperature per ASIC (asicTemps), reported as 0 on boards without per-ASIC sensors; the asic_max sensor of nerdqaxe_temperature_celsius is what the device acts on.", "asic")
	overheatTemp = desc("overheat_temperature_celsius", "Temperature at which the device shuts down (overheat_temp).")

	fanRPM   = desc("fan_speed_rpm", "Fan speed (fans.rpm).", "fan")
	fanRatio = desc("fan_speed_ratio", "Fan speed as a fraction of maximum (fans.speedPerc).", "fan")

	blocksFound        = desc("blocks_found_total", "Blocks found over the lifetime of the device (totalFoundBlocks).")
	sessionBlocksFound = desc("session_blocks_found_total", "Blocks found since the last restart (foundBlocks).")
	bestDifficulty     = desc("best_difficulty", "Best share difficulty over the lifetime of the device (bestDiff).")
	duplicateNonces    = desc("duplicate_hw_nonces_total", "Duplicate nonces returned by the hardware (duplicateHWNonces).")

	usingFallback = desc("stratum_using_fallback", "Whether the device is mining on the fallback pool (stratum.usingFallback).")
	poolMode      = desc("stratum_pool_mode", "Active pool mode, 1 for the current mode (stratum.activePoolMode).", "mode")

	poolLabelHelp     = " In failover mode the device reports only the selected pool, so pool 0 is whichever pool is active."
	poolConnected     = desc("pool_connected", "Whether the stratum connection is established (stratum.pools.connected)."+poolLabelHelp, "pool")
	poolDifficulty    = desc("pool_difficulty", "Share difficulty set by the pool (stratum.pools.poolDifficulty)."+poolLabelHelp, "pool")
	networkDifficulty = desc("network_difficulty", "Bitcoin network difficulty reported by the pool (stratum.pools.networkDifficulty)."+poolLabelHelp, "pool")
	poolAccepted      = desc("pool_shares_accepted_total", "Shares accepted by the pool (stratum.pools.accepted)."+poolLabelHelp, "pool")
	poolRejected      = desc("pool_shares_rejected_total", "Shares rejected by the pool (stratum.pools.rejected)."+poolLabelHelp, "pool")
	poolBestDiff      = desc("pool_best_difficulty", "Best share difficulty submitted to the pool this session (stratum.pools.bestDiff)."+poolLabelHelp, "pool")
	poolPingRTT       = desc("pool_ping_rtt_seconds", "Round trip time to the pool (stratum.pools.pingRtt)."+poolLabelHelp, "pool")
	poolPingLoss      = desc("pool_ping_loss_ratio", "Fraction of ping packets lost to the pool (stratum.pools.pingLoss)."+poolLabelHelp, "pool")

	uptime   = desc("uptime_seconds", "Time since the last restart (uptimeSeconds).")
	wifiRSSI = desc("wifi_rssi_dbm", "WiFi signal strength (wifiRSSI).")
	freeHeap = desc("free_heap_bytes", "Free heap per memory area (freeHeap, freeHeapInt).", "memory")
)

// Collector scrapes a NerdQAxe device on every Prometheus collection.
type Collector struct {
	client *nerdqaxe.Client
	logger *slog.Logger
	ctx    context.Context
}

// New returns a Collector scraping the device behind client.
func New(client *nerdqaxe.Client, logger *slog.Logger) *Collector {
	return &Collector{client: client, logger: logger, ctx: context.Background()}
}

// WithContext returns a copy of the Collector whose scrapes run under ctx, so
// that a device query stops when Prometheus gives up on the scrape.
func (c *Collector) WithContext(ctx context.Context) *Collector {
	clone := *c
	clone.ctx = ctx
	return &clone
}

// Describe implements prometheus.Collector. It sends every descriptor the
// collector can emit, so that inconsistencies are caught at registration time.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
	ch <- scrapeDuration
	ch <- info

	ch <- hashRate

	ch <- power
	ch <- inputVoltage
	ch <- inputCurrent
	ch <- coreVoltage
	ch <- coreVoltageConfigured
	ch <- frequency
	ch <- shutdown

	ch <- temperature
	ch <- asicTemp
	ch <- overheatTemp

	ch <- fanRPM
	ch <- fanRatio

	ch <- blocksFound
	ch <- sessionBlocksFound
	ch <- bestDifficulty
	ch <- duplicateNonces

	ch <- usingFallback
	ch <- poolMode

	ch <- poolConnected
	ch <- poolDifficulty
	ch <- networkDifficulty
	ch <- poolAccepted
	ch <- poolRejected
	ch <- poolBestDiff
	ch <- poolPingRTT
	ch <- poolPingLoss

	ch <- uptime
	ch <- wifiRSSI
	ch <- freeHeap
}

// Collect implements prometheus.Collector. It queries the device once, so that
// scrapes stay synchronous with Prometheus and no state is shared between them.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	i, err := c.client.SystemInfo(c.ctx)
	ch <- gauge(scrapeDuration, time.Since(start).Seconds())

	if err != nil {
		c.logger.Error("scrape failed", "target", c.client.Target(), "err", err)
		ch <- gauge(up, 0)
		return
	}
	ch <- gauge(up, 1)

	ch <- gauge(info, 1,
		i.DeviceModel, i.ASICModel, strconv.Itoa(i.ASICCount),
		i.Hostname, i.HostIP, i.MacAddr, i.Version, i.SSID)

	ch <- gauge(hashRate, i.HashRate*gigahashPerHash)

	ch <- gauge(power, i.Power)
	ch <- gauge(inputVoltage, i.Voltage/millivoltPerVolt)
	ch <- gauge(inputCurrent, i.CurrentA)
	ch <- gauge(coreVoltage, i.CoreVoltageActual/millivoltPerVolt)
	ch <- gauge(coreVoltageConfigured, i.CoreVoltage/millivoltPerVolt)
	ch <- gauge(frequency, i.Frequency*hertzPerMegahertz)
	ch <- gauge(shutdown, boolToFloat(i.Shutdown))

	ch <- gauge(temperature, i.Temp, "asic_max")
	ch <- gauge(temperature, i.VRTemp, "voltage_regulator")
	ch <- gauge(temperature, i.VRTempInt, "voltage_regulator_internal")
	for n, temp := range i.ASICTemps {
		ch <- gauge(asicTemp, temp, strconv.Itoa(n))
	}
	ch <- gauge(overheatTemp, i.OverheatTemp)

	for _, fan := range i.Fans {
		ch <- gauge(fanRPM, fan.RPM, fan.Label)
		ch <- gauge(fanRatio, fan.SpeedPerc/percentPerRatio, fan.Label)
	}

	ch <- counter(blocksFound, i.TotalFoundBlocks)
	ch <- counter(sessionBlocksFound, i.FoundBlocks)
	ch <- gauge(bestDifficulty, i.BestDiff)
	ch <- counter(duplicateNonces, i.DuplicateHWNonces)

	ch <- gauge(usingFallback, boolToFloat(i.Stratum.UsingFallback))
	for mode, label := range poolModes {
		ch <- gauge(poolMode, boolToFloat(mode == i.Stratum.PoolMode), label)
	}

	for n, pool := range i.Stratum.Pools {
		id := strconv.Itoa(n)
		ch <- gauge(poolConnected, boolToFloat(pool.Connected), id)
		ch <- gauge(poolDifficulty, pool.PoolDifficulty, id)
		ch <- gauge(networkDifficulty, pool.NetworkDifficulty, id)
		ch <- counter(poolAccepted, pool.Accepted, id)
		ch <- counter(poolRejected, pool.Rejected, id)
		ch <- gauge(poolBestDiff, pool.BestDiff, id)
		ch <- gauge(poolPingRTT, pool.PingRTT/millisecondPerSecond, id)
		ch <- gauge(poolPingLoss, pool.PingLoss, id)
	}

	ch <- gauge(uptime, i.UptimeSeconds)
	ch <- gauge(wifiRSSI, i.WifiRSSI)
	ch <- gauge(freeHeap, i.FreeHeap, "spiram")
	ch <- gauge(freeHeap, i.FreeHeapInt, "internal")
}

func gauge(d *prometheus.Desc, v float64, labels ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v, labels...)
}

func counter(d *prometheus.Desc, v float64, labels ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(d, prometheus.CounterValue, v, labels...)
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
