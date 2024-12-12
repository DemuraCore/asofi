package models

import (
	"time"

	"gorm.io/gorm"
)

type OTP struct {
	gorm.Model
	UserID    uint   `gorm:"not null"`
	Code      string `gorm:"not null"`
	ExpiresAt time.Time
}
