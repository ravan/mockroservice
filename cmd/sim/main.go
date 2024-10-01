package main

import (
	"github.com/ravan/microservice-sim/internal/config"
	"github.com/ravan/microservice-sim/internal/server"
	"github.com/urfave/cli/v2"
	"log/slog"

	"os"
)

func main() {
	app := &cli.App{
		Name:  "sim",
		Usage: "Microservice simulator for building demo application topologies",
		Action: func(ctx *cli.Context) error {
			configFile := os.Getenv("CONFIG_FILE")
			if configFile == "" {
				configFile = ctx.String("config")
			}
			conf, err := config.GetConfig(configFile)
			if err != nil {
				return err
			}
			return server.Run(conf)
		},
		Flags: []cli.Flag{&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "configuration file for service simulation",
		}},
	}

	if err := app.Run(os.Args); err != nil {
		slog.Error("Error running sim", slog.Any("error", err))
	}
}
