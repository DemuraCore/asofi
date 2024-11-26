package main

import (
	"aso/asofi/config"
	"aso/asofi/controllers"
	"aso/asofi/middlewares"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	config.ConnectDB()

	r := gin.Default()

	r.POST("/register", controllers.Register)
	r.POST("/login", controllers.Login)
	r.GET("/validate", controllers.ValidateToken)

	protected := r.Group("/")
	protected.Use(middlewares.AuthMiddleware())
	protected.GET("/protected", func(c *gin.Context) {
		userID := c.MustGet("user_id")
		c.JSON(200, gin.H{"message": "Welcome", "user_id": userID})
	})

	r.Run("0.0.0.0:3000")
}
