package cmd

import (
	"github.com/ravan/microservice-sim/internal/config"
	"github.com/ravan/microservice-sim/internal/server"
	"github.com/urfave/cli/v2"
	"os"
)

func NewServeCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "start the service",
		Action: func(ctx *cli.Context) error {
			conf, err := getConfig(ctx)
			if err != nil {
				return err
			}
			return server.Run(conf)
		},
	}
}

func getConfig(ctx *cli.Context) (*config.Configuration, error) {
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = ctx.String("config")
	}
	conf, err := config.GetConfig(configFile)
	if err != nil {
		return nil, err
	}
	return conf, nil
}
