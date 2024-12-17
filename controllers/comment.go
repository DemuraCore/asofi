package controllers

import (
	"aso/asofi/channels"
	"aso/asofi/config"
	"aso/asofi/models"
	"net/http"
	"strconv"

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

	comment := models.Comment{
		Content: req.Content,
		UserID:  uint(userID),
		PostID:  post.ID,
	}

	err := config.DB.Transaction(func(tx *gorm.DB) error {

		if err := tx.Create(&comment).Error; err != nil {
			return err
		}

		if err := tx.Preload("User").First(&comment, comment.ID).Error; err != nil {
			return err
		}

		post.CommentCount++
		if err := tx.Save(&post).Error; err != nil {
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
		channels.CommentBroadcast <- comment
	}()

	c.JSON(http.StatusCreated, gin.H{"data": comment})

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

		post.CommentCount--
		if err := tx.Save(&post).Error; err != nil {
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

	go func() {
		channels.CommentBroadcast <- comment
	}()

	c.JSON(http.StatusOK, gin.H{"data": comment})
}

func GetComments(c *gin.Context) {
	postID := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var post models.Post
	if err := config.DB.Where("id = ?", postID).First(&post).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	var comments []models.Comment
	if err := config.DB.Preload("User").Where("post_id = ?", postID).Order("created_at desc").Limit(limit).Offset(offset).Find(&comments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": comments})
}
