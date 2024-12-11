package main

import (
	"aso/asofi/config"
	"aso/asofi/controllers"
	"aso/asofi/middlewares"

	"github.com/gin-contrib/cors"
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
	// config.DB.AutoMigrate(&models.User{}, &models.Post{}, &models.Like{}, &models.Comment{}, &models.Session{})

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	r.POST("/register", controllers.Register)
	r.POST("/login", controllers.Login)

	core := r.Group("/")
	core.Use(middlewares.AuthMiddleware())
	core.GET("/users", controllers.GetUsers)
	core.GET("/me", controllers.GetMe)

	core.POST("/posts", controllers.CreatePost)

	me := core.Group("/me")
	me.GET("/follow/:id", controllers.Follow)
	me.DELETE("/unfollow/:id", controllers.Unfollow)

	r.Run(":2425")
}
