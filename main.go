package main

import (
	"aso/asofi/config"
	"aso/asofi/controllers"
	"aso/asofi/middlewares"
	"aso/asofi/models"
	"aso/asofi/websocket"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	config.ConnectDB()

	config.DB.AutoMigrate(&models.User{}, &models.Post{}, &models.Comment{}, &models.Like{}, &models.Session{}, &models.OTP{})

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
	r.DELETE("/logout", controllers.Logout)

	core := r.Group("/")
	core.Use(middlewares.AuthMiddleware())
	noyes := r.Group("/")
	noyes.Use(middlewares.AuthMiddlewareNotStrict())
	core.GET("/users", controllers.GetUsers)
	core.GET("/user/:username", controllers.GetUserProfile)
	core.GET("/me", controllers.GetMe)

	core.POST("/verify/send-code", controllers.SendCODE)
	core.POST("/verify/verify-email", controllers.VerifyCODE)

	core.POST("/posts", controllers.CreatePost)
	core.DELETE("/posts/:id", controllers.DeletePost)
	core.POST("/posts/like", controllers.LikePost)
	core.POST("/posts/unlike", controllers.UnlikePost)
	noyes.GET("/posts", controllers.ListPosts)

	me := core.Group("/me")
	me.GET("/follow/:id", controllers.Follow)
	me.DELETE("/unfollow/:id", controllers.Unfollow)

	r.GET("/ws", websocket.HandleConnections)

	go websocket.HandleMessages()

	r.Run(":2425")
}
