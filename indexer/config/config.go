package config

import (
	"math/big"
	"time"

	"flare-indexer/config"
	"flare-indexer/utils"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
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

type Gas struct {
	GasLimit uint64 `toml:"gas_limit"` // Gas limit to set for the transaction execution (0 = estimate)

	// type 0
	GasPrice *big.Int `toml:"gas_price"` // Gas price to use for the transaction execution (nil = gas price oracle)

	// type 2
	GasFeeCap *big.Int `toml:"gas_fee_cap"` // Gas fee cap to use for the 1559 transaction execution (nil = gas price oracle)
	GasTipCap *big.Int `toml:"gas_tip_cap"` // Gas priority fee cap to use for the 1559 transaction execution (nil = gas price oracle)
}

func (g *Gas) SetTransactOpts(txOpts *bind.TransactOpts) {
	txOpts.GasLimit = g.GasLimit
	txOpts.GasPrice = g.GasPrice
	txOpts.GasFeeCap = g.GasFeeCap
	txOpts.GasTipCap = g.GasTipCap
}

type MetricsConfig struct {
	PrometheusAddress string `toml:"prometheus_address" envconfig:"PROMETHEUS_ADDRESS"`
}

type IndexerConfig struct {
	Enabled    bool          `toml:"enabled"`
	Timeout    time.Duration `toml:"timeout"`
	BatchSize  int           `toml:"batch_size"`
	StartIndex uint64        `toml:"start_index"`
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
	Gas Gas `toml:"gas"`
}

type VotingConfig struct {
	CronjobConfig
	config.EpochConfig
	Gas Gas `toml:"gas"`
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
