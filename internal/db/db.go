package db

import (
	"fmt"
	"os"

	"github.com/kurosaki/l1/internal/yt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect() *gorm.DB {
	host := os.Getenv("HOST")
	dbPort := os.Getenv("DBPORT")
	user := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")
	dbName := os.Getenv("DBNAME")
	// host := "localhost"
	// dbPort := "5432"
	// user := "postgres"
	// password := "postgres"
	// dbName := "youtube_db"

	dbURI := fmt.Sprintf("host=%s user=%s dbname=%s sslmode=disable password=%s port=%s TimeZone=Asia/Seoul", host, user, dbName, password, dbPort)
	db, err := gorm.Open(postgres.Open(dbURI), &gorm.Config{})
	yt.HandlerError(err)
	return db
}
