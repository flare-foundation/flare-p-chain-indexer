package cronjob

import (
	"flare-indexer/database"
	"flare-indexer/indexer/migrations"
	"time"

	"gorm.io/gorm"
)

func init() {
	migrations.Container.Add("2023-08-25-00-00", "Create initial state for voting cronjob", createVotingCronjobState)
	migrations.Container.Add("2023-08-30-00-00", "Create initial state for mirror cronjob", createMirrorCronjobState)
	migrations.Container.Add("2023-09-30-00-00", "Create initial state for address binder cronjob", createAddressBinderCronjobState)

}

func createVotingCronjobState(db *gorm.DB) error {
	return database.CreateState(db, &database.State{
		Name:           votingStateName,
		NextDBIndex:    0,
		LastChainIndex: 0,
		Updated:        time.Now(),
	})
}

func createMirrorCronjobState(db *gorm.DB) error {
	return database.CreateState(db, &database.State{
		Name:           mirrorStateName,
		NextDBIndex:    0,
		LastChainIndex: 0,
		Updated:        time.Now(),
	})
}

func createAddressBinderCronjobState(db *gorm.DB) error {
	return database.CreateState(db, &database.State{
		Name:           addressBinderStateName,
		NextDBIndex:    0,
		LastChainIndex: 0,
		Updated:        time.Now(),
	})
}
