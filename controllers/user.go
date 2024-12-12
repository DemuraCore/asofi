package controllers

import (
	"aso/asofi/config"
	"aso/asofi/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetUsers(c *gin.Context) {
	var users []models.User
	config.DB.Find(&users)

	c.JSON(http.StatusOK, gin.H{"data": users})
}

func GetMe(c *gin.Context) {
	userID := int(c.MustGet("user_id").(float64))
	var user models.User
	if err := config.DB.Preload("Posts").Where("id = ?", userID).First(&user).Error; err != nil {
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

	// Check if user is already following the user
	var count int64
	config.DB.Table("user_follows").Where("follower_id = ? AND followed_id = ?", followerID, following.ID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You are already following this user"})
		return
	}

	if err := config.DB.Model(&follower).Association("Following").Append(&following); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error following user"})
		return
	}

	if err := UpdateFollowingCount(follower.ID, 1); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating following count"})
		return
	}

	if err := UpdateFollowersCount(following.ID, 1); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating followers count"})
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
	var count int64
	config.DB.Table("user_follows").Where("follower_id = ? AND followed_id = ?", followerID, following.ID).Count(&count)
	if count == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You are not following this user"})
		return
	}

	if err := config.DB.Model(&follower).Association("Following").Delete(&following); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error unfollowing user"})
		return
	}

	if err := UpdateFollowingCount(follower.ID, -1); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating following count"})
		return
	}

	if err := UpdateFollowersCount(following.ID, -1); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating followers count"})
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

func GetUserProfile(c *gin.Context) {
	username := c.Param("username")
	var user models.User
	if err := config.DB.Preload("Posts").Where("username = ?", username).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": user})
}

func UpdateFollowingCount(userID uint, delta int) error {
	return config.DB.Model(&models.User{}).Where("id = ?", userID).Update("following_count", gorm.Expr("following_count + ?", delta)).Error
}

func UpdateFollowersCount(userID uint, delta int) error {
	return config.DB.Model(&models.User{}).Where("id = ?", userID).Update("followers_count", gorm.Expr("followers_count + ?", delta)).Error
}
