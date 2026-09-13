package main

import (
	"context"
	"errors"
	"fmt"
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
			&cli.StringFlag{
				Name:    "target",
				Aliases: []string{"t"},
				Usage:   "base `URL` of the NerdQAxe device, e.g. http://192.0.2.10",
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

	// target is not a required flag, so that --version works without it.
	if cmd.String("target") == "" {
		return fmt.Errorf("missing required flag --target")
	}

	// Initialize NerdQaxe client
	client, err := nerdqaxe.New(cmd.String("target"), cmd.Duration("timeout"))
	if err != nil {
		return err
	}

	// Fail fast on a target that cannot be scraped, rather than starting up
	// and only reporting the problem through nerdqaxe_up on every scrape.
	checkCtx, cancel := context.WithTimeout(ctx, cmd.Duration("timeout"))
	defer cancel()

	info, err := client.SystemInfo(checkCtx)
	if err != nil {
		return fmt.Errorf("failed to query device: %w", err)
	}
	logger.Info().
		Str("device_model", info.DeviceModel).
		Str("asic_model", info.ASICModel).
		Str("hostname", info.Hostname).
		Str("version", info.Version).
		Msg("device reachable")

	// Initialize Prometheus registry and register collectors. The device
	// collector is registered per scrape instead, see below.
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		versioncollector.NewCollector("nerdqaxe_exporter"),
	)
	deviceCollector := collector.New(client, logger)

	// Initialize HTTP server handlers
	path := cmd.String("web.metrics-path")
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("invalid metrics path %q: must start with /", path)
	}

	handlerOpts := promhttp.HandlerOpts{ErrorLog: promErrorLog{logger}}
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bind the device collector to the request, so that the device query
		// stops as soon as Prometheus gives up on the scrape.
		scrapeRegistry := prometheus.NewPedanticRegistry()
		scrapeRegistry.MustRegister(deviceCollector.WithContext(r.Context()))

		promhttp.HandlerFor(prometheus.Gatherers{registry, scrapeRegistry}, handlerOpts).ServeHTTP(w, r)
	}))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "<html><head><title>NerdQAxe Exporter</title></head><body>\n"+
			"<h1>NerdQAxe Exporter</h1>\n<p>Scraping <code>%s</code></p>\n"+
			"<p><a href=%q>Metrics</a></p>\n</body></html>\n", client.Target(), path)
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
	go func() {
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
	logger.Info().Str("address", address).Str("path", path).Str("target", client.Target()).Msg("listening")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("HTTP server failed: %w", err)
	}

	// ListenAndServe returns as soon as Shutdown starts, so wait Shutdown
	// complete before letting the process exit.
	<-shutdownDone

	return nil
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
