package config

import (
	"flare-indexer/config"
)

type Config struct {
	DB       config.DBConfig     `toml:"db"`
	Logger   config.LoggerConfig `toml:"logger"`
	Chain    config.ChainConfig  `toml:"chain"`
	Services ServicesConfig      `toml:"services"`
	Epochs   config.EpochConfig  `toml:"epochs"`
}

type ServicesConfig struct {
	Address string `toml:"address"`
}

func newConfig() *Config {
	return &Config{
		Services: ServicesConfig{
			Address: "localhost:8000",
		},
	}
}

func (c Config) LoggerConfig() config.LoggerConfig {
	return c.Logger
}

func (c Config) ChainConfig() config.ChainConfig {
	return c.Chain
}

func BuildConfig() (*Config, error) {
	cfgFileName := config.ConfigFileName()
	cfg := newConfig()
	err := config.ParseConfigFile(cfg, cfgFileName, false)
	if err != nil {
		return nil, err
	}
	err = config.ReadEnv(cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
