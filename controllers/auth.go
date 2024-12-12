package controllers

import (
	"aso/asofi/config"
	"aso/asofi/models"
	"aso/asofi/utils"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type RegisterRequest struct {
	Email    string `json:"email" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func Register(c *gin.Context) {
	var input RegisterRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  400,
			"error": err.Error()})
		return
	}

	var user models.User

	if err := config.DB.Where("email = ?", input.Email).First(&user).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  400,
			"error": "Email already registered"})
		return
	}

	if err := config.DB.Where("username = ?", input.Username).First(&user).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  400,
			"error": "Username already taken"})
		return
	}

	hashedPassword, err := utils.HashPassword(input.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  500,
			"error": "Error hashing password"})
		return
	}

	user.Password = hashedPassword
	user.Email = input.Email
	user.Username = input.Username
	user.Name = input.Name
	config.DB.Create(&user)

	token, err := utils.GenerateToken(int(user.ID), user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  500,
			"error": "Error generating token"})
		return
	}

	session := models.Session{
		UserID: user.ID,
		Token:  token,
	}
	config.DB.Create(&session)

	c.JSON(http.StatusCreated, gin.H{"code": 200, "data": user, "token": token})
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func Login(c *gin.Context) {
	var input LoginRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": err.Error()})
		return
	}

	var user models.User
	config.DB.Where("email = ?", input.Email).First(&user)
	if user.ID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "error": "User not found"})
		return
	}

	if err := utils.VerifyPassword(user.Password, input.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": "Invalid credentials"})
		return
	}

	token, err := utils.GenerateToken(int(user.ID), user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "error": "Error generating token"})
		return
	}
	// session add
	session := models.Session{
		UserID: user.ID,
		Token:  token,
	}

	config.DB.Create(&session)

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": token})
}

// In Auth Service's controllers/auth.go
func ValidateToken(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": "Missing token"})
		return
	}

	claims, err := utils.ValidateToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "error": "Invalid token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id": claims["user_id"],
		"exp":     claims["exp"],
	})
}

func Logout(c *gin.Context) {
	// delete session
	token := c.GetHeader("Authorization")
	config.DB.Where("token = ?", token).Delete(&models.Session{})

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": "Successfully logged out"})
}

type VerifyOTPRequest struct {
	Code string `json:"code" binding:"required"`
}

func SendCODE(c *gin.Context) {
	// Extract user ID from the token
	userID := int(c.MustGet("user_id").(float64))

	// Fetch the user's email from the database
	var user models.User
	if err := config.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.Verified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User already verified"})
		return
	}

	// Generate OTP
	code, err := utils.GenerateOTP(config.DB, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating OTP"})
		return
	}

	// Send OTP to the user's email
	if err := utils.SendOTP(user.Email, code); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error sending OTP"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OTP sent"})
}

func VerifyCODE(c *gin.Context) {
	var input VerifyOTPRequest

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := int(c.MustGet("user_id").(float64))

	// Fetch the user's email from the database
	var user models.User
	if err := config.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if user.Verified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User already verified"})
		return
	}

	var otp models.OTP
	if err := config.DB.Where("user_id = ? AND code = ?", user.ID, input.Code).First(&otp).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid OTP"})
		return
	}

	if time.Now().After(otp.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "OTP expired"})
		return
	}

	user.Verified = true
	config.DB.Save(&user)
	config.DB.Delete(&otp)

	c.JSON(http.StatusOK, gin.H{"message": "User verified"})
}
