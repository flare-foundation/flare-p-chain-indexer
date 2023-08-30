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
	config config.UptimeConfig
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

func (c *uptimeCronjob) OnStart() error {
	entities := []*database.UptimeCronjob{&database.UptimeCronjob{
		NodeID:    nil,
		Status:    database.UptimeCronjobStatusIndexerStarted,
		Timestamp: time.Now(),
	}}
	return database.CreateUptimeCronjobEntry(c.db, entities)
}

func (c *uptimeCronjob) Call() error {
	validators, status, err := CallPChainGetConnectedValidators(c.client)
	if err != nil {
		return err
	}

	now := time.Now()
	var entities []*database.UptimeCronjob
	if status < 0 {
		entities = []*database.UptimeCronjob{&database.UptimeCronjob{
			NodeID:    nil,
			Status:    database.UptimeCronjobStatus(status),
			Timestamp: now,
		}}
	} else {
		entities = make([]*database.UptimeCronjob, len(validators))
		for i, v := range validators {
			nodeID := v.NodeID.String()
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
