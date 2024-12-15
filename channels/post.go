// channels/channel.go
package channels

import "aso/asofi/models"

// Shared broadcast channel for WebSocket messages
var FeedBroadcast = make(chan models.Post)
var PostBroadcast = make(chan models.Post)
var CommentBroadcast = make(chan models.Comment)
