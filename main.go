package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/prometheus/common/version"
	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name: "nerdqaxe-exporter",
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
		},

		Usage: "Prometheus exporter for NerdQaxe",
		Action: func(context.Context, *cli.Command) error {
			fmt.Println("Hello there")
			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
