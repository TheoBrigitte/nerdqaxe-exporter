package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
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
				Action: func(context.Context, *cli.Command, bool) error {
					_, err := fmt.Print(version.Print("nerdqaxe-exporter"))
					return err
				},
			},
			&cli.StringFlag{
				Name:     "target",
				Aliases:  []string{"t"},
				Usage:    "base `URL` of the NerdQAxe device, e.g. http://192.0.2.10",
				Sources:  cli.EnvVars("NERDQAXE_TARGET"),
				Required: true,
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
				Name:  "web.telemetry-path",
				Usage: "`path` under which to expose metrics",
				Value: "/metrics",
			},
		},
		Action: run,
	}

	// Setup interrupt signal handling for gracefully shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cmd.Run(ctx, os.Args); err != nil {
		slog.Error("exporter failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd *cli.Command) error {
	logger := slog.Default()

	// Initialize NerdQaxe client
	client, err := nerdqaxe.New(cmd.String("target"), cmd.Duration("timeout"))
	if err != nil {
		return err
	}

	// Initialize Prometheus registry and register collectors
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		versioncollector.NewCollector("nerdqaxe_exporter"),
		collector.New(client, logger),
	)

	// Initialize HTTP server handlers
	path := cmd.String("web.telemetry-path")
	mux := http.NewServeMux()
	mux.Handle(path, promhttp.HandlerFor(registry, promhttp.HandlerOpts{ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError)}))
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

	// Let an in-flight scrape finish before going away, so that Prometheus
	// gets a complete response rather than a scrape error.
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()

		// A scrape takes at most one device timeout, plus a margin to send
		// the response.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cmd.Duration("timeout")+time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTP server shutdown failed", "err", err)
		}
	}()

	// Start HTTP server and wait for shutdown or error
	logger.Info("listening", "address", address, "path", path, "target", client.Target())
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("HTTP server failed: %w", err)
	}

	// ListenAndServe returns as soon as Shutdown starts, so wait Shutdown
	// complete before letting the process exit.
	<-shutdownDone

	return nil
}
