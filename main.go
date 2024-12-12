// Main.go
package main

import (
	"aso/asofi/channels" // Import the shared channel
	"aso/asofi/config"
	"aso/asofi/controllers"
	"aso/asofi/middlewares"
	"aso/asofi/models"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clients = make(map[*websocket.Conn]bool)

func handleConnections(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer func() {
		delete(clients, ws)
		ws.Close()
	}()
	clients[ws] = true
	log.Println("Client connected")

	for {
		var post models.Post
		err := ws.ReadJSON(&post)
		if err != nil {
			log.Println("Error reading JSON:", err)
			delete(clients, ws)
			break
		}
		channels.Broadcast <- post
	}
}

func handleMessages() {
	for {
		post := <-channels.Broadcast
		log.Printf("Broadcasting post: %+v\n", post)
		for client := range clients {
			err := client.WriteJSON(post)
			if err != nil {
				log.Println("Error writing JSON to client:", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}

func main() {
	godotenv.Load()
	config.ConnectDB()

	// config.DB.AutoMigrate(&models.User{}, &models.Post{}, &models.Comment{}, &models.Like{}, &models.Session{}, &models.OTP{})

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
	core.GET("/users", controllers.GetUsers)
	core.GET("/user/:username", controllers.GetUserProfile)
	core.GET("/me", controllers.GetMe)

	core.POST("/verify/send-code", controllers.SendCODE)
	core.POST("/verify/verify-email", controllers.VerifyCODE)

	core.POST("/posts", controllers.CreatePost)
	core.DELETE("/posts/:id", controllers.DeletePost)
	r.GET("/posts", controllers.ListPosts)

	me := core.Group("/me")
	me.GET("/follow/:id", controllers.Follow)
	me.DELETE("/unfollow/:id", controllers.Unfollow)

	r.GET("/ws", handleConnections)

	go handleMessages()

	r.Run(":2425")
}
