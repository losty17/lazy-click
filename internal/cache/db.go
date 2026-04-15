package cache

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(
		&SpaceEntity{},
		&ListEntity{},
		&TaskEntity{},
		&TagEntity{},
		&TaskTagEntity{},
		&CommentEntity{},
		&SyncQueueEntity{},
	); err != nil {
		return nil, err
	}

	return db, nil
}
