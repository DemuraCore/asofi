package main

import (
	"aso/asofi/config"
	"aso/asofi/controllers"
	"aso/asofi/middlewares"
	"aso/asofi/models"
	"aso/asofi/websocket"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func main() {
	godotenv.Load()
	config.ConnectDB()
	config.InitRedis()
	// config.DB.Migrator().DropTable(&models.Post{}, &models.Comment{}, &models.Like{}, &models.Session{}, &models.OTP{}, &models.Role{}, &models.UserFollow{}, &models.Report{})

	// config.DB.AutoMigrate(&models.User{}, &models.Post{}, &models.Comment{}, &models.Like{}, &models.Session{}, &models.OTP{}, &models.Role{}, &models.UserFollow{}, &models.Report{})

	// initializeDB()

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "https://aso.vahry.my.id"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Accept", "Cache-Control", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.POST("/register", controllers.Register)
	r.POST("/login", controllers.Login)
	r.DELETE("/logout", controllers.Logout)
	r.MaxMultipartMemory = 8 << 20 // 8 MiB

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
	noyes.GET("/posts/:id", controllers.GetPost)
	noyes.GET("/posts/user/:username", controllers.GetPostByUser)
	core.DELETE("/posts/:id", controllers.DeletePost)
	core.PUT("/posts/:id", controllers.UpdatePost)
	core.POST("/posts/like", controllers.LikePost)
	core.POST("/posts/unlike", controllers.UnlikePost)
	core.POST("/posts/:id/comments", controllers.CreateComment)
	core.DELETE("/posts/comments/:commentID", controllers.DeleteComment)
	core.PUT("/posts/comments/:commentID", controllers.UpdateComment)
	noyes.GET("/posts/:id/comments", controllers.GetComments)
	noyes.GET("/posts", controllers.ListPosts)

	me := core.Group("/me")
	me.GET("/follow/:id", controllers.Follow)
	me.DELETE("/unfollow/:id", controllers.Unfollow)
	me.PATCH("/update", controllers.EditProfile)

	// simple health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.GET("/ws", websocket.HandleConnections)

	go websocket.HandleMessages()

	r.Run(":2425")
}

func initializeDB() {
	roles := []string{"User", "Moderator", "Admin"}
	for _, roleName := range roles {
		var role models.Role
		if err := config.DB.Where("name = ?", roleName).First(&role).Error; err != nil {
			if err := config.DB.Create(&models.Role{Name: roleName}).Error; err != nil {
				log.Fatalf("Failed to create role %s: %v", roleName, err)
			}
		}
	}

	// Create admin user
	var adminRole models.Role
	if err := config.DB.Where("name = ?", "Admin").First(&adminRole).Error; err != nil {
		log.Fatalf("Failed to find role Admin: %v", err)
	}

	var adminUser models.User
	if err := config.DB.Where("username = ?", "admin").First(&adminUser).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
			if err != nil {
				log.Fatalf("Failed to hash password: %v", err)
			}

			adminUser = models.User{
				Username: "admin",
				Name:     "Admin User",
				Verified: true,
				Email:    "admin@example.com",
				Password: string(hashedPassword),
				RoleID:   adminRole.ID,
			}

			if err := config.DB.Create(&adminUser).Error; err != nil {
				log.Fatalf("Failed to create admin user: %v", err)
			}
		} else {
			log.Fatalf("Failed to find admin user: %v", err)
		}
	}
}
