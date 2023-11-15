package context

import (
	"flag"
	globalConfig "flare-indexer/config"
	"flare-indexer/database"
	"flare-indexer/services/config"

	"github.com/ethereum/go-ethereum/ethclient"

	"gorm.io/gorm"
)

type ServicesContext interface {
	Config() *config.Config
	DB() *gorm.DB
	EthRPCClient() *ethclient.Client
}

type ServicesFlags struct {
	ConfigFileName string
}

type servicesContext struct {
	config       *config.Config
	db           *gorm.DB
	ethRPCClient *ethclient.Client
}

func BuildContext() (ServicesContext, error) {
	flags := parseServicesFlags()

	cfg, err := config.BuildConfig(flags.ConfigFileName)
	if err != nil {
		return nil, err
	}
	globalConfig.GlobalConfigCallback.Call(cfg)

	db, err := database.Connect(&cfg.DB)
	if err != nil {
		return nil, err
	}

	ethRPCClient, err := cfg.Chain.DialETH()
	if err != nil {
		return nil, err
	}

	return &servicesContext{
		config:       cfg,
		db:           db,
		ethRPCClient: ethRPCClient,
	}, nil
}

func (c *servicesContext) Config() *config.Config { return c.config }

func (c *servicesContext) DB() *gorm.DB { return c.db }

func (c *servicesContext) EthRPCClient() *ethclient.Client { return c.ethRPCClient }

func parseServicesFlags() *ServicesFlags {
	cfgFlag := flag.String("config", globalConfig.CONFIG_FILE, "Configuration file (toml format)")
	flag.Parse()
	return &ServicesFlags{
		ConfigFileName: *cfgFlag,
	}
}
