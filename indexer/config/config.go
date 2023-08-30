package config

import (
	"flare-indexer/config"
	"flare-indexer/utils"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type Config struct {
	DB            config.DBConfig     `toml:"db"`
	Logger        config.LoggerConfig `toml:"logger"`
	Chain         config.ChainConfig  `toml:"chain"`
	Metrics       MetricsConfig       `toml:"metrics"`
	XChainIndexer IndexerConfig       `toml:"x_chain_indexer"`
	PChainIndexer IndexerConfig       `toml:"p_chain_indexer"`
	UptimeCronjob UptimeConfig        `toml:"uptime_cronjob"`
	Mirror        MirrorConfig        `toml:"mirroring_cronjob"`
	VotingCronjob VotingConfig        `toml:"voting_cronjob"`
	Epochs        EpochConfig         `toml:"epochs"`
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

type MirrorConfig struct {
	CronjobConfig
	MirroringContract common.Address `toml:"mirroring_contract" envconfig:"MIRRORING_CONTRACT_ADDRESS"`
}

type VotingConfig struct {
	CronjobConfig
	ContractAddress common.Address `toml:"contract_address" envconfig:"VOTING_CONTRACT_ADDRESS"`
}

type EpochConfig struct {
	Period time.Duration   `toml:"epoch_period" envconfig:"EPOCH_PERIOD"`
	Start  utils.Timestamp `toml:"epoch_time" envconfig:"EPOCH_TIME"`
}

type UptimeConfig struct {
	CronjobConfig
	EpochConfig
	EnableVoting    bool          `toml:"enable_voting"`
	VotingInterval  time.Duration `toml:"voting_interval"`
	UptimeThreshold float64       `toml:"uptime_threshold"`
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
		UptimeCronjob: UptimeConfig{
			CronjobConfig: CronjobConfig{
				Enabled:        false,
				TimeoutSeconds: 60,
			},
		},
		Chain: config.ChainConfig{
			NodeURL: "http://localhost:9650/",
		},
		Epochs: EpochConfig{
			Period: 90 * time.Second,
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
