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
	followerID := uint(c.MustGet("user_id").(float64)) // Current user ID
	followingID, err := strconv.Atoi(c.Param("id"))    // User to follow
	if err != nil {
		RespondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	if followerID == uint(followingID) {
		RespondWithError(c, http.StatusBadRequest, "You cannot follow yourself")
		return
	}

	// Check if the user to follow exists
	var followingExists bool
	if err := config.DB.Model(&models.User{}).
		Select("count(1) > 0").
		Where("id = ?", followingID).
		Find(&followingExists).Error; err != nil || !followingExists {
		RespondWithError(c, http.StatusNotFound, "User to follow not found")
		return
	}

	// Check if already following
	var alreadyFollowing bool
	if err := config.DB.Model(&models.UserFollow{}).
		Select("count(1) > 0").
		Where("follower_id = ? AND followed_id = ?", followerID, followingID).
		Find(&alreadyFollowing).Error; err == nil && alreadyFollowing {
		RespondWithError(c, http.StatusBadRequest, "You are already following this user")
		return
	}

	// Create follow relationship in a transaction
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		// Insert follow relationship
		follow := models.UserFollow{
			FollowerID: followerID,
			FollowedID: uint(followingID),
		}
		if err := tx.Create(&follow).Error; err != nil {
			return err
		}

		// Update counts atomically
		if err := tx.Model(&models.User{}).
			Where("id = ?", followerID).
			UpdateColumn("following_count", gorm.Expr("following_count + 1")).Error; err != nil {
			return err
		}

		if err := tx.Model(&models.User{}).
			Where("id = ?", followingID).
			UpdateColumn("followers_count", gorm.Expr("followers_count + 1")).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		RespondWithError(c, http.StatusInternalServerError, "Error following user")
		return
	}

	// Broadcast to both users
	go func() {
		var follower, followed models.User
		config.DB.First(&follower, followerID)
		config.DB.First(&followed, followingID)
		channels.ProfileBroadcast <- follower
		channels.ProfileBroadcast <- followed
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Successfully followed user"})
}

type ProfileData struct {
	Profile models.User
}

func RespondWithError(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{"error": message})
}

func Unfollow(c *gin.Context) {
	followerID := uint(c.MustGet("user_id").(float64)) // Current user ID
	followingID, err := strconv.Atoi(c.Param("id"))    // User to unfollow
	if err != nil {
		RespondWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	if followerID == uint(followingID) {
		RespondWithError(c, http.StatusBadRequest, "You cannot unfollow yourself")
		return
	}

	// Check if the user to unfollow exists
	var followingExists bool
	if err := config.DB.Model(&models.User{}).
		Select("count(1) > 0").
		Where("id = ?", followingID).
		Find(&followingExists).Error; err != nil || !followingExists {
		RespondWithError(c, http.StatusNotFound, "User to unfollow not found")
		return
	}

	// Check if the user is currently following
	var isFollowing bool
	if err := config.DB.Model(&models.UserFollow{}).
		Select("count(1) > 0").
		Where("follower_id = ? AND followed_id = ?", followerID, followingID).
		Find(&isFollowing).Error; err != nil || !isFollowing {
		RespondWithError(c, http.StatusBadRequest, "You are not following this user")
		return
	}

	// Perform unfollow operation in a transaction
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		// Delete the follow relationship
		if err := tx.Where("follower_id = ? AND followed_id = ?", followerID, followingID).
			Delete(&models.UserFollow{}).Error; err != nil {
			return err
		}

		// Decrement following count
		if err := tx.Model(&models.User{}).
			Where("id = ?", followerID).
			UpdateColumn("following_count", gorm.Expr("following_count - 1")).Error; err != nil {
			return err
		}

		// Decrement followers count
		if err := tx.Model(&models.User{}).
			Where("id = ?", followingID).
			UpdateColumn("followers_count", gorm.Expr("followers_count - 1")).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		RespondWithError(c, http.StatusInternalServerError, "Error unfollowing user")
		return
	}

	go func() {
		var follower, followed models.User
		config.DB.First(&follower, followerID)
		config.DB.First(&followed, followingID)
		channels.ProfileBroadcast <- follower
		channels.ProfileBroadcast <- followed
	}()

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
	requesterID, loggedIn := c.Get("user_id")

	var user models.User
	if err := config.DB.Where("username = ?", username).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var isFollow bool
	if loggedIn {
		// Check if the requester follows the user
		err := config.DB.Model(&models.UserFollow{}).
			Select("count(1) > 0").
			Where("follower_id = ? AND followed_id = ?", uint(requesterID.(float64)), user.ID).
			Find(&isFollow).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error checking follow status"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"Profile":  user,
			"isFollow": isFollow,
		},
	})
}
