package controllers

import (
	"aso/asofi/channels"
	"aso/asofi/config"
	"aso/asofi/models"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"gorm.io/gorm"
)

func GetMe(c *gin.Context) {
	userID := int(c.MustGet("user_id").(float64))
	cacheKey := fmt.Sprintf("user:%d", userID)

	// Attempt to retrieve cached user data
	if cachedUser, err := config.GetCache(cacheKey); err == nil && cachedUser != "" {
		var user models.User
		if err := json.Unmarshal([]byte(cachedUser), &user); err == nil {
			c.JSON(http.StatusOK, gin.H{"data": user})
			return
		}
	}

	var user models.User
	if err := config.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Cache the user data
	if cacheData, err := json.Marshal(user); err == nil {
		_ = config.SetCache(cacheKey, cacheData, 0)
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

	// First check if target user exists and get their username
	var targetUser models.User
	if err := config.DB.Where("id = ?", followingID).First(&targetUser).Error; err != nil {
		RespondWithError(c, http.StatusNotFound, "User to follow not found")
		return
	}

	// Check follow status from cache first
	cacheKey := fmt.Sprintf("user_profile:%s", targetUser.Username)
	if cachedData, err := config.GetCache(cacheKey); err == nil && cachedData != "" {
		var cached CachedProfileData
		if err := json.Unmarshal([]byte(cachedData), &cached); err == nil {
			if cached.IsFollow[followerID] {
				RespondWithError(c, http.StatusBadRequest, "You are already following this user")
				return
			}
		}
	}

	// Create follow relationship in a transaction
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		// Double check in DB to be safe
		var alreadyFollowing bool
		if err := tx.Model(&models.UserFollow{}).
			Select("count(1) > 0").
			Where("follower_id = ? AND followed_id = ?", followerID, followingID).
			Find(&alreadyFollowing).Error; err == nil && alreadyFollowing {
			return fmt.Errorf("already following")
		}
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

		// Update follower cache optimistically
		updateProfileCache(follower.Username, func(cache *CachedProfileData) {
			cache.Profile.FollowingCount++
			cache.Version++
		})

		// Update followed cache optimistically
		updateProfileCache(followed.Username, func(cache *CachedProfileData) {
			cache.Profile.FollowersCount++
			cache.Version++
			cache.IsFollow[followerID] = true
		})

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

	// Get target user first
	var targetUser models.User
	if err := config.DB.Where("id = ?", followingID).First(&targetUser).Error; err != nil {
		RespondWithError(c, http.StatusNotFound, "User to unfollow not found")
		return
	}
	// Check follow status from cache first
	cacheKey := fmt.Sprintf("user_profile:%s", targetUser.Username)
	if cachedData, err := config.GetCache(cacheKey); err == nil && cachedData != "" {
		var cached CachedProfileData
		if err := json.Unmarshal([]byte(cachedData), &cached); err == nil {
			if !cached.IsFollow[followerID] {
				RespondWithError(c, http.StatusBadRequest, "You are not following this user")
				return
			}
		}
	}

	err = config.DB.Transaction(func(tx *gorm.DB) error {
		// Double check in DB to be safe
		var isFollowing bool
		if err := tx.Model(&models.UserFollow{}).
			Select("count(1) > 0").
			Where("follower_id = ? AND followed_id = ?", followerID, followingID).
			Find(&isFollowing).Error; err != nil || !isFollowing {
			return fmt.Errorf("not following")
		}
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

		// Update follower cache optimistically
		updateProfileCache(follower.Username, func(cache *CachedProfileData) {
			cache.Profile.FollowingCount--
			cache.Version++
		})

		// Update followed cache optimistically
		updateProfileCache(followed.Username, func(cache *CachedProfileData) {
			cache.Profile.FollowersCount--
			cache.Version++
			cache.IsFollow[followerID] = false
		})

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

	cacheKey := fmt.Sprintf("user_profile:%s", username)
	var user models.User
	// delete cache for debugging
	// _ = config.RedisClient.Del(config.RedisCtx, cacheKey)

	// Attempt to retrieve cached user profile
	if cachedData, err := config.GetCache(cacheKey); err == nil && cachedData != "" {
		var cached CachedProfileData
		if err := json.Unmarshal([]byte(cachedData), &cached); err == nil {
			isFollow := false
			if loggedIn {
				// Get follow status from cache
				isFollow = cached.IsFollow[uint(requesterID.(float64))]
			}

			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"Profile":  cached.Profile,
					"isFollow": isFollow,
				},
			})
			return
		}
	}

	// Fetch user profile from the database
	if err := config.DB.Where("username = ?", username).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Create new cache entry
	cacheData := CachedProfileData{
		Profile:  user,
		IsFollow: make(map[uint]bool),
	}

	// Check follow status and update cache if logged in
	var isFollow bool
	if loggedIn {
		err := config.DB.Model(&models.UserFollow{}).
			Select("count(1) > 0").
			Where("follower_id = ? AND followed_id = ?", uint(requesterID.(float64)), user.ID).
			Find(&isFollow).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error checking follow status"})
			return
		}
		cacheData.IsFollow[uint(requesterID.(float64))] = isFollow
	}

	// Cache the complete profile data
	if jsonData, err := json.Marshal(cacheData); err == nil {
		_ = config.SetCache(cacheKey, jsonData, 0)
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"Profile":  user,
			"isFollow": isFollow,
		},
	})
}
func GetUsers(c *gin.Context) {
	var users []models.User
	config.DB.Find(&users)

	c.JSON(http.StatusOK, gin.H{"data": users})
}

// Create a struct to hold both user and follow status
type CachedProfileData struct {
	Profile  models.User
	IsFollow map[uint]bool
	Version  int64 // Add version for optimistic locking
}

func PreloadProfileCache(userData models.User) {
	cacheKey := fmt.Sprintf("user_profile:%s", userData.Username)

	// Create cache data structure
	cacheData := CachedProfileData{
		Profile:  userData,
		IsFollow: make(map[uint]bool),
	}

	// Get all followers for this user
	var follows []models.UserFollow
	if err := config.DB.Where("followed_id = ?", userData.ID).Find(&follows).Error; err == nil {
		// Build map of followerID -> true
		for _, follow := range follows {
			cacheData.IsFollow[follow.FollowerID] = true
		}
	}

	// Cache the complete profile data
	if jsonData, err := json.Marshal(cacheData); err == nil {
		_ = config.SetCache(cacheKey, jsonData, 0)
	}
}

func updateProfileCache(username string, updateFn func(*CachedProfileData)) {
	cacheKey := fmt.Sprintf("user_profile:%s", username)

	// Get existing cache
	cachedData, err := config.GetCache(cacheKey)
	if err != nil {
		// If no cache exists, preload it
		var user models.User
		if err := config.DB.Where("username = ?", username).First(&user).Error; err == nil {
			PreloadProfileCache(user)
		}
		return
	}

	var cached CachedProfileData
	if err := json.Unmarshal([]byte(cachedData), &cached); err != nil {
		return
	}

	// Apply update
	updateFn(&cached)

	// Write back to cache
	if jsonData, err := json.Marshal(cached); err == nil {
		_ = config.SetCache(cacheKey, jsonData, 0)
	}
}

type UpdateProfileInput struct {
	Name     string                `form:"name" json:"name" binding:"required"`
	Username string                `form:"username" json:"username" binding:"required"`
	Bio      string                `form:"bio" json:"bio" binding:"required"`
	Avatar   *multipart.FileHeader `form:"avatar" json:"avatar"`
}

func EditProfile(c *gin.Context) {
	userID := int(c.MustGet("user_id").(float64))
	cacheKey := fmt.Sprintf("user:%d", userID)
	var user models.User
	if err := config.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var input UpdateProfileInput
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Handle avatar upload
	file, header, err := c.Request.FormFile("avatar")
	if err == nil {
		defer file.Close()

		// Generate a unique file name
		fileName := fmt.Sprintf("%d_%s", userID, filepath.Base(header.Filename))

		// Upload the file to Minio
		_, err = config.MinioClient.PutObject(context.Background(), "aso", fileName, file, header.Size, minio.PutObjectOptions{

			ContentType: header.Header.Get("Content-Type"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err})
			return
		}

		// Set the avatar URL
		user.Avatar = fmt.Sprintf("https://%s/%s/%s", os.Getenv("MINIO_ENDPOINT"), "aso", fileName)
	}

	user.Name = input.Name
	user.Username = input.Username
	user.Bio = input.Bio
	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating user profile"})
		return
	}

	// Update cache
	updateProfileCache(user.Username, func(cache *CachedProfileData) {
		cache.Profile = user
		cache.Version++
	})
	InvalidateUserPostsCache(user.ID)
	config.RedisClient.Del(config.RedisCtx, cacheKey)

	go func() {
		channels.ProfileBroadcast <- user
	}()

	c.JSON(http.StatusOK, gin.H{"data": user})
}
