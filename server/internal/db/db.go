package db

import (
	"log"

	"github.com/aitjcize/photoframe-server/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Init(dbPath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	log.Println("Database connection established")

	// Auto Migrate Schema
	err = db.AutoMigrate(
		&model.Setting{},
		&model.Image{},
		&model.GoogleAuth{},
	)
	if err != nil {
		return nil, err
	}

	return db, nil
}
