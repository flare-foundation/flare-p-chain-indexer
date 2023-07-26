package database

import (
	"time"

	"gorm.io/gorm"
)

func FetchState(db *gorm.DB, name string) (State, error) {
	var currentState State
	err := db.Where(&State{Name: name}).First(&currentState).Error
	return currentState, err
}

func FetchMigrations(db *gorm.DB) ([]Migration, error) {
	var migrations []Migration
	err := db.Order("version asc").Find(&migrations).Error
	return migrations, err
}

func CreateMigration(db *gorm.DB, m *Migration) error {
	return db.Create(m).Error
}

func UpdateMigration(db *gorm.DB, m *Migration) error {
	return db.Save(m).Error
}

func CreateState(db *gorm.DB, s *State) error {
	return db.Create(s).Error
}

func UpdateState(db *gorm.DB, s *State) error {
	return db.Save(s).Error
}

func CreateUptimeCronjobEntry(db *gorm.DB, entities []*UptimeCronjob) error {
	if len(entities) > 0 {
		return db.Create(entities).Error
	}
	return nil
}

func FetchLastUptimeAggregation(db *gorm.DB) ([]UptimeAggregation, error) {
	var lastAggregation []UptimeAggregation
	err := db.Raw("SELECT node_groups.* FROM (SELECT ua.*, ROW_NUMBER() OVER (PARTITION BY ua.node_id ORDER BY ua.timestamp DESC) as rownum FROM uptime_aggregations ua) as node_groups WHERE node_groups.rownum = 1").Scan(&lastAggregation).Error
	return lastAggregation, err
}

func FetchNodeUptimes(db *gorm.DB, nodeID string, startTime time.Time, endTime time.Time) ([]UptimeCronjob, error) {
	var uptimes []UptimeCronjob
	err := db.Where("(node_id = ? OR )AND timestamp >= ? AND timestamp < ?", nodeID, startTime, endTime).Order("timestamp asc").Find(&uptimes).Error
	return uptimes, err
}
