package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
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
				Value: ":9800",
			},
			&cli.StringFlag{
				Name:  "web.telemetry-path",
				Usage: "`path` under which to expose metrics",
				Value: "/metrics",
			},
		},
		Action: run,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("exporter failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd *cli.Command) error {
	logger := slog.Default()

	client, err := nerdqaxe.New(cmd.String("target"), cmd.Duration("timeout"))
	if err != nil {
		return err
	}

	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		versioncollector.NewCollector("nerdqaxe_exporter"),
		collector.New(client, logger),
	)

	path := cmd.String("web.telemetry-path")
	mux := http.NewServeMux()
	mux.Handle(path, promhttp.HandlerFor(registry, promhttp.HandlerOpts{ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError)}))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "<html><head><title>NerdQAxe Exporter</title></head><body>\n"+
			"<h1>NerdQAxe Exporter</h1>\n<p>Scraping <code>%s</code></p>\n"+
			"<p><a href=%q>Metrics</a></p>\n</body></html>\n", client.Target(), path)
	})

	address := cmd.String("web.listen-address")
	server := &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		server.Close()
	}()

	logger.Info("listening", "address", address, "path", path, "target", client.Target())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}
