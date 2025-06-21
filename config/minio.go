package config

import (
	"log"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Initialize Minio client (add this to your config package)
var MinioClient *minio.Client

func InitMinio() {

	accessKeyID := os.Getenv("MINIO_ACCESS")
	secretAccessKey := os.Getenv("MINIO_SECRET")
	endpoint := os.Getenv("MINIO_ENDPOINT")
	useSSL := true

	var err error
	MinioClient, err = minio.New(endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("%#v\n", minioClient)
}
