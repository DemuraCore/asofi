package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	ID        uint      `gorm:"primaryKey"`
	Username  string    `gorm:"unique;null"`
	Email     string    `gorm:"unique;not null"`
	Password  string    `gorm:"not null" json:"-"`
	IsPrivate bool      `gorm:"default:false"`
	Posts     []Post    `gorm:"foreignKey:UserID"`
	Likes     []Like    `gorm:"foreignKey:UserID"`
	Comments  []Comment `gorm:"foreignKey:UserID"`
	Followers []User    `gorm:"many2many:user_follows;joinForeignKey:FollowerID;joinReferences:FollowedID"`
	Following []User    `gorm:"many2many:user_follows;joinForeignKey:FollowedID;joinReferences:FollowerID"`
	Session   []Session
	CreatedAt time.Time
	UpdatedAt time.Time
}
type Post struct {
	gorm.Model
	ID        uint   `gorm:"primaryKey"`
	Content   string `gorm:"not null"`
	UserID    uint   `gorm:"not null"`
	User      User
	Like      []Like
	Comment   []Comment
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Like struct {
	gorm.Model
	ID        uint `gorm:"primaryKey"`
	UserID    uint `gorm:"not null"`
	PostID    uint `gorm:"not null"`
	Post      Post
	User      User
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Comment struct {
	gorm.Model
	ID        uint   `gorm:"primaryKey"`
	Content   string `gorm:"not null"`
	UserID    uint   `gorm:"not null"`
	PostID    uint   `gorm:"not null"`
	User      User
	Post      Post
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Session struct {
	gorm.Model
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"not null"`
	Token     string `gorm:"not null"`
	User      User
	CreatedAt time.Time
	UpdatedAt time.Time
}
