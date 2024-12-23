package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Initialize Minio client (add this to your config package)
var MinioClient *minio.Client

func init() {
	// Load environment variables from .env file
	godotenv.Load()

	accessKeyID := os.Getenv("MINIO_ACCESS")
	secretAccessKey := os.Getenv("MINIO_SECRET")
	endpoint := os.Getenv("MINIO_ENDPOINT")
	// useSSL := true

	var err error
	MinioClient, err = minio.New(endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
	})
	if err != nil {
		log.Fatalln(err)
	}
}
