package database

import (
	"time"
)

type UptimeCronjob struct {
	BaseEntity
	Timestamp time.Time `gorm:"index"`
	NodeID    string    `gorm:"type:varchar(60);index"`
	Connected bool
}
