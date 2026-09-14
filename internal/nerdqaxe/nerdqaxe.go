// Package nerdqaxe provides a client for the NerdQAxe firmware HTTP API.
package nerdqaxe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const systemInfoPath = "/api/system/info"

// SystemInfo is the subset of /api/system/info used by the exporter.
type SystemInfo struct {
	// Identity
	DeviceModel string `json:"deviceModel"`
	ASICModel   string `json:"ASICModel"`
	Hostname    string `json:"hostname"`
	HostIP      string `json:"hostip"`
	MacAddr     string `json:"macAddr"`
	Version     string `json:"version"`
	SSID        string `json:"ssid"`
	ASICCount   int    `json:"asicCount"`

	// HashRate is the current hashrate, in GH/s. The device also reports 1m
	// to 1d averages; they are left out so that Prometheus does the averaging.
	HashRate float64 `json:"hashRate"`

	// Power
	Power             float64 `json:"power"`             // W
	Voltage           float64 `json:"voltage"`           // mV
	CurrentA          float64 `json:"currentA"`          // A
	CoreVoltage       float64 `json:"coreVoltage"`       // mV, configured
	CoreVoltageActual float64 `json:"coreVoltageActual"` // mV, measured
	Frequency         float64 `json:"frequency"`         // MHz
	Shutdown          bool    `json:"shutdown"`

	// Temperatures, in degrees Celsius.
	Temp         float64   `json:"temp"`
	VRTemp       float64   `json:"vrTemp"`
	VRTempInt    float64   `json:"vrTempInt"`
	ASICTemps    []float64 `json:"asicTemps"`
	OverheatTemp float64   `json:"overheat_temp"`

	Fans []Fan `json:"fans"`

	// Mining. Shares are also reported at the top level, but those are totals
	// over Stratum.Pools, so they are left out.
	FoundBlocks       float64 `json:"foundBlocks"`
	TotalFoundBlocks  float64 `json:"totalFoundBlocks"`
	BestDiff          float64 `json:"bestDiff"`
	BestSessionDiff   float64 `json:"bestSessionDiff"`
	DuplicateHWNonces float64 `json:"duplicateHWNonces"`
	Stratum           Stratum `json:"stratum"`

	// Stratum configuration. The device reports the primary and the fallback
	// pool at the top level, whereas Stratum.Pools holds their live state.
	StratumURL              string `json:"stratumURL"`
	StratumPort             int    `json:"stratumPort"`
	StratumTLS              bool   `json:"stratumTLS"`
	StratumProtocol         int    `json:"stratumProtocol"`
	FallbackStratumURL      string `json:"fallbackStratumURL"`
	FallbackStratumPort     int    `json:"fallbackStratumPort"`
	FallbackStratumTLS      bool   `json:"fallbackStratumTLS"`
	FallbackStratumProtocol int    `json:"fallbackStratumProtocol"`

	// System
	UptimeSeconds float64 `json:"uptimeSeconds"`
	WifiRSSI      float64 `json:"wifiRSSI"`
	FreeHeap      float64 `json:"freeHeap"`    // SPI RAM, bytes
	FreeHeapInt   float64 `json:"freeHeapInt"` // internal RAM, bytes
}

// Fan holds the state of a single fan channel.
type Fan struct {
	Label     string  `json:"label"`
	RPM       float64 `json:"rpm"`
	SpeedPerc float64 `json:"speedPerc"`
}

// Stratum holds the state of the pool connections.
type Stratum struct {
	PoolMode      int     `json:"activePoolMode"`
	UsingFallback bool    `json:"usingFallback"`
	TotalBestDiff float64 `json:"totalBestDiff"`
	Pools         []Pool  `json:"pools"`
}

// Pool holds the state of a single stratum pool.
type Pool struct {
	Connected         bool    `json:"connected"`
	PoolDifficulty    float64 `json:"poolDifficulty"`
	NetworkDifficulty float64 `json:"networkDifficulty"`
	Accepted          float64 `json:"accepted"`
	Rejected          float64 `json:"rejected"`
	PingRTT           float64 `json:"pingRtt"` // ms
	PingLoss          float64 `json:"pingLoss"`
	BestDiff          float64 `json:"bestDiff"`
}

// Client queries the NerdQAxe HTTP API.
type Client struct {
	target string
	http   *http.Client
}

// New returns a Client querying the device at target, e.g. http://192.168.1.42.
func New(target string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("invalid target %q: %w", target, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid target %q: scheme must be http or https", target)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid target %q: missing host", target)
	}
	// A zero timeout means no timeout at all in http.Client, which would let
	// a scrape hang forever against an unresponsive device.
	if timeout <= 0 {
		return nil, fmt.Errorf("invalid timeout %s: must be positive", timeout)
	}

	return &Client{
		target: u.Scheme + "://" + u.Host,
		http:   &http.Client{Timeout: timeout},
	}, nil
}

// Target returns the device base URL.
func (c *Client) Target() string {
	return c.target
}

// SystemInfo fetches the current device state.
func (c *Client) SystemInfo(ctx context.Context) (*SystemInfo, error) {
	u, err := url.JoinPath(c.target, systemInfoPath)
	if err != nil {
		return nil, fmt.Errorf("join path: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", req.URL, err)
	}
	defer res.Body.Close() // nolint:errcheck

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned unexpected status %s", req.URL, res.Status)
	}

	var info SystemInfo
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &info, nil
}
