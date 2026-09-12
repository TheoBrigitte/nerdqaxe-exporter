// Package collector exposes NerdQAxe device state as Prometheus metrics.
package collector

import (
	"context"
	"log/slog"
	"strconv"

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

func desc(name, help string, labels ...string) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName(namespace, "", name), help, labels, nil)
}

var (
	up                    = desc("up", "Whether the last scrape of the device succeeded.")
	info                  = desc("info", "Device identity, always 1.", "device_model", "asic_model", "asic_count", "hostname", "ip", "mac", "version", "ssid")
	hashRate              = desc("hashrate_hashes_per_second", "Hashrate reported by the device.", "window")
	power                 = desc("power_watts", "Power drawn by the device.")
	inputVoltage          = desc("input_voltage_volts", "Input voltage.")
	inputCurrent          = desc("input_current_amperes", "Input current.")
	coreVoltage           = desc("core_voltage_volts", "ASIC core voltage, configured and measured.", "kind")
	frequency             = desc("asic_frequency_hertz", "ASIC clock frequency.")
	shutdown              = desc("shutdown", "Whether the device has shut down mining.")
	temperature           = desc("temperature_celsius", "Temperature per sensor.", "sensor")
	asicTemp              = desc("asic_temperature_celsius", "Temperature per ASIC.", "asic")
	overheatTemp          = desc("overheat_temperature_celsius", "Temperature at which the device shuts down.")
	fanRPM                = desc("fan_speed_rpm", "Fan speed.", "fan")
	fanRatio              = desc("fan_speed_ratio", "Fan speed as a fraction of maximum.", "fan")
	sharesAccepted        = desc("shares_accepted_total", "Shares accepted by the pools.")
	sharesRejected        = desc("shares_rejected_total", "Shares rejected by the pools.")
	blocksFound           = desc("blocks_found_total", "Blocks found over the lifetime of the device.")
	sessionBlocksFound    = desc("session_blocks_found", "Blocks found since the last restart.")
	bestDifficulty        = desc("best_difficulty", "Best share difficulty over the lifetime of the device.")
	bestSessionDifficulty = desc("best_session_difficulty", "Best share difficulty since the last restart.")
	duplicateNonces       = desc("duplicate_hw_nonces_total", "Duplicate nonces returned by the hardware.")
	usingFallback         = desc("stratum_using_fallback", "Whether the device is mining on the fallback pool.")
	poolMode              = desc("stratum_pool_mode", "Active pool mode.")
	poolConnected         = desc("pool_connected", "Whether the stratum connection is established.", "pool")
	poolDifficulty        = desc("pool_difficulty", "Share difficulty set by the pool.", "pool")
	networkDifficulty     = desc("network_difficulty", "Bitcoin network difficulty reported by the pool.", "pool")
	poolAccepted          = desc("pool_shares_accepted_total", "Shares accepted by the pool.", "pool")
	poolRejected          = desc("pool_shares_rejected_total", "Shares rejected by the pool.", "pool")
	poolBestDiff          = desc("pool_best_difficulty", "Best share difficulty submitted to the pool this session.", "pool")
	poolPingRTT           = desc("pool_ping_rtt_seconds", "Round trip time to the pool.", "pool")
	poolPingLoss          = desc("pool_ping_loss_ratio", "Fraction of ping packets lost to the pool.", "pool")
	uptime                = desc("uptime_seconds", "Time since the last restart.")
	wifiRSSI              = desc("wifi_rssi_dbm", "WiFi signal strength.")
	freeHeap              = desc("free_heap_bytes", "Free heap per memory region.", "region")
	pingRTT               = desc("ping_rtt_seconds", "Round trip time of the last ping.")
	pingLoss              = desc("ping_loss_ratio", "Fraction of ping packets recently lost.")
)

// Collector scrapes a NerdQAxe device on every Prometheus collection.
type Collector struct {
	client *nerdqaxe.Client
	logger *slog.Logger
}

// New returns a Collector scraping the device behind client.
func New(client *nerdqaxe.Client, logger *slog.Logger) *Collector {
	return &Collector{client: client, logger: logger}
}

// Describe implements prometheus.Collector. It is left unimplemented so that
// metrics are only described from what an actual scrape returns.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {}

// Collect implements prometheus.Collector.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	i, err := c.client.SystemInfo(context.Background())
	if err != nil {
		c.logger.Error("scrape failed", "target", c.client.Target(), "err", err)
		ch <- gauge(up, 0)
		return
	}
	ch <- gauge(up, 1)

	ch <- gauge(info, 1,
		i.DeviceModel, i.ASICModel, strconv.Itoa(i.ASICCount),
		i.Hostname, i.HostIP, i.MacAddr, i.Version, i.SSID)

	ch <- gauge(hashRate, i.HashRate*gigahashPerHash, "current")
	ch <- gauge(hashRate, i.HashRate1m*gigahashPerHash, "1m")
	ch <- gauge(hashRate, i.HashRate10m*gigahashPerHash, "10m")
	ch <- gauge(hashRate, i.HashRate1h*gigahashPerHash, "1h")
	ch <- gauge(hashRate, i.HashRate1d*gigahashPerHash, "1d")

	ch <- gauge(power, i.Power)
	ch <- gauge(inputVoltage, i.Voltage/millivoltPerVolt)
	ch <- gauge(inputCurrent, i.CurrentA)
	ch <- gauge(coreVoltage, i.CoreVoltage/millivoltPerVolt, "configured")
	ch <- gauge(coreVoltage, i.CoreVoltageActual/millivoltPerVolt, "measured")
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

	ch <- counter(sharesAccepted, i.SharesAccepted)
	ch <- counter(sharesRejected, i.SharesRejected)
	ch <- counter(blocksFound, i.TotalFoundBlocks)
	ch <- gauge(sessionBlocksFound, i.FoundBlocks)
	ch <- gauge(bestDifficulty, i.BestDiff)
	ch <- gauge(bestSessionDifficulty, i.BestSessionDiff)
	ch <- counter(duplicateNonces, i.DuplicateHWNonces)

	ch <- gauge(usingFallback, boolToFloat(i.Stratum.UsingFallback))
	ch <- gauge(poolMode, float64(i.Stratum.PoolMode))
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
	ch <- gauge(pingRTT, i.LastPingRTT/millisecondPerSecond)
	ch <- gauge(pingLoss, i.RecentPingLos)
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
