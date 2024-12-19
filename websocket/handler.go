package websocket

import (
	"aso/asofi/channels"
	"aso/asofi/controllers"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const (
	MaxConnectionsPerUser = 3
	HeartbeatInterval     = 30 * time.Second
)

var (
	clients        = make(map[*websocket.Conn]bool)            // Feed clients
	postClients    = make(map[string]map[*websocket.Conn]bool) // Post-specific clients
	profileClients = make(map[string]map[*websocket.Conn]bool) // Profile clients
)

func HandleConnections(c *gin.Context) {
	channel := c.Query("channel") // e.g., "feed" or "post" or "profile"
	ID := c.Query("id")           // e.g., post ID for post channel

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer ws.Close()

	if channel == "feed" {
		handleFeedConnections(ws)
	} else if channel == "post" && ID != "" {
		handlePostConnections(ws, ID)
	} else if channel == "profile" && ID != "" {
		handleProfileConnections(ws, ID)
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

func handleProfileConnections(ws *websocket.Conn, userID string) {
	if _, ok := profileClients[userID]; !ok {
		profileClients[userID] = make(map[*websocket.Conn]bool)
	}
	profileClients[userID][ws] = true
	defer func() {
		delete(profileClients[userID], ws)
		if len(profileClients[userID]) == 0 {
			delete(profileClients, userID)
		}
		ws.Close()
	}()
	log.Printf("Client connected to profile %s", userID)

	// Keep connection alive
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			log.Printf("Profile %s connection error: %v", userID, err)
			break
		}
	}
}

func HandleMessages() {
	for {
		select {
		case post := <-channels.FeedBroadcast:
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
			// Broadcasr to profile-specific clients
			if profileClients, ok := profileClients[strconv.Itoa(int(post.UserID))]; ok {
				for client := range profileClients {
					err := client.WriteJSON(postData)
					if err != nil {
						log.Println("Error writing to profile client:", err)
						client.Close()
						delete(profileClients, client)
					}
				}
			}

		case comment := <-channels.CommentBroadcast:
			commentData := controllers.CommentData{
				Comment: comment,
			}
			postID := strconv.Itoa(int(comment.PostID))
			if postClients, ok := postClients[postID]; ok {
				for client := range postClients {
					err := client.WriteJSON(commentData)
					if err != nil {
						log.Println("Error writing to post client:", err)
						client.Close()
						delete(postClients, client)
					}
				}
			}
		case profile := <-channels.ProfileBroadcast:
			profileData := controllers.ProfileData{
				Profile: profile,
			}

			if profileClients, ok := profileClients[strconv.Itoa(int(profile.ID))]; ok {
				for client := range profileClients {
					err := client.WriteJSON(profileData)
					if err != nil {
						log.Println("Error writing to profile client:", err)
						client.Close()
						delete(profileClients, client)
					}
				}
			}
		}
	}
}
