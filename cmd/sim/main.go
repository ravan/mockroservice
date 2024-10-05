package main

import (
	"github.com/ravan/microservice-sim/internal/cmd"
	"github.com/urfave/cli/v2"
	"log/slog"

	"os"
)

func main() {
	app := &cli.App{
		Name:  "sim",
		Usage: "Microservice simulator for building demo application topologies",
		Commands: []*cli.Command{
			cmd.NewServeCommand(),
			cmd.NewGenerateCommand(),
		},
		DefaultCommand: "serve",
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
