package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("Starting Asofi API V1.0...")
	log.Printf("Starting server on port 3000")

	gin.SetMode(gin.DebugMode)
	router := gin.Default()

	// Main Router V1
	MainRouter := router.Group("/v1")
	MainRouter.GET("/v1/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})
}