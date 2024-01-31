package database

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func FetchLastUptimeAggregation(db *gorm.DB) (*UptimeAggregation, error) {
	var lastAggregation UptimeAggregation
	err := db.Order("epoch desc").First(&lastAggregation).Error
	if err == nil {
		return &lastAggregation, nil
	} else if err == gorm.ErrRecordNotFound {
		return nil, nil
	} else {
		return nil, err
	}
}

func FetchNodeUptimes(db *gorm.DB, nodeID string, startTime time.Time, endTime time.Time) ([]UptimeCronjob, error) {
	var uptimes []UptimeCronjob
	err := db.Where("node_id = ? AND timestamp >= ? AND timestamp < ?", nodeID, startTime, endTime).Order("timestamp asc").Find(&uptimes).Error
	return uptimes, err
}

func PersistUptimeAggregations(db *gorm.DB, aggregations []*UptimeAggregation) error {
	if len(aggregations) == 0 {
		return nil
	}
	return db.Create(aggregations).Error
}

func DeleteUptimesBefore(db *gorm.DB, timestamp time.Time) error {
	return db.Where("timestamp < ?", timestamp).Delete(&UptimeCronjob{}).Error
}

func CreateAddresses(db *gorm.DB, addresses []*Address) error {
	if len(addresses) == 0 {
		return nil
	}

	return db.Clauses(clause.OnConflict{UpdateAll: true}).Create(addresses).Error
}

func FetchAddress(db *gorm.DB, addr string) (*Address, error) {
	var address Address
	err := db.Where("eth_address = ?", addr).Or("bech_address = ?", addr).First(&address).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &address, err
}
