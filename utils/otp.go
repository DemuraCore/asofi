package utils

import (
	"aso/asofi/models"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"time"

	"gorm.io/gorm"
)

func GenerateOTP(db *gorm.DB, userID uint) (string, error) {
	otp := make([]byte, 10)
	_, err := rand.Read(otp)
	if err != nil {
		return "", err
	}

	code := base32.StdEncoding.EncodeToString(otp)
	expiresAt := time.Now().Add(10 * time.Minute)

	otpRecord := models.OTP{
		UserID:    userID,
		Code:      code,
		ExpiresAt: expiresAt,
	}

	if err := db.Create(&otpRecord).Error; err != nil {
		return "", err
	}

	return code, nil
}

func SendOTP(email, code string) error {
	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	from := "ASO <noreply@vahry.my.id>"
	to := email
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")

	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Your OTP Code\r\n\r\nYour OTP code is: %s", from, to, code)

	auth := smtp.PlainAuth("", username, password, smtpHost)
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{to}, []byte(message))
	if err != nil {
		log.Printf("Failed to send OTP email to %s: %v", email, err)
		return err
	}
	log.Printf("OTP email sent to %s successfully", email)
	return nil
}
