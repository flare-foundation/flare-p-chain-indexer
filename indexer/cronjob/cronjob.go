package cronjob

import (
	"flare-indexer/database"
	"flare-indexer/indexer/config"
	"flare-indexer/logger"
	"flare-indexer/utils"
	"flare-indexer/utils/staking"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Cronjob interface {
	Name() string
	Enabled() bool
	Timeout() time.Duration
	RandomTimeoutDelta() time.Duration
	Call() error
	OnStart() error
	OnError(err error)
}

func RunCronjob(c Cronjob) {
	if !c.Enabled() {
		logger.Debug("%s cronjob disabled", c.Name())
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
		if err != nil {
			logger.Error("%s cronjob error %s", c.Name(), err.Error())
			c.OnError(err)
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

type epochCronjobMetrics struct {
	// Current epoch
	lastEpoch prometheus.Gauge

	// Last processsed epoch
	lastProcessedEpoch prometheus.Gauge

	// Error count
	errorCount prometheus.Counter
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

func (c *epochCronjob) OnError(err error) {
	if c.metrics != nil {
		c.metrics.errorCount.Inc()
	}
}

// Get processing range (closed interval)
func (c *epochCronjob) getEpochRange(start int64, now time.Time) *epochRange {
	return c.getTrimmedEpochRange(start, c.epochs.GetEpochIndex(now)-1)
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

func (c *epochCronjob) indexerBehind(idxState *database.State, epoch int64) bool {
	epochEnd := c.epochs.GetEndTime(epoch)
	return epochEnd.After(idxState.Updated.Add(-c.delay)) || idxState.NextDBIndex <= idxState.LastChainIndex
}

func newEpochCronjobMetrics(namespace string) *epochCronjobMetrics {
	return &epochCronjobMetrics{
		lastEpoch: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_epoch",
			Help:      "Last completed epoch",
		}),
		lastProcessedEpoch: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_processed_epoch",
			Help:      "Last processed epoch",
		}),
		errorCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "error_count",
			Help:      "Number of errors",
		}),
	}
}
