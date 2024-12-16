// channels/channel.go
package channels

import "aso/asofi/models"

// Shared broadcast channel for WebSocket messages
var ProfileBroadcast = make(chan models.User)
