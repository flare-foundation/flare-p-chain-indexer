package cronjob

import (
	"flare-indexer/database"
	"flare-indexer/indexer/config"
	"flare-indexer/indexer/context"
	"flare-indexer/utils"
	"flare-indexer/utils/chain"
	"time"

	"github.com/ybbus/jsonrpc/v3"
	"gorm.io/gorm"
)

type uptimeCronjob struct {
	config config.CronjobConfig
	db     *gorm.DB

	client jsonrpc.RPCClient
}

func NewUptimeCronjob(ctx context.IndexerContext) Cronjob {
	client := jsonrpc.NewClient(utils.JoinPaths(ctx.Config().Chain.NodeURL, "ext/bc/P"+chain.RPCClientOptions(ctx.Config().Chain.ApiKey)))
	return &uptimeCronjob{
		config: ctx.Config().UptimeCronjob,
		db:     ctx.DB(),
		client: client,
	}
}

func (c *uptimeCronjob) Name() string {
	return "uptime"
}

func (c *uptimeCronjob) TimeoutSeconds() int {
	return c.config.TimeoutSeconds
}

func (c *uptimeCronjob) Enabled() bool {
	return c.config.Enabled
}

func (c *uptimeCronjob) Call() error {
	validators, err := CallPChainGetConnectedValidators(c.client)
	if err != nil {
		return err
	}

	now := time.Now()
	entities := make([]*database.UptimeCronjob, len(validators))
	for i, v := range validators {
		entities[i] = &database.UptimeCronjob{
			NodeID:    v.NodeID.String(),
			Connected: v.Connected,
			Timestamp: now,
		}
	}
	return database.CreateUptimeCronjobEntry(c.db, entities)
}
