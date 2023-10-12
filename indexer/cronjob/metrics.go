package cronjob

import (
	"flare-indexer/indexer/shared"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type epochCronjobMetrics struct {
	shared.MetricsBase

	// Current epoch
	lastEpoch prometheus.Gauge

	// Last processsed epoch
	lastProcessedEpoch prometheus.Gauge
}

func newEpochCronjobMetrics(namespace string) *epochCronjobMetrics {
	return &epochCronjobMetrics{
		MetricsBase: *shared.NewMetricsBase(namespace),
		lastEpoch: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_epoch",
			Help:      "Last completed epoch",
		}),
		lastProcessedEpoch: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_processed_epoch",
			Help:      "Last processed epoch",
		}),
	}
}
