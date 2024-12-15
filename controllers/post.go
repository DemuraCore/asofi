// Controller.go
package controllers

import (
	"aso/asofi/channels" // Import the shared channel
	"aso/asofi/config"
	"aso/asofi/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func DeletePost(c *gin.Context) {
	postID := c.Param("id")
	userID := int(c.MustGet("user_id").(float64))

	var post models.Post
	if err := config.DB.Preload("User").Where("id = ? AND user_id = ?", postID, userID).First(&post).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found or you are not authorized to delete this post"})
		return
	}

	if err := config.DB.Delete(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error deleting post"})
		return
	}

	channels.FeedBroadcast <- models.Post{ID: post.ID} // Notify feed channel of deletion
	channels.PostBroadcast <- models.Post{ID: post.ID} // Notify post-specific channel

	c.JSON(http.StatusOK, gin.H{"message": "Post deleted successfully"})
}

func CreatePost(c *gin.Context) {
	var post models.Post
	if err := c.ShouldBindJSON(&post); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if post.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Content is required"})
		return
	}

	userID := int(c.MustGet("user_id").(float64))

	// Check if the user is verified
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to fetch user"})
		return
	}

	if !user.Verified {
		c.JSON(http.StatusForbidden, gin.H{"error": "User needs to verify email"})
		return
	}

	post.UserID = uint(userID)

	if err := config.DB.Create(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to create post"})
		return
	}
	config.DB.Preload("User").First(&post, post.ID)

	channels.FeedBroadcast <- post // Feed channel

	c.JSON(http.StatusCreated, gin.H{"data": post})
}

type PostIDRequest struct {
	PostID uint `json:"post_id" binding:"required"`
}

func LikePost(c *gin.Context) {
	var req PostIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	userID := int(c.MustGet("user_id").(float64))

	var post models.Post
	if err := config.DB.Preload("User").Where("id = ?", req.PostID).First(&post).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	var like models.Like
	if err := config.DB.Where("post_id = ? AND user_id = ?", req.PostID, userID).First(&like).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You have already liked this post"})
		return
	}

	like = models.Like{
		PostID: post.ID,
		UserID: uint(userID),
	}

	if err := config.DB.Create(&like).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error liking post"})
		return
	}

	post.LikeCount++
	config.DB.Save(&post)

	channels.FeedBroadcast <- post

	c.JSON(http.StatusOK, gin.H{"message": "Post liked successfully"})
}

func UnlikePost(c *gin.Context) {
	var req PostIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	userID := int(c.MustGet("user_id").(float64))

	var post models.Post
	if err := config.DB.Where("id = ?", req.PostID).Preload("User").First(&post).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	var like models.Like
	if err := config.DB.Where("post_id = ? AND user_id = ?", req.PostID, userID).First(&like).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You have not liked this post"})
		return
	}

	if err := config.DB.Delete(&like).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error unliking post"})
		return
	}

	post.LikeCount--
	config.DB.Save(&post)

	channels.FeedBroadcast <- post

	c.JSON(http.StatusOK, gin.H{"message": "Post unliked successfully"})
}

type PostData struct {
	Post    models.Post
	IsLiked bool
}

func ListPosts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var posts []models.Post
	if err := config.DB.Preload("User").Order("created_at desc").Limit(limit).Offset(offset).Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userID, exists := c.Get("user_id")
	var postDataList []PostData
	for _, post := range posts {
		postData := PostData{
			Post: post,
		}
		if exists {
			postData.IsLiked = IsPostLikedByUser(post.ID, uint(userID.(float64)))
		}
		postDataList = append(postDataList, postData)
	}

	c.JSON(http.StatusOK, gin.H{"data": postDataList})
}

func IsPostLikedByUser(postID uint, userID uint) bool {
	var like models.Like
	if err := config.DB.Where("post_id = ? AND user_id = ?", postID, userID).First(&like).Error; err != nil {
		return false
	}
	return true
}

func GetPost(c *gin.Context) {
	postID := c.Param("id")

	var post models.Post
	if err := config.DB.Preload("User").Where("id = ?", postID).First(&post).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	userID, exists := c.Get("user_id")
	var postData PostData
	postData.Post = post
	if exists {
		postData.IsLiked = IsPostLikedByUser(post.ID, uint(userID.(float64)))
	}

	c.JSON(http.StatusOK, gin.H{"data": postData})
}

type CommentRequest struct {
	Content string `json:"content" binding:"required"`
}

func CreateComment(c *gin.Context) {
	postID := c.Param("id")
	userID := int(c.MustGet("user_id").(float64))

	var post models.Post
	if err := config.DB.Where("id = ?", postID).First(&post).Error; err != nil {
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
	if err := config.DB.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to create comment"})
		return
	}

	post.CommentCount++
	config.DB.Save(&post)

	channels.PostBroadcast <- post

	c.JSON(http.StatusCreated, gin.H{"data": comment})

}

func DeleteComment(c *gin.Context) {
	commentID := c.Param("id")
	userID := int(c.MustGet("user_id").(float64))

	var comment models.Comment
	if err := config.DB.Where("id = ? AND user_id = ?", commentID, userID).First(&comment).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comment not found or you are not authorized to delete this comment"})
		return
	}

	var post models.Post
	if err := config.DB.Where("id = ?", comment.PostID).First(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to fetch post"})
		return
	}

	if err := config.DB.Delete(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error deleting comment"})
		return
	}

	post.CommentCount--
	config.DB.Save(&post)

	channels.PostBroadcast <- post

	c.JSON(http.StatusOK, gin.H{"message": "Comment deleted successfully"})
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
	if err := config.DB.Where("post_id = ?", postID).Order("created_at desc").Limit(limit).Offset(offset).Find(&comments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": comments})
}
