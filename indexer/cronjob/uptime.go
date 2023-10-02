package cronjob

import (
	"flare-indexer/database"
	"flare-indexer/indexer/config"
	"flare-indexer/indexer/context"
	"flare-indexer/utils"
	"flare-indexer/utils/chain"
	"time"

	"gorm.io/gorm"
)

type uptimeCronjob struct {
	config config.UptimeConfig
	db     *gorm.DB

	client chain.UptimeClient
}

func NewUptimeCronjob(ctx context.IndexerContext) Cronjob {
	endpoint := utils.JoinPaths(ctx.Config().Chain.NodeURL, "ext/bc/P"+chain.RPCClientOptions(ctx.Config().Chain.ApiKey))
	return &uptimeCronjob{
		config: ctx.Config().UptimeCronjob,
		db:     ctx.DB(),
		client: chain.NewAvalancheUptimeClient(endpoint),
	}
}

func (c *uptimeCronjob) Name() string {
	return "uptime"
}

func (c *uptimeCronjob) Timeout() time.Duration {
	return c.config.Timeout
}

func (c *uptimeCronjob) Enabled() bool {
	return c.config.Enabled
}

func (c *uptimeCronjob) RandomTimeoutDelta() time.Duration {
	return 0
}

func (c *uptimeCronjob) OnStart() error {
	entities := []*database.UptimeCronjob{&database.UptimeCronjob{
		NodeID:    nil,
		Status:    database.UptimeCronjobStatusIndexerStarted,
		Timestamp: c.client.Now(),
	}}
	return database.CreateUptimeCronjobEntry(c.db, entities)
}

func (c *uptimeCronjob) Call() error {
	validators, status, err := c.client.GetValidatorStatus()
	if err != nil {
		return err
	}
	now := c.client.Now()
	var entities []*database.UptimeCronjob
	if status < 0 {
		entities = []*database.UptimeCronjob{&database.UptimeCronjob{
			NodeID:    nil,
			Status:    status,
			Timestamp: now,
		}}
	} else {
		entities = make([]*database.UptimeCronjob, len(validators))
		for i, v := range validators {
			nodeID := v.NodeID
			var status database.UptimeCronjobStatus
			if v.Connected {
				status = database.UptimeCronjobStatusConnected
			} else {
				status = database.UptimeCronjobStatusDisconnected
			}
			entities[i] = &database.UptimeCronjob{
				NodeID:    &nodeID,
				Status:    status,
				Timestamp: now,
			}
		}
	}
	return database.CreateUptimeCronjobEntry(c.db, entities)
}
