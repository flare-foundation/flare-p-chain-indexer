package context

import (
	"flag"
	globalConfig "flare-indexer/config"
	"flare-indexer/database"
	"flare-indexer/indexer/config"

	"gorm.io/gorm"
)

type IndexerContext interface {
	Config() *config.Config
	DB() *gorm.DB
	Flags() *IndexerFlags
}

type IndexerFlags struct {
	Version bool

	ConfigFileName string

	// Set start epoch for voting cronjob to this value, overrides config and database value,
	// valid value is > 0
	ResetVotingCronjob int64

	// Set start epoch for mirroring cronjob to this value, overrides config and database value,
	// valid value is > 0
	ResetMirrorCronjob int64
}

type indexerContext struct {
	config *config.Config
	db     *gorm.DB
	flags  *IndexerFlags
}

func BuildContext(flags *IndexerFlags) (IndexerContext, error) {
	cfg, err := config.BuildConfig(flags.ConfigFileName)
	if err != nil {
		return nil, err
	}
	globalConfig.GlobalConfigCallback.Call(cfg)

	db, err := database.ConnectAndInitialize(&cfg.DB)
	if err != nil {
		return nil, err
	}

	return &indexerContext{
		config: cfg,
		db:     db,
		flags:  flags,
	}, nil
}

func (c *indexerContext) Config() *config.Config { return c.config }

func (c *indexerContext) DB() *gorm.DB { return c.db }

func (c *indexerContext) Flags() *IndexerFlags { return c.flags }

func ParseIndexerFlags() *IndexerFlags {
	versionFlag := flag.Bool("version", false, "Print version information and exit")
	cfgFlag := flag.String("config", globalConfig.CONFIG_FILE, "Configuration file (toml format)")
	resetVotingFlag := flag.Int64("reset-voting", 0, "Set start epoch for voting cronjob to this value, overrides config and database value, valid values are > 0")
	resetMirrorFlag := flag.Int64("reset-mirroring", 0, "Set start epoch for mirroring cronjob to this value, overrides config and database value, valid values are > 0")
	flag.Parse()

	return &IndexerFlags{
		Version:            *versionFlag,
		ConfigFileName:     *cfgFlag,
		ResetVotingCronjob: *resetVotingFlag,
		ResetMirrorCronjob: *resetMirrorFlag,
	}
}
