package controllers

import (
	"aso/asofi/config"
	"aso/asofi/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

func CreatePost(c *gin.Context) {
	var post models.Post
	if err := c.ShouldBindJSON(&post); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// check if content is empty
	if post.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Content is required"})
		return
	}

	userID := int(c.MustGet("user_id").(float64))
	post.UserID = uint(userID)
	config.DB.Create(&post)

	c.JSON(http.StatusCreated, gin.H{"data": post})
}
