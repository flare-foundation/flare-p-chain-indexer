package database

import (
	"time"
)

type UptimeCronjobStatus int8

const (
	UptimeCronjobStatusConnected      UptimeCronjobStatus = 1
	UptimeCronjobStatusDisconnected   UptimeCronjobStatus = 0
	UptimeCronjobStatusTimeout        UptimeCronjobStatus = -1
	UptimeCronjobStatusServiceError   UptimeCronjobStatus = -2
	UptimeCronjobStatusIndexerStarted UptimeCronjobStatus = -3
)

type UptimeCronjob struct {
	BaseEntity
	Timestamp time.Time `gorm:"index"`
	NodeID    *string   `gorm:"type:varchar(60);index"`
	Status    UptimeCronjobStatus
}

// Start time and end time of the aggregation period are redundant since they can
// be calculated from epoch
type UptimeAggregation struct {
	BaseEntity
	Epoch     int `gorm:"uniqueIndex:idx_epoch_node_index;index"`
	StartTime time.Time
	EndTime   time.Time
	NodeID    string `gorm:"uniqueIndex:idx_epoch_node_index;type:varchar(60)"`

	// Uptime in seconds in the given epoch interval [StartTime, EndTime]
	Value int64
}
