package main

import (
	"fmt"

	"github.com/haveachin/infrared/pkg/config"
	ir "github.com/haveachin/infrared/pkg/infrared"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
)

var (
	configPath = "config.yml"
)

func initFlags() {
	pflag.StringVarP(&configPath, "config", "c", configPath, "path to config file")
	pflag.Parse()
}

func init() {
	initFlags()
}

func main() {
	if err := run(); err != nil {
		log.Fatal().
			Err(err)
	}
}

func run() error {
	cfg, err := config.NewFromPath(configPath, nil)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	cfgMap, err := cfg.Read()
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	srv := ir.New(
		ir.AddConfigFromMap(cfgMap),
	)

	return srv.ListenAndServe()
}
