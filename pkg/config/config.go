package config

import (
	"sync"

	"dario.cat/mergo"
	"github.com/haveachin/infrared/pkg/config/provider"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/log"
)

type Base struct {
	Providers struct {
		File *provider.FileConfig `mapstructure:"file"`
	} `mapstructure:"providers"`
}

type config struct {
	config Base

	dataChan chan provider.Data
	onChange OnChange

	mu           sync.RWMutex
	providers    map[provider.Type]provider.Provider
	providerData map[provider.Type]map[string]any
}

type OnChange func(cfg map[string]any)

type Config interface {
	Providers() map[provider.Type]provider.Provider
	Read() (map[string]any, error)
	Reload() (map[string]any, error)
}

func NewFromPath(path string, onChange OnChange) (Config, error) {
	var configMap map[string]any
	if err := provider.ReadConfigFile(path, &configMap); err != nil {
		return nil, err
	}

	return NewFromMap(configMap, onChange)
}

func NewFromMap(configMap map[string]any, onChange OnChange) (Config, error) {
	var base Base
	if err := Unmarshal(configMap, &base); err != nil {
		return nil, err
	}

	if onChange == nil {
		onChange = func(map[string]any) {}
	}

	providers := make(map[provider.Type]provider.Provider)
	if base.Providers.File != nil {
		prov, err := provider.NewFile(*base.Providers.File)
		if err != nil {
			return nil, err
		}
		providers[provider.FileType] = prov
	}

	cfg := config{
		config:    base,
		dataChan:  make(chan provider.Data),
		onChange:  onChange,
		providers: providers,
		providerData: map[provider.Type]map[string]any{
			provider.ConfigType: configMap,
		},
	}

	for _, prov := range cfg.providers {
		data, err := prov.Provide(cfg.dataChan)
		if err != nil {
			log.Fatal().
				Err(err).
				Msg("failed to provide config data")
			continue
		}

		if data.IsNil() {
			continue
		}

		cfg.providerData[data.Type] = data.Config
	}

	go cfg.listenToProviders()
	return &cfg, nil
}

func (c *config) listenToProviders() {
	for data := range c.dataChan {
		c.mu.Lock()
		c.providerData[data.Type] = data.Config
		c.mu.Unlock()

		log.Info().
			Str("provider", data.Type.String()).
			Msg("config changed")

		if _, err := c.Reload(); err != nil {
			log.Error().
				Err(err).
				Msg("failed to reload config")
			continue
		}
	}
}

func (c *config) Reload() (map[string]any, error) {
	cfg, err := c.Read()
	if err != nil {
		return nil, err
	}
	c.onChange(cfg)
	return cfg, nil
}

func (c *config) Providers() map[provider.Type]provider.Provider {
	c.mu.Lock()
	defer c.mu.Unlock()

	providersCopy := map[provider.Type]provider.Provider{}
	for k, v := range c.providers {
		providersCopy[k] = v
	}
	return providersCopy
}

func (c *config) Read() (map[string]any, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cfgData := map[string]any{}
	for _, provData := range c.providerData {
		if err := mergo.Merge(&cfgData, provData); err != nil {
			return nil, err
		}
	}
	return cfgData, nil
}

func Unmarshal(cfg any, v any) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result: v,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
		),
	})
	if err != nil {
		return err
	}

	return decoder.Decode(cfg)
}
