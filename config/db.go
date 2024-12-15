package config

import (
	"log"

	_ "github.com/lib/pq"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDB() {

	// host := os.Getenv("DB_HOST")
	// portStr := os.Getenv("DB_PORT")
	// port, err := strconv.Atoi(portStr)
	// if err != nil {
	// 	log.Fatalf("Invalid port number: %v", err)
	// }
	// user := os.Getenv("DB_USER")
	// password := os.Getenv("DB_PASSWORD")
	// dbname := os.Getenv("DB_NAME")

	// dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
	// 	host, port, user, password, dbname)

	// db, err := gorm.Open(sq.Open(dsn), &gorm.Config{})
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	DB = db
	log.Println("Connected to PostgreSQL database!")
}
