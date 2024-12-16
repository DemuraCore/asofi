// Controller.go
package controllers

import (
	"aso/asofi/channels" // Import the shared channel
	"aso/asofi/config"
	"aso/asofi/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
	go func() {
		channels.FeedBroadcast <- models.Post{ID: post.ID}
	}()
	c.JSON(http.StatusOK, gin.H{"message": "Post deleted successfully"})
}

func CreatePost(c *gin.Context) {
	var post models.Post

	// Validate incoming JSON
	if err := c.ShouldBindJSON(&post); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
		return
	}

	// Check content
	if post.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Content is required"})
		return
	}

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Fetch user and validate verification status
	var user models.User
	if err := config.DB.Select("id, verified").Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to fetch user"})
		return
	}

	if !user.Verified {
		c.JSON(http.StatusForbidden, gin.H{"error": "User needs to verify email"})
		return
	}

	// Assign user ID to the post
	post.UserID = uint(userID.(float64))

	// Create post in the database
	if err := config.DB.Create(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to create post"})
		return
	}

	// Fetch the created post with associated User (if necessary)
	if err := config.DB.Preload("User").First(&post, post.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching created post"})
		return
	}

	// Broadcast post creation asynchronously
	go func() {
		channels.FeedBroadcast <- post
	}()

	// Respond with the created post
	c.JSON(http.StatusCreated, gin.H{"data": post})
}

type PostIDRequest struct {
	PostID uint `json:"post_id" binding:"required"`
}

func LikePost(c *gin.Context) {
	var req PostIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondWithError(c, http.StatusBadRequest, "Invalid request")
		return
	}

	userID := uint(c.MustGet("user_id").(float64))

	// Check if the post exists
	var post models.Post
	if err := config.DB.Where("id = ?", req.PostID).First(&post).Error; err != nil {
		RespondWithError(c, http.StatusNotFound, "Post not found")
		return
	}

	// Check if the like already exists
	var likeExists bool
	if err := config.DB.Model(&models.Like{}).
		Select("count(1) > 0").
		Where("post_id = ? AND user_id = ?", req.PostID, userID).
		Find(&likeExists).Error; err == nil && likeExists {
		RespondWithError(c, http.StatusBadRequest, "You have already liked this post")
		return
	}

	// Create a like and update the like count in a transaction
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		// Create the like entry
		like := models.Like{
			PostID: req.PostID,
			UserID: userID,
		}
		if err := tx.Create(&like).Error; err != nil {
			return err
		}

		// Increment the like count
		if err := tx.Model(&models.Post{}).
			Where("id = ?", req.PostID).
			UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		RespondWithError(c, http.StatusInternalServerError, "Transaction failed")
		return
	}

	// Fetch the full post data for broadcasting
	if err := config.DB.Preload("User").Where("id = ?", req.PostID).First(&post).Error; err != nil {
		RespondWithError(c, http.StatusInternalServerError, "Failed to fetch full post data for broadcasting")
		return
	}

	// Broadcast asynchronously
	go func() {
		channels.FeedBroadcast <- post
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Post liked successfully"})
}

// Reusable error response helper
func RespondWithError(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{"error": message})
	c.Abort()
}

func UnlikePost(c *gin.Context) {
	var req PostIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondWithError(c, http.StatusBadRequest, "Invalid request")
		return
	}

	userID := uint(c.MustGet("user_id").(float64))

	// Check if the post exists
	var postExists bool
	if err := config.DB.Model(&models.Post{}).
		Select("count(1) > 0").
		Where("id = ?", req.PostID).
		Find(&postExists).Error; err != nil || !postExists {
		RespondWithError(c, http.StatusNotFound, "Post not found")
		return
	}

	// Check if the like exists
	var likeExists bool
	if err := config.DB.Model(&models.Like{}).
		Select("count(1) > 0").
		Where("post_id = ? AND user_id = ?", req.PostID, userID).
		Find(&likeExists).Error; err != nil || !likeExists {
		RespondWithError(c, http.StatusBadRequest, "You have not liked this post")
		return
	}

	// Remove like and decrement like count in a transaction
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		// Delete the like entry
		if err := tx.Where("post_id = ? AND user_id = ?", req.PostID, userID).Delete(&models.Like{}).Error; err != nil {
			return err
		}

		// Decrement the like count
		if err := tx.Model(&models.Post{}).
			Where("id = ?", req.PostID).
			UpdateColumn("like_count", gorm.Expr("like_count - 1")).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		RespondWithError(c, http.StatusInternalServerError, "Transaction failed")
		return
	}

	// Fetch full post data for broadcasting
	var post models.Post
	if err := config.DB.Preload("User").Where("id = ?", req.PostID).First(&post).Error; err != nil {
		RespondWithError(c, http.StatusInternalServerError, "Failed to fetch full post data for broadcasting")
		return
	}

	// Broadcast asynchronously
	go func() {
		channels.FeedBroadcast <- post
	}()

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

	userID, userExists := c.Get("user_id")

	// Fetch posts with pagination
	var posts []models.Post
	if err := config.DB.Preload("User").Order("created_at desc").Limit(limit).Offset(offset).Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var postDataList []PostData

	if userExists {
		userUintID := uint(userID.(float64))

		// Fetch all liked posts by the user in a single query
		var likedPostIDs []uint
		if err := config.DB.Model(&models.Like{}).
			Where("user_id = ? AND post_id IN ?", userUintID, getPostIDs(posts)).
			Pluck("post_id", &likedPostIDs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		likedPostMap := createPostIDMap(likedPostIDs)

		// Construct the post data list
		for _, post := range posts {
			postDataList = append(postDataList, PostData{
				Post:    post,
				IsLiked: likedPostMap[post.ID], // Check if post is liked using the map
			})
		}
	} else {
		// If user not logged in, just construct post data without `IsLiked`
		for _, post := range posts {
			postDataList = append(postDataList, PostData{Post: post})
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": postDataList})
}

func GetPost(c *gin.Context) {
	postID := c.Param("id")

	userID, userExists := c.Get("user_id")
	userUintID := uint(0)
	if userExists {
		userUintID = uint(userID.(float64))
	}

	// Query post and include `is_liked` in the result if the user exists
	var result struct {
		models.Post
		IsLiked bool `gorm:"column:is_liked"`
	}

	query := config.DB.Table("posts").
		Select("posts.*, EXISTS(SELECT 1 FROM likes WHERE likes.post_id = posts.id AND likes.user_id = ?) AS is_liked", userUintID).
		Where("posts.id = ?", postID).
		Preload("User")

	if err := query.First(&result).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	// Construct PostData response
	postData := PostData{
		Post:    result.Post,
		IsLiked: result.IsLiked,
	}

	c.JSON(http.StatusOK, gin.H{"data": postData})
}

func UpdatePost(c *gin.Context) {
	postID := c.Param("id")
	userID := int(c.MustGet("user_id").(float64))

	var post models.Post
	if err := config.DB.Preload("User").Where("id = ? AND user_id = ?", postID, userID).First(&post).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found or you are not authorized to update this post"})
		return
	}

	var req models.Post
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Content is required"})
		return
	}

	if err := config.DB.Model(&post).Updates(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating post"})
		return
	}

	go func() {
		channels.FeedBroadcast <- post
	}()

	c.JSON(http.StatusOK, gin.H{"data": post})
}

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

func getPostIDs(posts []models.Post) []uint {
	postIDs := make([]uint, len(posts))
	for i, post := range posts {
		postIDs[i] = post.ID
	}
	return postIDs
}

func createPostIDMap(postIDs []uint) map[uint]bool {
	postMap := make(map[uint]bool, len(postIDs))
	for _, id := range postIDs {
		postMap[id] = true
	}
	return postMap
}
