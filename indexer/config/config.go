package config

import (
	"flare-indexer/config"
	"flare-indexer/utils"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type Config struct {
	DB                config.DBConfig     `toml:"db"`
	Logger            config.LoggerConfig `toml:"logger"`
	Chain             config.ChainConfig  `toml:"chain"`
	Metrics           MetricsConfig       `toml:"metrics"`
	XChainIndexer     IndexerConfig       `toml:"x_chain_indexer"`
	PChainIndexer     IndexerConfig       `toml:"p_chain_indexer"`
	UptimeCronjob     UptimeConfig        `toml:"uptime_cronjob"`
	Mirror            MirrorConfig        `toml:"mirroring_cronjob"`
	VotingCronjob     VotingConfig        `toml:"voting_cronjob"`
	ContractAddresses ContractAddresses   `toml:"contract_addresses"`
}

type MetricsConfig struct {
	PrometheusAddress string `toml:"prometheus_address" envconfig:"PROMETHEUS_ADDRESS"`
}

type IndexerConfig struct {
	Enabled       bool          `toml:"enabled"`
	Timeout       time.Duration `toml:"timeout"`
	BatchSize     int           `toml:"batch_size"`
	StartIndex    uint64        `toml:"start_index"`
	SaveAddresses bool          `toml:"save_addresses"` // only for p-chain indexer
}

type CronjobConfig struct {
	Enabled   bool          `toml:"enabled"`
	Timeout   time.Duration `toml:"timeout"`
	BatchSize int64         `toml:"batch_size"`
	Delay     time.Duration `toml:"delay"`
}

type MirrorConfig struct {
	CronjobConfig
	config.EpochConfig
}

type VotingConfig struct {
	CronjobConfig
	config.EpochConfig
	GasLimit uint64 `toml:"gas_limit" envconfig:"VOTING_GAS_LIMIT"`
}

type UptimeConfig struct {
	CronjobConfig
	Period                         time.Duration   `toml:"period" envconfig:"UPTIME_EPOCH_PERIOD"`
	Start                          utils.Timestamp `toml:"start" envconfig:"UPTIME_EPOCH_START"`
	First                          int64           `toml:"first" envconfig:"UPTIME_EPOCH_FIRST"`
	EnableVoting                   bool            `toml:"enable_voting"`
	UptimeThreshold                float64         `toml:"uptime_threshold"`
	DeleteOldUptimesEpochThreshold int64           `toml:"delete_old_uptimes_epoch_threshold"`
}

type ContractAddresses struct {
	config.ContractAddresses
	Mirroring common.Address `toml:"mirroring" envconfig:"MIRRORING_CONTRACT_ADDRESS"`
}

func newConfig() *Config {
	return &Config{
		XChainIndexer: IndexerConfig{
			Enabled:    true,
			Timeout:    3000 * time.Millisecond,
			BatchSize:  10,
			StartIndex: 0,
		},
		PChainIndexer: IndexerConfig{
			Enabled:    true,
			Timeout:    3000 * time.Millisecond,
			BatchSize:  10,
			StartIndex: 0,
		},
		UptimeCronjob: UptimeConfig{
			CronjobConfig: CronjobConfig{
				Enabled: false,
				Timeout: 60 * time.Second,
			},
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
