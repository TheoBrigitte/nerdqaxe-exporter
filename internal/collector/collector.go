// Package collector exposes NerdQAxe device state as Prometheus metrics.
package collector

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

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

// dualPoolMode is the pool mode in which the device mines to both pools at
// once and reports each of them in stratum.pools.
const dualPoolMode = 1

const (
	rolePrimary  = "primary"
	roleFallback = "fallback"
)

// poolRole names the configured pool an entry of stratum.pools belongs to. In
// dual mode the device reports both pools in slot order, primary first. In
// failover mode it reports only the pool it has selected, which usingFallback
// names. See StratumManagerDualPool and StratumManagerFallback in the firmware.
func poolRole(s nerdqaxe.Stratum, n int) string {
	if s.PoolMode == dualPoolMode {
		if n == dualPoolMode {
			return roleFallback
		}
		return rolePrimary
	}
	if s.UsingFallback {
		return roleFallback
	}
	return rolePrimary
}

// stratumIdentity returns the configured identity of a pool role, in the label
// order shared by stratumInfo and poolConnected.
func stratumIdentity(i *nerdqaxe.SystemInfo, role string) []string {
	if role == roleFallback {
		return []string{i.FallbackStratumURL, strconv.Itoa(i.FallbackStratumPort),
			strconv.FormatBool(i.FallbackStratumTLS), stratumProtocol(i.FallbackStratumProtocol)}
	}
	return []string{i.StratumURL, strconv.Itoa(i.StratumPort),
		strconv.FormatBool(i.StratumTLS), stratumProtocol(i.StratumProtocol)}
}

// stratumProtocols maps the device stratum protocol to a label value, see
// StratumProtocol in the firmware.
var stratumProtocols = map[int]string{0: "stratum_v1", 1: "stratum_v2"}

func stratumProtocol(p int) string {
	if name, ok := stratumProtocols[p]; ok {
		return name
	}
	return strconv.Itoa(p)
}

// The help of the two labels shared by several pool metrics.
const (
	poolLabelHelp = " In failover mode the device reports only the selected pool, so pool 0 is whichever pool is active."
	roleLabelHelp = " The role tells which configured pool this is."
)

// metrics are the descriptors of one device. Each description names the field
// of /api/system/info the metric is built from, so that users can trace a
// metric back to the device API.
type metrics struct {
	up                    *prometheus.Desc
	scrapeDuration        *prometheus.Desc
	info                  *prometheus.Desc
	hashRate              *prometheus.Desc
	power                 *prometheus.Desc
	inputVoltage          *prometheus.Desc
	inputCurrent          *prometheus.Desc
	coreVoltage           *prometheus.Desc
	coreVoltageConfigured *prometheus.Desc
	frequency             *prometheus.Desc
	shutdown              *prometheus.Desc
	temperature           *prometheus.Desc
	asicTemp              *prometheus.Desc
	overheatTemp          *prometheus.Desc
	fanRPM                *prometheus.Desc
	fanRatio              *prometheus.Desc
	blocksFound           *prometheus.Desc
	sessionBlocksFound    *prometheus.Desc
	bestDifficulty        *prometheus.Desc
	sessionBestDiff       *prometheus.Desc
	duplicateNonces       *prometheus.Desc
	stratumInfo           *prometheus.Desc
	usingFallback         *prometheus.Desc
	poolMode              *prometheus.Desc
	poolConnected         *prometheus.Desc
	poolDifficulty        *prometheus.Desc
	networkDifficulty     *prometheus.Desc
	poolAccepted          *prometheus.Desc
	poolRejected          *prometheus.Desc
	poolBestDiff          *prometheus.Desc
	poolPingRTT           *prometheus.Desc
	poolPingLoss          *prometheus.Desc
	uptime                *prometheus.Desc
	wifiRSSI              *prometheus.Desc
	freeHeap              *prometheus.Desc
}

// deviceLabels name the device a metric comes from, so that metrics from
// several devices can be told apart without joining on nerdqaxe_info. The
// device names itself, so they are variable labels whose values come from the
// response of each scrape, and they come last in the label values of a metric.
var deviceLabels = []string{"hostname", "mac"}

// newMetrics builds the descriptors of the collector.
func newMetrics() metrics {
	desc := func(name, help string, labels ...string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, "", name), help, append(labels, deviceLabels...), nil)
	}

	return metrics{
		up:             desc("up", "Whether the last scrape of the device succeeded."),
		scrapeDuration: desc("scrape_duration_seconds", "Duration of the last query to the device."),
		info: desc("info", "Device identity, always 1 (deviceModel, ASICModel, asicCount, hostip, version, ssid); hostname and macAddr label every metric.",
			"device_model", "asic_model", "asic_count", "ip", "version", "ssid"),

		hashRate: desc("hashrate_hashes_per_second", "Current hashrate (hashRate)."),

		power:                 desc("power_watts", "Power drawn by the device (power)."),
		inputVoltage:          desc("input_voltage_volts", "Input voltage (voltage)."),
		inputCurrent:          desc("input_current_amperes", "Input current (currentA)."),
		coreVoltage:           desc("core_voltage_volts", "Measured ASIC core voltage (coreVoltageActual)."),
		coreVoltageConfigured: desc("core_voltage_configured_volts", "Configured ASIC core voltage (coreVoltage)."),
		frequency:             desc("asic_frequency_hertz", "Configured ASIC clock frequency (frequency)."),
		shutdown:              desc("shutdown", "Whether the device has shut down mining (shutdown)."),

		temperature:  desc("temperature_celsius", "Temperature per sensor (temp, vrTemp, vrTempInt).", "sensor"),
		asicTemp:     desc("asic_temperature_celsius", "Temperature per ASIC (asicTemps), reported as 0 on boards without per-ASIC sensors; the asic_max sensor of nerdqaxe_temperature_celsius is what the device acts on.", "asic"),
		overheatTemp: desc("overheat_temperature_celsius", "Temperature at which the device shuts down (overheat_temp)."),

		fanRPM:   desc("fan_speed_rpm", "Fan speed (fans.rpm).", "fan"),
		fanRatio: desc("fan_speed_ratio", "Fan speed as a fraction of maximum (fans.speedPerc).", "fan"),

		blocksFound:        desc("blocks_found_total", "Blocks found over the lifetime of the device (totalFoundBlocks)."),
		sessionBlocksFound: desc("session_blocks_found_total", "Blocks found since the last restart (foundBlocks)."),
		bestDifficulty:     desc("best_difficulty", "Best share difficulty over the lifetime of the device (bestDiff)."),
		sessionBestDiff:    desc("session_best_difficulty", "Best share difficulty since the last restart (bestSessionDiff)."),
		duplicateNonces:    desc("duplicate_hw_nonces_total", "Duplicate nonces returned by the hardware (duplicateHWNonces)."),

		stratumInfo: desc("stratum_info", "Configured stratum pool, always 1 (stratumURL, stratumPort, stratumTLS, stratumProtocol and their fallback counterparts). The role tells apart the primary and the fallback pool, which stratum_using_fallback resolves to a pool label.",
			"role", "url", "port", "tls", "protocol"),
		usingFallback: desc("stratum_using_fallback", "Whether the device is mining on the fallback pool (stratum.usingFallback)."),
		poolMode:      desc("stratum_pool_mode", "Active pool mode, 1 for the current mode (stratum.activePoolMode).", "mode"),

		poolConnected: desc("pool_connected", "Whether the stratum connection is established (stratum.pools.connected)."+poolLabelHelp+roleLabelHelp+" The url, port, tls and protocol labels are the same pool identity stratum_info carries.",
			"pool", "role", "url", "port", "tls", "protocol"),
		poolDifficulty:    desc("pool_difficulty", "Share difficulty set by the pool (stratum.pools.poolDifficulty)."+poolLabelHelp, "pool"),
		networkDifficulty: desc("network_difficulty", "Bitcoin network difficulty reported by the pool (stratum.pools.networkDifficulty)."+poolLabelHelp, "pool"),
		poolAccepted:      desc("pool_shares_accepted_total", "Shares accepted by the pool (stratum.pools.accepted)."+poolLabelHelp, "pool"),
		poolRejected:      desc("pool_shares_rejected_total", "Shares rejected by the pool (stratum.pools.rejected)."+poolLabelHelp, "pool"),
		poolBestDiff:      desc("pool_best_difficulty", "Best share difficulty submitted to the pool this session (stratum.pools.bestDiff)."+poolLabelHelp, "pool"),
		poolPingRTT:       desc("pool_ping_rtt_seconds", "Round trip time to the pool (stratum.pools.pingRtt)."+poolLabelHelp+roleLabelHelp, "pool", "role"),
		poolPingLoss:      desc("pool_ping_loss_ratio", "Fraction of ping packets lost to the pool (stratum.pools.pingLoss)."+poolLabelHelp+roleLabelHelp, "pool", "role"),

		uptime:   desc("uptime_seconds", "Time since the last restart (uptimeSeconds)."),
		wifiRSSI: desc("wifi_rssi_dbm", "WiFi signal strength (wifiRSSI)."),
		freeHeap: desc("free_heap_bytes", "Free heap per memory area (freeHeap, freeHeapInt).", "memory"),
	}
}

// Collector scrapes a NerdQAxe device on every Prometheus collection.
type Collector struct {
	client *nerdqaxe.Client
	logger zerolog.Logger
	ctx    context.Context

	// The device identity read at startup, which names a scrape that failed
	// and has no response to read it from.
	hostname, mac string

	metrics
}

// New returns a Collector scraping the device behind client. The info it
// reports names the device of a scrape that fails before any has succeeded.
func New(client *nerdqaxe.Client, logger zerolog.Logger, info *nerdqaxe.SystemInfo) *Collector {
	return &Collector{client: client, logger: logger, ctx: context.Background(),
		hostname: info.Hostname, mac: info.MacAddr, metrics: newMetrics()}
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
	ch <- c.up
	ch <- c.scrapeDuration
	ch <- c.info

	ch <- c.hashRate

	ch <- c.power
	ch <- c.inputVoltage
	ch <- c.inputCurrent
	ch <- c.coreVoltage
	ch <- c.coreVoltageConfigured
	ch <- c.frequency
	ch <- c.shutdown

	ch <- c.temperature
	ch <- c.asicTemp
	ch <- c.overheatTemp

	ch <- c.fanRPM
	ch <- c.fanRatio

	ch <- c.blocksFound
	ch <- c.sessionBlocksFound
	ch <- c.bestDifficulty
	ch <- c.sessionBestDiff
	ch <- c.duplicateNonces

	ch <- c.stratumInfo
	ch <- c.usingFallback
	ch <- c.poolMode

	ch <- c.poolConnected
	ch <- c.poolDifficulty
	ch <- c.networkDifficulty
	ch <- c.poolAccepted
	ch <- c.poolRejected
	ch <- c.poolBestDiff
	ch <- c.poolPingRTT
	ch <- c.poolPingLoss

	ch <- c.uptime
	ch <- c.wifiRSSI
	ch <- c.freeHeap
}

// Collect implements prometheus.Collector. It queries the device once, so that
// scrapes stay synchronous with Prometheus and no state is shared between them.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	i, err := c.client.SystemInfo(c.ctx)
	duration := time.Since(start).Seconds()

	// The device names itself, so take the identity from the response of this
	// scrape, and fall back to the one read at startup when it did not answer.
	hostname, mac := c.hostname, c.mac
	if err == nil {
		hostname, mac = i.Hostname, i.MacAddr
	}

	ch <- gauge(c.scrapeDuration, duration, hostname, mac)

	if err != nil {
		c.logger.Error().Err(err).Str("target", c.client.Target()).Msg("scrape failed")
		ch <- gauge(c.up, 0, hostname, mac)
		return
	}
	ch <- gauge(c.up, 1, hostname, mac)

	ch <- gauge(c.info, 1,
		i.DeviceModel, i.ASICModel, strconv.Itoa(i.ASICCount),
		i.HostIP, i.Version, i.SSID, hostname, mac)

	ch <- gauge(c.hashRate, i.HashRate*gigahashPerHash, hostname, mac)

	ch <- gauge(c.power, i.Power, hostname, mac)
	ch <- gauge(c.inputVoltage, i.Voltage/millivoltPerVolt, hostname, mac)
	ch <- gauge(c.inputCurrent, i.CurrentA, hostname, mac)
	ch <- gauge(c.coreVoltage, i.CoreVoltageActual/millivoltPerVolt, hostname, mac)
	ch <- gauge(c.coreVoltageConfigured, i.CoreVoltage/millivoltPerVolt, hostname, mac)
	ch <- gauge(c.frequency, i.Frequency*hertzPerMegahertz, hostname, mac)
	ch <- gauge(c.shutdown, boolToFloat(i.Shutdown), hostname, mac)

	ch <- gauge(c.temperature, i.Temp, "asic_max", hostname, mac)
	ch <- gauge(c.temperature, i.VRTemp, "voltage_regulator", hostname, mac)
	ch <- gauge(c.temperature, i.VRTempInt, "voltage_regulator_internal", hostname, mac)
	for n, temp := range i.ASICTemps {
		ch <- gauge(c.asicTemp, temp, strconv.Itoa(n), hostname, mac)
	}
	ch <- gauge(c.overheatTemp, i.OverheatTemp, hostname, mac)

	for _, fan := range i.Fans {
		ch <- gauge(c.fanRPM, fan.RPM, fan.Label, hostname, mac)
		ch <- gauge(c.fanRatio, fan.SpeedPerc/percentPerRatio, fan.Label, hostname, mac)
	}

	ch <- counter(c.blocksFound, i.TotalFoundBlocks, hostname, mac)
	ch <- counter(c.sessionBlocksFound, i.FoundBlocks, hostname, mac)
	ch <- gauge(c.bestDifficulty, i.BestDiff, hostname, mac)
	ch <- gauge(c.sessionBestDiff, i.BestSessionDiff, hostname, mac)
	ch <- counter(c.duplicateNonces, i.DuplicateHWNonces, hostname, mac)

	ch <- gauge(c.stratumInfo, 1, append(append([]string{rolePrimary}, stratumIdentity(i, rolePrimary)...), hostname, mac)...)
	ch <- gauge(c.stratumInfo, 1, append(append([]string{roleFallback}, stratumIdentity(i, roleFallback)...), hostname, mac)...)

	ch <- gauge(c.usingFallback, boolToFloat(i.Stratum.UsingFallback), hostname, mac)
	for mode, label := range poolModes {
		ch <- gauge(c.poolMode, boolToFloat(mode == i.Stratum.PoolMode), label, hostname, mac)
	}

	for n, pool := range i.Stratum.Pools {
		id := strconv.Itoa(n)
		role := poolRole(i.Stratum, n)
		ch <- gauge(c.poolConnected, boolToFloat(pool.Connected),
			append(append([]string{id, role}, stratumIdentity(i, role)...), hostname, mac)...)
		ch <- gauge(c.poolDifficulty, pool.PoolDifficulty, id, hostname, mac)
		ch <- gauge(c.networkDifficulty, pool.NetworkDifficulty, id, hostname, mac)
		ch <- counter(c.poolAccepted, pool.Accepted, id, hostname, mac)
		ch <- counter(c.poolRejected, pool.Rejected, id, hostname, mac)
		ch <- gauge(c.poolBestDiff, pool.BestDiff, id, hostname, mac)
		ch <- gauge(c.poolPingRTT, pool.PingRTT/millisecondPerSecond, id, role, hostname, mac)
		ch <- gauge(c.poolPingLoss, pool.PingLoss, id, role, hostname, mac)
	}

	ch <- gauge(c.uptime, i.UptimeSeconds, hostname, mac)
	ch <- gauge(c.wifiRSSI, i.WifiRSSI, hostname, mac)
	ch <- gauge(c.freeHeap, i.FreeHeap, "spiram", hostname, mac)
	ch <- gauge(c.freeHeap, i.FreeHeapInt, "internal", hostname, mac)
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

// Group collects several devices as one Prometheus collector. Devices share
// the same descriptors, so they cannot be registered as separate collectors;
// they are told apart by the hostname and mac labels they report.
type Group []*Collector

// WithContext returns a copy of the group whose scrapes run under ctx.
func (g Group) WithContext(ctx context.Context) Group {
	clone := make(Group, len(g))
	for n, c := range g {
		clone[n] = c.WithContext(ctx)
	}
	return clone
}

// Describe implements prometheus.Collector. Every device has the same
// descriptors, so one of them describes the group.
func (g Group) Describe(ch chan<- *prometheus.Desc) {
	if len(g) > 0 {
		g[0].Describe(ch)
	}
}

// Collect implements prometheus.Collector. It queries every device at once, so
// that a scrape takes as long as the slowest device rather than all of them.
func (g Group) Collect(ch chan<- prometheus.Metric) {
	var wg sync.WaitGroup
	for _, c := range g {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Collect(ch)
		}()
	}
	wg.Wait()
}
