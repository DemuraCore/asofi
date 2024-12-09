package controllers

import (
	"aso/asofi/config"
	"aso/asofi/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetUsers(c *gin.Context) {
	var users []models.User
	config.DB.Find(&users)

	c.JSON(http.StatusOK, gin.H{"data": users})
}

func GetMe(c *gin.Context) {
	userID := int(c.MustGet("user_id").(float64))
	var user models.User
	if err := config.DB.Preload("Followers").Preload("Following").Preload("Posts").Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": user})
}

func Follow(c *gin.Context) {
	followerID := int(c.MustGet("user_id").(float64))
	followingID := c.Param("id")

	var follower, following models.User
	if err := config.DB.Where("id = ?", followerID).First(&follower).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Follower not found"})
		return
	}

	if err := config.DB.Where("id = ?", followingID).First(&following).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User to follow not found"})
		return
	}

	if followerID == int(following.ID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You cannot follow yourself"})
		return
	}

	// check if user is already following the user
	var count int64 = config.DB.Model(&follower).Association("Following").Count()

	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You are already following this user"})
		return
	}

	if err := config.DB.Model(&follower).Association("Following").Append(&following); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error following user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully followed user"})
}
func Unfollow(c *gin.Context) {
	followerID := int(c.MustGet("user_id").(float64))
	followingID := c.Param("id")

	var follower, following models.User
	if err := config.DB.Where("id = ?", followerID).First(&follower).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Follower not found"})
		return
	}

	if err := config.DB.Where("id = ?", followingID).First(&following).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User to unfollow not found"})
		return
	}

	if followerID == int(following.ID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You cannot unfollow yourself"})
		return
	}

	// Check if the user is following the other user
	var count int64 = config.DB.Model(&follower).Association("Following").Count()

	if count == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You are not following this user"})
		return
	}

	if err := config.DB.Model(&follower).Association("Following").Delete(&following); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error unfollowing user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully unfollowed user"})
}

func GetFollower(c *gin.Context) {
	userID := c.Param("id")
	var user models.User
	var followers []models.User

	if err := config.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	config.DB.Model(&user).Association("Followers").Find(&followers)

	c.JSON(http.StatusOK, gin.H{"data": followers})
}
