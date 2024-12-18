package controllers

import (
	"aso/asofi/channels"
	"aso/asofi/config"
	"aso/asofi/models"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CommentRequest struct {
	Content string `json:"content" binding:"required"`
}

type CommentData struct {
	Comment models.Comment
}

func CreateComment(c *gin.Context) {
	postID := c.Param("id")
	userID := int(c.MustGet("user_id").(float64))

	var post models.Post
	if err := config.DB.Preload("User").Where("id = ?", postID).First(&post).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	var req CommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var comment models.Comment

	// Perform the transaction
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		// Insert the comment
		comment = models.Comment{
			Content: req.Content,
			UserID:  uint(userID),
			PostID:  post.ID,
		}
		if err := tx.Create(&comment).Error; err != nil {
			return err
		}

		// Reload the comment with the user information
		if err := tx.Preload("User").First(&comment, comment.ID).Error; err != nil {
			return err
		}

		// Update the comment_count in the posts table
		if err := tx.Model(&post).UpdateColumn("comment_count", gorm.Expr("comment_count + 1")).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction failed"})
		return
	}

	// Respond to the client immediately
	c.JSON(http.StatusCreated, gin.H{"data": comment})

	// Perform background tasks asynchronously
	go func() {
		channels.FeedBroadcast <- post
		channels.CommentBroadcast <- comment

		InvalidateCommentsCache(post.ID)
		PreloadCommentsCache(post.ID, 1) // Preload page 1 cache after invalidation
	}()
}

func DeleteComment(c *gin.Context) {
	commentID := c.Param("commentID")
	userID := int(c.MustGet("user_id").(float64))

	var comment models.Comment
	if err := config.DB.Where("id = ? AND user_id = ?", commentID, userID).First(&comment).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comment not found or you are not authorized to delete this comment"})
		return
	}

	var post models.Post
	if err := config.DB.Preload("User").Where("id = ?", comment.PostID).First(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to fetch post"})
		return
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {

		if err := tx.Delete(&comment).Error; err != nil {
			return err
		}

		if err := tx.Model(&post).UpdateColumn("comment_count", gorm.Expr("comment_count - ?", 1)).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction failed"})
		return
	}
	go func() {
		channels.FeedBroadcast <- post
		comment.UserID = 0
		channels.CommentBroadcast <- comment
		InvalidateCommentsCache(comment.PostID)
		PreloadCommentsCache(comment.PostID, 1) // Preload page 1 cache after invalidation
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Comment deleted successfully"})
}

func UpdateComment(c *gin.Context) {
	commentID := c.Param("commentID")
	userID := int(c.MustGet("user_id").(float64))

	var comment models.Comment
	if err := config.DB.Preload("User").Where("id = ? AND user_id = ?", commentID, userID).First(&comment).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comment not found or you are not authorized to update this comment"})
		return
	}

	var req CommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Content is required"})
		return
	}

	comment.Content = req.Content

	if err := config.DB.Save(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating comment"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": comment})
	go func() {
		channels.CommentBroadcast <- comment
		InvalidateCommentsCache(comment.PostID)
		PreloadCommentsCache(comment.PostID, 1) // Preload page 1 cache after invalidation
	}()

}

func GetComments(c *gin.Context) {
	postID := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit
	cacheKey := fmt.Sprintf("post:%s:comments:page:%d", postID, page)

	// Attempt to retrieve cached comments
	if cachedComments, err := config.GetCache(cacheKey); err == nil && cachedComments != "" {
		var comments []models.Comment
		if err := json.Unmarshal([]byte(cachedComments), &comments); err == nil {
			c.JSON(http.StatusOK, gin.H{"data": comments})
			return
		}
	}

	// Fetch post and validate existence
	var post models.Post
	if err := config.DB.Where("id = ?", postID).First(&post).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	// Fetch comments from the database
	var comments []models.Comment
	if err := config.DB.Preload("User").Where("post_id = ?", postID).
		Order("created_at desc").Limit(limit).Offset(offset).Find(&comments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Cache comments for faster future access
	if cacheData, err := json.Marshal(comments); err == nil {
		_ = config.SetCache(cacheKey, cacheData, 5*time.Minute)
	}

	c.JSON(http.StatusOK, gin.H{"data": comments})
}

// Invalidate cache for all pages of the post
func InvalidateCommentsCache(postID uint) {
	prefix := fmt.Sprintf("post:%d:comments:page:", postID)
	iter := config.RedisClient.Scan(config.RedisCtx, 0, prefix+"*", 0).Iterator()
	for iter.Next(config.RedisCtx) {
		config.RedisClient.Del(config.RedisCtx, iter.Val())
	}
	if err := iter.Err(); err != nil {
		log.Printf("Error invalidating cache: %v", err)
	}
}

// Preload cache for a specific page of comments
func PreloadCommentsCache(postID uint, page int) {
	limit := 10
	offset := (page - 1) * limit
	cacheKey := fmt.Sprintf("post:%d:comments:page:%d", postID, page)

	var comments []models.Comment
	if err := config.DB.Preload("User").Where("post_id = ?", postID).
		Order("created_at desc").Limit(limit).Offset(offset).Find(&comments).Error; err == nil {
		if cacheData, err := json.Marshal(comments); err == nil {
			_ = config.SetCache(cacheKey, cacheData, 5*time.Minute)
		}
	}
}
