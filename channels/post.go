// channels/channel.go
package channels

import "aso/asofi/models"

// Shared broadcast channel for WebSocket messages
var Broadcast = make(chan models.Post)
