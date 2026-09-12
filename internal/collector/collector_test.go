package collector

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

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

	return New(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
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
		w.Write(fixture)
	})

	expected := `
# HELP nerdqaxe_asic_frequency_hertz ASIC clock frequency.
# TYPE nerdqaxe_asic_frequency_hertz gauge
nerdqaxe_asic_frequency_hertz 6e+08
# HELP nerdqaxe_core_voltage_volts ASIC core voltage, configured and measured.
# TYPE nerdqaxe_core_voltage_volts gauge
nerdqaxe_core_voltage_volts{kind="configured"} 1.15
nerdqaxe_core_voltage_volts{kind="measured"} 1.148
# HELP nerdqaxe_fan_speed_ratio Fan speed as a fraction of maximum.
# TYPE nerdqaxe_fan_speed_ratio gauge
nerdqaxe_fan_speed_ratio{fan="M1"} 0.9
nerdqaxe_fan_speed_ratio{fan="M2"} 0.9
# HELP nerdqaxe_fan_speed_rpm Fan speed.
# TYPE nerdqaxe_fan_speed_rpm gauge
nerdqaxe_fan_speed_rpm{fan="M1"} 732
nerdqaxe_fan_speed_rpm{fan="M2"} 2347
# HELP nerdqaxe_free_heap_bytes Free heap per memory region.
# TYPE nerdqaxe_free_heap_bytes gauge
nerdqaxe_free_heap_bytes{region="internal"} 95068
nerdqaxe_free_heap_bytes{region="spiram"} 6.972844e+06
# HELP nerdqaxe_hashrate_hashes_per_second Hashrate reported by the device.
# TYPE nerdqaxe_hashrate_hashes_per_second gauge
nerdqaxe_hashrate_hashes_per_second{window="10m"} 4.878985e+12
nerdqaxe_hashrate_hashes_per_second{window="1d"} 4.878346e+12
nerdqaxe_hashrate_hashes_per_second{window="1h"} 4.875798e+12
nerdqaxe_hashrate_hashes_per_second{window="1m"} 4.874594e+12
nerdqaxe_hashrate_hashes_per_second{window="current"} 4.881947e+12
# HELP nerdqaxe_info Device identity, always 1.
# TYPE nerdqaxe_info gauge
nerdqaxe_info{asic_count="4",asic_model="BM1370",device_model="NerdQAxe++",hostname="nerdqaxe",ip="192.0.2.10",mac="AA:BB:CC:DD:EE:FF",ssid="example-wifi",version="V1.0.37.2-LTS"} 1
# HELP nerdqaxe_input_current_amperes Input current.
# TYPE nerdqaxe_input_current_amperes gauge
nerdqaxe_input_current_amperes 6.148438
# HELP nerdqaxe_input_voltage_volts Input voltage.
# TYPE nerdqaxe_input_voltage_volts gauge
nerdqaxe_input_voltage_volts 12.04688
# HELP nerdqaxe_pool_connected Whether the stratum connection is established.
# TYPE nerdqaxe_pool_connected gauge
nerdqaxe_pool_connected{pool="0"} 1
# HELP nerdqaxe_pool_ping_rtt_seconds Round trip time to the pool.
# TYPE nerdqaxe_pool_ping_rtt_seconds gauge
nerdqaxe_pool_ping_rtt_seconds{pool="0"} 0.016
# HELP nerdqaxe_pool_shares_accepted_total Shares accepted by the pool.
# TYPE nerdqaxe_pool_shares_accepted_total counter
nerdqaxe_pool_shares_accepted_total{pool="0"} 874110
# HELP nerdqaxe_power_watts Power drawn by the device.
# TYPE nerdqaxe_power_watts gauge
nerdqaxe_power_watts 74.25
# HELP nerdqaxe_shares_accepted_total Shares accepted by the pools.
# TYPE nerdqaxe_shares_accepted_total counter
nerdqaxe_shares_accepted_total 874110
# HELP nerdqaxe_shares_rejected_total Shares rejected by the pools.
# TYPE nerdqaxe_shares_rejected_total counter
nerdqaxe_shares_rejected_total 231
# HELP nerdqaxe_temperature_celsius Temperature per sensor.
# TYPE nerdqaxe_temperature_celsius gauge
nerdqaxe_temperature_celsius{sensor="asic_max"} 55.0625
nerdqaxe_temperature_celsius{sensor="voltage_regulator"} 54.4375
nerdqaxe_temperature_celsius{sensor="voltage_regulator_internal"} 58.125
# HELP nerdqaxe_up Whether the last scrape of the device succeeded.
# TYPE nerdqaxe_up gauge
nerdqaxe_up 1
# HELP nerdqaxe_uptime_seconds Time since the last restart.
# TYPE nerdqaxe_uptime_seconds gauge
nerdqaxe_uptime_seconds 3.554014e+06
# HELP nerdqaxe_wifi_rssi_dbm WiFi signal strength.
# TYPE nerdqaxe_wifi_rssi_dbm gauge
nerdqaxe_wifi_rssi_dbm -44
`

	err = testutil.CollectAndCompare(c, strings.NewReader(expected),
		"nerdqaxe_asic_frequency_hertz",
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
		"nerdqaxe_power_watts",
		"nerdqaxe_shares_accepted_total",
		"nerdqaxe_shares_rejected_total",
		"nerdqaxe_temperature_celsius",
		"nerdqaxe_up",
		"nerdqaxe_uptime_seconds",
		"nerdqaxe_wifi_rssi_dbm",
	)
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
nerdqaxe_up 0
`

	if err := testutil.CollectAndCompare(c, strings.NewReader(expected)); err != nil {
		t.Error(err)
	}
}
