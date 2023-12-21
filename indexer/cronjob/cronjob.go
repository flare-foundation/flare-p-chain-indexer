package cronjob

import (
	"flare-indexer/indexer/config"
	"flare-indexer/indexer/shared"
	"flare-indexer/logger"
	"flare-indexer/utils"
	"flare-indexer/utils/staking"
	"time"
)

type Cronjob interface {
	Name() string
	Enabled() bool
	Timeout() time.Duration
	RandomTimeoutDelta() time.Duration
	Call() error
	OnStart() error

	// Set health status of cronjob
	// (can be implemented to ignore the status based on other conditions)
	UpdateCronjobStatus(status shared.HealthStatus)
}

func RunCronjob(c Cronjob) {
	if !c.Enabled() {
		logger.Debug("%s cronjob disabled", c.Name())
		c.UpdateCronjobStatus(shared.HealthStatusOk)
		return
	}

	err := c.OnStart()
	if err != nil {
		logger.Error("%s cronjob on start error %v", c.Name(), err)
		return
	}

	logger.Debug("starting %s cronjob", c.Name())

	ticker := utils.NewRandomizedTicker(c.Timeout(), c.RandomTimeoutDelta())
	for {
		<-ticker

		err := c.Call()
		if err == nil {
			c.UpdateCronjobStatus(shared.HealthStatusOk)
		} else {
			logger.Error("%s cronjob error %s", c.Name(), err.Error())
			c.UpdateCronjobStatus(shared.HealthStatusError)
		}
	}
}

const (
	defaultEpochBatchSize int64 = 100
)

type epochCronjob struct {
	enabled   bool
	timeout   time.Duration // call cronjob every "timeout"
	epochs    staking.EpochInfo
	delay     time.Duration // voting delay
	batchSize int64
	metrics   *epochCronjobMetrics
}

type epochRange struct {
	start int64
	end   int64
}

func newEpochCronjob(cronjobCfg *config.CronjobConfig, epochs staking.EpochInfo) epochCronjob {
	return epochCronjob{
		enabled:   cronjobCfg.Enabled,
		timeout:   cronjobCfg.Timeout,
		epochs:    epochs,
		batchSize: cronjobCfg.BatchSize,
		delay:     cronjobCfg.Delay,
	}
}

func (c *epochCronjob) Enabled() bool {
	return c.enabled
}

func (c *epochCronjob) Timeout() time.Duration {
	return c.timeout
}

func (c *epochCronjob) RandomTimeoutDelta() time.Duration {
	return 0
}

func (c *epochCronjob) UpdateCronjobStatus(status shared.HealthStatus) {
	if c.metrics != nil {
		c.metrics.SetStatus(status)
	}
}

// Get trimmed processing range (closed interval)
func (c *epochCronjob) getTrimmedEpochRange(start, end int64) *epochRange {
	start = utils.Max(start, c.epochs.First)
	batchSize := c.batchSize
	if batchSize == 0 {
		batchSize = defaultEpochBatchSize
	} else if batchSize < 0 {
		batchSize = end - start + 1
	}
	if end >= start+batchSize {
		end = batchSize + start - 1
	}
	return &epochRange{start, end}
}

func (c *epochCronjob) updateLastEpochMetrics(epoch int64) {
	if c.metrics != nil {
		c.metrics.lastEpoch.Set(float64(epoch))
	}
}

func (c *epochCronjob) updateLastProcessedEpochMetrics(epoch int64) {
	if c.metrics != nil {
		c.metrics.lastProcessedEpoch.Set(float64(epoch))
	}
}
