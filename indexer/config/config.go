package config

import (
	"flare-indexer/config"
)

type Config struct {
	DB            config.DBConfig     `toml:"db"`
	Logger        config.LoggerConfig `toml:"logger"`
	Chain         config.ChainConfig  `toml:"chain"`
	Metrics       MetricsConfig       `toml:"metrics"`
	XChainIndexer IndexerConfig       `toml:"x_chain_indexer"`
	PChainIndexer IndexerConfig       `toml:"p_chain_indexer"`
	UptimeCronjob UptimeCronjobConfig `toml:"uptime_cronjob"`
}

type MetricsConfig struct {
	PrometheusAddress string `toml:"prometheus_address"`
}

type IndexerConfig struct {
	Enabled          bool   `toml:"enabled"`
	TimeoutMillis    int    `toml:"timeout_millis"`
	BatchSize        int    `toml:"batch_size"`
	StartIndex       uint64 `toml:"start_index"`
	OutputsCacheSize int    `toml:"outputs_cache_size"`
}

type CronjobConfig struct {
	Enabled        bool `toml:"enabled"`
	TimeoutSeconds int  `toml:"timeout_seconds"`
}

type UptimeCronjobConfig struct {
	CronjobConfig
	AggregateStartTimestamp  int64 `toml:"aggregate_start_timestamp"`
	AggregateIntervalSeconds int64 `toml:"aggregate_interval_seconds"`
}

func newConfig() *Config {
	return &Config{
		XChainIndexer: IndexerConfig{
			Enabled:       true,
			TimeoutMillis: 3000,
			BatchSize:     10,
			StartIndex:    0,
		},
		PChainIndexer: IndexerConfig{
			Enabled:       true,
			TimeoutMillis: 3000,
			BatchSize:     10,
			StartIndex:    0,
		},
		UptimeCronjob: UptimeCronjobConfig{
			CronjobConfig: CronjobConfig{
				Enabled:        false,
				TimeoutSeconds: 60,
			},
			AggregateStartTimestamp:  0,
			AggregateIntervalSeconds: 3 * 24 * 60 * 60,
		},
		Chain: config.ChainConfig{
			NodeURL: "http://localhost:9650/",
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
