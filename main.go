package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"github.com/TheoBrigitte/nerdqaxe-exporter/internal/collector"
	"github.com/TheoBrigitte/nerdqaxe-exporter/internal/nerdqaxe"
)

func main() {
	cmd := &cli.Command{
		Name:  "nerdqaxe-exporter",
		Usage: "Prometheus exporter for NerdQaxe",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "version",
				Aliases: []string{"V"},
				Usage:   "print only the version",
			},
			&cli.StringSliceFlag{
				Name:    "target",
				Aliases: []string{"t"},
				Usage:   "base `URL` of a NerdQAxe device, e.g. http://nerdqaxe.local; repeat for several devices",
				Sources: cli.EnvVars("NERDQAXE_TARGET"),
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Usage: "timeout for a device query",
				Value: 5 * time.Second,
			},
			&cli.StringFlag{
				Name:  "web.listen-address",
				Usage: "`address` to listen on for telemetry",
				Value: ":10055",
			},
			&cli.StringFlag{
				Name:  "web.metrics-path",
				Usage: "`path` under which to expose metrics",
				Value: "/metrics",
			},
			&cli.StringFlag{
				Name:  "log.level",
				Usage: "only log messages at or above this `level`: debug, info, warn, error",
				Value: "info",
			},
			&cli.StringFlag{
				Name:  "log.format",
				Usage: "log output `format`: json, console",
				Value: "json",
			},
		},
		Action: run,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Error().Err(err).Msg("exporter failed")
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd *cli.Command) error {
	logger, err := newLogger(cmd.String("log.level"), cmd.String("log.format"))
	if err != nil {
		return err
	}

	// main logs the error this returns through the zerolog global, so point
	// the global at the configured logger to keep the flags applying to it.
	log.Logger = logger

	// --version is not a scrape, print the version and stop here.
	if cmd.Bool("version") {
		_, err := fmt.Print(version.Print("nerdqaxe-exporter"))
		return err
	}

	logger.Info().Str("version", version.Version).Str("revision", version.Revision).Msg("starting nerdqaxe-exporter")

	// target is not a required flag, so that --version works without it.
	targets := cmd.StringSlice("target")
	if len(targets) == 0 {
		return fmt.Errorf("missing required flag --target")
	}

	// Initialize one NerdQaxe client and collector per device.
	deviceCollectors, err := newDeviceCollectors(ctx, targets, cmd.Duration("timeout"), logger)
	if err != nil {
		return err
	}

	// Initialize Prometheus registry and register collectors. The device
	// collector is registered per scrape instead, see below.
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		versioncollector.NewCollector("nerdqaxe_exporter"),
	)

	// Initialize HTTP server handlers
	path := cmd.String("web.metrics-path")
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("invalid metrics path %q: must start with /", path)
	}

	handlerOpts := promhttp.HandlerOpts{ErrorLog: promErrorLog{logger}}
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bind the device collectors to the request, so that the device
		// queries stop as soon as Prometheus gives up on the scrape. The
		// group queries the devices in parallel.
		scrapeRegistry := prometheus.NewPedanticRegistry()
		scrapeRegistry.MustRegister(deviceCollectors.WithContext(r.Context()))

		promhttp.HandlerFor(prometheus.Gatherers{registry, scrapeRegistry}, handlerOpts).ServeHTTP(w, r)
	}))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		var scraping strings.Builder
		for _, target := range targets {
			fmt.Fprintf(&scraping, "<li><code>%s</code></li>\n", html.EscapeString(target)) // nolint:errcheck
		}
		fmt.Fprintf(w, "<html><head><title>NerdQAxe Exporter</title></head><body>\n"+ // nolint:errcheck
			"<h1>NerdQAxe Exporter</h1>\n<p>Scraping</p>\n<ul>\n%s</ul>\n"+
			"<p><a href=%q>Metrics</a></p>\n</body></html>\n", scraping.String(), path)
	})

	// Initialize HTTP server
	address := cmd.String("web.listen-address")
	server := &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Setup interrupt signal handling for gracefully shutdown
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Let an in-flight scrape finish before going away, so that Prometheus
	// gets a complete response rather than a scrape error.
	shutdownDone := make(chan struct{})
	go func() { // nolint:gosec //
		defer close(shutdownDone)
		<-ctx.Done()

		// Restore the default signal handling, so that a second signal
		// kills the process instead of being swallowed while draining.
		stop()

		// A scrape takes at most one device timeout, plus a margin to send
		// the response.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cmd.Duration("timeout")+time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error().Err(err).Msg("HTTP server shutdown failed")
		}
	}()

	// Start HTTP server and wait for shutdown or error
	logger.Info().Str("address", address).Str("path", path).Strs("targets", targets).Msg("listening")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("HTTP server failed: %w", err)
	}

	// ListenAndServe returns as soon as Shutdown starts, so wait Shutdown
	// complete before letting the process exit.
	<-shutdownDone

	return nil
}

// newDeviceCollectors builds a collector per target. It queries every device
// once, to fail fast on a target that cannot be scraped rather than starting
// up and only reporting the problem through nerdqaxe_up on every scrape.
func newDeviceCollectors(ctx context.Context, targets []string, timeout time.Duration, logger zerolog.Logger) (collector.Group, error) {
	// Devices are told apart by the hostname and mac labels they report, so
	// the same device listed twice would emit duplicate metrics.
	seen := make(map[string]bool, len(targets))

	collectors := make(collector.Group, 0, len(targets))
	for _, target := range targets {
		client, err := nerdqaxe.New(target, timeout)
		if err != nil {
			return nil, err
		}
		if seen[client.Target()] {
			return nil, fmt.Errorf("duplicate target %q", client.Target())
		}
		seen[client.Target()] = true

		info, err := checkDevice(ctx, client, timeout)
		if err != nil {
			return nil, fmt.Errorf("failed to query device %s: %w", client.Target(), err)
		}
		logger.Info().
			Str("target", client.Target()).
			Str("device_model", info.DeviceModel).
			Str("asic_model", info.ASICModel).
			Str("hostname", info.Hostname).
			Str("version", info.Version).
			Msg("device reachable")

		collectors = append(collectors, collector.New(client, logger, info))
	}

	return collectors, nil
}

// checkDevice queries a device once, under a timeout of its own so that the
// startup check of one device does not eat into the budget of the next.
func checkDevice(ctx context.Context, client *nerdqaxe.Client, timeout time.Duration) (*nerdqaxe.SystemInfo, error) {
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return client.SystemInfo(checkCtx)
}

// newLogger builds the root logger from the --log.level and --log.format flags.
func newLogger(level, format string) (zerolog.Logger, error) {
	parsedLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		return zerolog.Nop(), fmt.Errorf("invalid --log.level %q: %w", level, err)
	}

	var writer io.Writer
	switch format {
	case "json":
		writer = os.Stderr
	case "console":
		writer = zerolog.ConsoleWriter{Out: os.Stderr}
	default:
		return zerolog.Nop(), fmt.Errorf("invalid --log.format %q: must be json or console", format)
	}

	return zerolog.New(writer).Level(parsedLevel).With().Timestamp().Logger(), nil
}

// promErrorLog adapts a zerolog logger to the promhttp.Logger interface.
type promErrorLog struct {
	logger zerolog.Logger
}

func (l promErrorLog) Println(v ...any) {
	l.logger.Error().Msg(fmt.Sprint(v...))
}
