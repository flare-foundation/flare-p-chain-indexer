package config

import (
	"flare-indexer/config"

	"github.com/ava-labs/libevm/common"
)

type Config struct {
	DB                config.DBConfig          `toml:"db"`
	Logger            config.LoggerConfig      `toml:"logger"`
	Chain             config.ChainConfig       `toml:"chain"`
	Services          ServicesConfig           `toml:"services"`
	ContractAddresses config.ContractAddresses `toml:"contract_addresses"`
}

type ServicesConfig struct {
	Address        string         `toml:"address"`
	VotingContract common.Address `toml:"votingContract"`
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

func BuildConfig(cfgFileName string) (*Config, error) {
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
