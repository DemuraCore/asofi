package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	sq "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDB() {

	host := os.Getenv("DB_HOST")
	portStr := os.Getenv("DB_PORT")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatalf("Invalid port number: %v", err)
	}
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := gorm.Open(sq.Open(dsn), &gorm.Config{})
	// db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	DB = db
	log.Println("Connected to PostgreSQL database!")
}

var RedisClient *redis.Client
var RedisCtx = context.Background()

func InitRedis() {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     "194.15.36.47:1432", // Redis server address
		Password: "min2kota",          // No password by default
		DB:       0,                   // Use default DB
	})

	// Test connection
	if _, err := RedisClient.Ping(RedisCtx).Result(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Redis connected successfully!")
}

func SetCache(key string, value interface{}, expiration time.Duration) error {
	return RedisClient.Set(RedisCtx, key, value, expiration).Err()
}

func GetCache(key string) (string, error) {
	return RedisClient.Get(RedisCtx, key).Result()
}
