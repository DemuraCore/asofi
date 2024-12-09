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
	// delete all tables
	// config.DB.Migrator().DropTable(&models.User{}, &models.Post{}, &models.Like{}, &models.Comment{})
	// delete relationship table
	// config.DB.Migrator().DropTable("user_follows")
	// config.DB.AutoMigrate(&models.User{}, &models.Post{}, &models.Like{}, &models.Comment{})

	r := gin.Default()

	r.POST("/register", controllers.Register)
	r.POST("/login", controllers.Login)

	core := r.Group("/")
	core.Use(middlewares.AuthMiddleware())
	core.GET("/protected", func(c *gin.Context) {
		userID := c.MustGet("user_id")
		c.JSON(200, gin.H{"message": "Welcome", "user_id": userID})
	})
	core.GET("/users", controllers.GetUsers)
	core.GET("/me", controllers.GetMe)
	core.GET("/follow/:id", controllers.Follow)
	core.DELETE("/unfollow/:id", controllers.Unfollow)
	r.Run("0.0.0.0:3000")
}
