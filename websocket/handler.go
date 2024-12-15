package websocket

import (
	"aso/asofi/channels"
	"aso/asofi/controllers"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var (
	clients     = make(map[*websocket.Conn]bool)            // Feed clients
	postClients = make(map[string]map[*websocket.Conn]bool) // Post-specific clients
)

func HandleConnections(c *gin.Context) {
	channel := c.Query("channel") // e.g., "feed" or "post"
	postID := c.Query("id")       // e.g., post ID for post channel

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer ws.Close()

	if channel == "feed" {
		handleFeedConnections(ws)
	} else if channel == "post" && postID != "" {
		handlePostConnections(ws, postID)
	} else {
		ws.WriteMessage(websocket.TextMessage, []byte("Invalid channel"))
		return
	}
}

func handleFeedConnections(ws *websocket.Conn) {
	clients[ws] = true
	defer delete(clients, ws)
	log.Println("Client connected to feed")

	// Keep connection alive
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			log.Println("Feed connection error:", err)
			break
		}
	}
}

func handlePostConnections(ws *websocket.Conn, postID string) {
	if _, ok := postClients[postID]; !ok {
		postClients[postID] = make(map[*websocket.Conn]bool)
	}
	postClients[postID][ws] = true
	defer func() {
		delete(postClients[postID], ws)
		if len(postClients[postID]) == 0 {
			delete(postClients, postID)
		}
		ws.Close()
	}()
	log.Printf("Client connected to post %s", postID)

	// Keep connection alive
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			log.Printf("Post %s connection error: %v", postID, err)
			break
		}
	}
}

func HandleMessages() {
	for {
		post := <-channels.FeedBroadcast

		postData := controllers.PostData{
			Post: post,
		}

		// Broadcast to feed clients
		for client := range clients {
			err := client.WriteJSON(postData)
			if err != nil {
				log.Println("Error writing to feed client:", err)
				client.Close()
				delete(clients, client)
			}
		}

		// Broadcast to post-specific clients
		postID := strconv.Itoa(int(post.ID))
		if postClients, ok := postClients[postID]; ok {
			for client := range postClients {
				err := client.WriteJSON(postData)
				if err != nil {
					log.Println("Error writing to post client:", err)
					client.Close()
					delete(postClients, client)
				}
			}
		}
	}
}
