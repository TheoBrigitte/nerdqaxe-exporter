package collector

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rs/zerolog"

	"github.com/TheoBrigitte/nerdqaxe-exporter/internal/nerdqaxe"
)

func newCollector(t *testing.T, handler http.HandlerFunc) *Collector {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := nerdqaxe.New(server.URL, time.Second)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	// The device names itself on every scrape, so the startup info names it
	// differently from the fixture, to tell the two apart.
	info := &nerdqaxe.SystemInfo{Hostname: "at-startup", MacAddr: "00:00:00:00:00:00"}

	return New(client, zerolog.New(io.Discard), info)
}

func TestCollect(t *testing.T) {
	fixture, err := os.ReadFile("testdata/system_info.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	c := newCollector(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/system/info" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, err := w.Write(fixture)
		if err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	})

	expected := `
# HELP nerdqaxe_asic_frequency_hertz Configured ASIC clock frequency (frequency).
# TYPE nerdqaxe_asic_frequency_hertz gauge
nerdqaxe_asic_frequency_hertz{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF"} 6e+08
# HELP nerdqaxe_core_voltage_configured_volts Configured ASIC core voltage (coreVoltage).
# TYPE nerdqaxe_core_voltage_configured_volts gauge
nerdqaxe_core_voltage_configured_volts{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF"} 1.15
# HELP nerdqaxe_core_voltage_volts Measured ASIC core voltage (coreVoltageActual).
# TYPE nerdqaxe_core_voltage_volts gauge
nerdqaxe_core_voltage_volts{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF"} 1.148
# HELP nerdqaxe_fan_speed_ratio Fan speed as a fraction of maximum (fans.speedPerc).
# TYPE nerdqaxe_fan_speed_ratio gauge
nerdqaxe_fan_speed_ratio{fan="M1",hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF"} 0.9
nerdqaxe_fan_speed_ratio{fan="M2",hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF"} 0.9
# HELP nerdqaxe_fan_speed_rpm Fan speed (fans.rpm).
# TYPE nerdqaxe_fan_speed_rpm gauge
nerdqaxe_fan_speed_rpm{fan="M1",hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF"} 732
nerdqaxe_fan_speed_rpm{fan="M2",hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF"} 2347
# HELP nerdqaxe_free_heap_bytes Free heap per memory area (freeHeap, freeHeapInt).
# TYPE nerdqaxe_free_heap_bytes gauge
nerdqaxe_free_heap_bytes{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF",memory="internal"} 95068
nerdqaxe_free_heap_bytes{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF",memory="spiram"} 6.972844e+06
# HELP nerdqaxe_hashrate_hashes_per_second Current hashrate (hashRate).
# TYPE nerdqaxe_hashrate_hashes_per_second gauge
nerdqaxe_hashrate_hashes_per_second{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF"} 4.881947e+12
# HELP nerdqaxe_info Device identity, always 1 (deviceModel, ASICModel, asicCount, hostip, version, ssid); hostname and macAddr label every metric.
# TYPE nerdqaxe_info gauge
nerdqaxe_info{asic_count="4",asic_model="BM1370",device_model="NerdQAxe++",hostname="nerdqaxe",ip="192.0.2.10",mac="AA:BB:CC:DD:EE:FF",ssid="example-wifi",version="V1.0.37.2-LTS"} 1
# HELP nerdqaxe_input_current_amperes Input current (currentA).
# TYPE nerdqaxe_input_current_amperes gauge
nerdqaxe_input_current_amperes{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF"} 6.148438
# HELP nerdqaxe_input_voltage_volts Input voltage (voltage).
# TYPE nerdqaxe_input_voltage_volts gauge
nerdqaxe_input_voltage_volts{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF"} 12.04688
# HELP nerdqaxe_pool_connected Whether the stratum connection is established (stratum.pools.connected). In failover mode the device reports only the selected pool, so pool 0 is whichever pool is active. The role tells which configured pool this is. The url, port, tls and protocol labels are the same pool identity stratum_info carries.
# TYPE nerdqaxe_pool_connected gauge
nerdqaxe_pool_connected{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF",pool="0",port="4333",protocol="stratum_v1",role="primary",tls="true",url="pool.example.com"} 1
# HELP nerdqaxe_pool_ping_rtt_seconds Round trip time to the pool (stratum.pools.pingRtt). In failover mode the device reports only the selected pool, so pool 0 is whichever pool is active. The role tells which configured pool this is.
# TYPE nerdqaxe_pool_ping_rtt_seconds gauge
nerdqaxe_pool_ping_rtt_seconds{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF",pool="0",role="primary"} 0.016
# HELP nerdqaxe_pool_shares_accepted_total Shares accepted by the pool (stratum.pools.accepted). In failover mode the device reports only the selected pool, so pool 0 is whichever pool is active.
# TYPE nerdqaxe_pool_shares_accepted_total counter
nerdqaxe_pool_shares_accepted_total{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF",pool="0"} 874110
# HELP nerdqaxe_pool_shares_rejected_total Shares rejected by the pool (stratum.pools.rejected). In failover mode the device reports only the selected pool, so pool 0 is whichever pool is active.
# TYPE nerdqaxe_pool_shares_rejected_total counter
nerdqaxe_pool_shares_rejected_total{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF",pool="0"} 231
# HELP nerdqaxe_power_watts Power drawn by the device (power).
# TYPE nerdqaxe_power_watts gauge
nerdqaxe_power_watts{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF"} 74.25
# HELP nerdqaxe_stratum_pool_mode Active pool mode, 1 for the current mode (stratum.activePoolMode).
# TYPE nerdqaxe_stratum_pool_mode gauge
nerdqaxe_stratum_pool_mode{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF",mode="dual"} 0
nerdqaxe_stratum_pool_mode{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF",mode="failover"} 1
# HELP nerdqaxe_temperature_celsius Temperature per sensor (temp, vrTemp, vrTempInt).
# TYPE nerdqaxe_temperature_celsius gauge
nerdqaxe_temperature_celsius{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF",sensor="asic_max"} 55.0625
nerdqaxe_temperature_celsius{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF",sensor="voltage_regulator"} 54.4375
nerdqaxe_temperature_celsius{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF",sensor="voltage_regulator_internal"} 58.125
# HELP nerdqaxe_up Whether the last scrape of the device succeeded.
# TYPE nerdqaxe_up gauge
nerdqaxe_up{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF"} 1
# HELP nerdqaxe_uptime_seconds Time since the last restart (uptimeSeconds).
# TYPE nerdqaxe_uptime_seconds gauge
nerdqaxe_uptime_seconds{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF"} 3.554014e+06
# HELP nerdqaxe_wifi_rssi_dbm WiFi signal strength (wifiRSSI).
# TYPE nerdqaxe_wifi_rssi_dbm gauge
nerdqaxe_wifi_rssi_dbm{hostname="nerdqaxe",mac="AA:BB:CC:DD:EE:FF"} -44
`

	err = testutil.CollectAndCompare(c, strings.NewReader(expected),
		"nerdqaxe_asic_frequency_hertz",
		"nerdqaxe_core_voltage_configured_volts",
		"nerdqaxe_core_voltage_volts",
		"nerdqaxe_fan_speed_ratio",
		"nerdqaxe_fan_speed_rpm",
		"nerdqaxe_free_heap_bytes",
		"nerdqaxe_hashrate_hashes_per_second",
		"nerdqaxe_info",
		"nerdqaxe_input_current_amperes",
		"nerdqaxe_input_voltage_volts",
		"nerdqaxe_pool_connected",
		"nerdqaxe_pool_ping_rtt_seconds",
		"nerdqaxe_pool_shares_accepted_total",
		"nerdqaxe_pool_shares_rejected_total",
		"nerdqaxe_power_watts",
		"nerdqaxe_stratum_pool_mode",
		"nerdqaxe_temperature_celsius",
		"nerdqaxe_up",
		"nerdqaxe_uptime_seconds",
		"nerdqaxe_wifi_rssi_dbm")
	if err != nil {
		t.Error(err)
	}
}

func TestCollectDeviceDown(t *testing.T) {
	c := newCollector(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	expected := `
# HELP nerdqaxe_up Whether the last scrape of the device succeeded.
# TYPE nerdqaxe_up gauge
nerdqaxe_up{hostname="at-startup",mac="00:00:00:00:00:00"} 0
`

	if err := testutil.CollectAndCompare(c, strings.NewReader(expected), "nerdqaxe_up"); err != nil {
		t.Error(err)
	}

	// The scrape duration is about the scrape itself, so it is reported even
	// when the device did not answer.
	if got := testutil.CollectAndCount(c, "nerdqaxe_scrape_duration_seconds"); got != 1 {
		t.Errorf("scrape duration metrics = %d, want 1", got)
	}
}

func TestPoolRole(t *testing.T) {
	// The device reports both pools in slot order in dual mode, and only the
	// selected pool in failover mode, so the index alone does not name a pool.
	for _, tc := range []struct {
		name     string
		stratum  nerdqaxe.Stratum
		index    int
		expected string
	}{
		{"failover on primary", nerdqaxe.Stratum{PoolMode: 0, UsingFallback: false}, 0, "primary"},
		{"failover on fallback", nerdqaxe.Stratum{PoolMode: 0, UsingFallback: true}, 0, "fallback"},
		{"dual primary slot", nerdqaxe.Stratum{PoolMode: 1}, 0, "primary"},
		{"dual fallback slot", nerdqaxe.Stratum{PoolMode: 1}, 1, "fallback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := poolRole(tc.stratum, tc.index); got != tc.expected {
				t.Errorf("poolRole(%+v, %d) = %q, want %q", tc.stratum, tc.index, got, tc.expected)
			}
		})
	}
}
