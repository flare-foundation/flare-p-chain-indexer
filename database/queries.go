package database

import (
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
