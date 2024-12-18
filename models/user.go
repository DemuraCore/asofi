package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	ID             uint      `gorm:"primaryKey"`
	Username       string    `gorm:"unique;null"`
	Name           string    `gorm:"not null"`
	Email          string    `gorm:"unique;not null" json:"-"`
	Password       string    `gorm:"not null" json:"-"`
	Bio            string    `gorm:"default:Hello, I'm using Aso!"`
	IsPrivate      bool      `gorm:"default:false"`
	Avatar         string    `gorm:"default:default.jpg"`
	Verified       bool      `gorm:"default:false"`
	Posts          []Post    `gorm:"foreignKey:UserID"`
	Likes          []Like    `gorm:"foreignKey:UserID"`
	Comments       []Comment `gorm:"foreignKey:UserID"`
	Followers      []User    `gorm:"many2many:user_follows;joinForeignKey:FollowerID;joinReferences:FollowedID"`
	Following      []User    `gorm:"many2many:user_follows;joinForeignKey:FollowedID;joinReferences:FollowerID"`
	FollowingCount int       `gorm:"default:0"`
	FollowersCount int       `gorm:"default:0"`
	Session        []Session `gorm:"foreignKey:UserID"`
	RoleID         uint      `gorm:"default:1"`
	Role           Role
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type UserFollow struct {
	ID         uint `gorm:"primaryKey"`
	FollowerID uint `gorm:"not null;uniqueIndex:idx_follower_followed"`
	FollowedID uint `gorm:"not null;uniqueIndex:idx_follower_followed"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
type Post struct {
	gorm.Model
	ID           uint   `gorm:"primaryKey"`
	Content      string `gorm:"not null"`
	UserID       uint   `gorm:"not null"`
	User         User
	Like         []Like    `gorm:"foreignKey:PostID"`
	Comment      []Comment `gorm:"foreignKey:PostID"`
	ReportCount  int       `gorm:"default:0"`
	LikeCount    int       `gorm:"default:0"`
	CommentCount int       `gorm:"default:0"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Like struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint `gorm:"not null;uniqueIndex:idx_user_post"`
	PostID    uint `gorm:"not null;uniqueIndex:idx_user_post"`
	Post      Post `gorm:"foreignKey:PostID"`
	User      User `gorm:"foreignKey:UserID"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Comment struct {
	gorm.Model
	ID        uint   `gorm:"primaryKey"`
	Content   string `gorm:"not null"`
	UserID    uint   `gorm:"not null"`
	PostID    uint   `gorm:"not null"`
	User      User   `gorm:"foreignKey:UserID"`
	Post      Post
	CreatedAt time.Time
	UpdatedAt time.Time
}
type ReportedType string

const (
	ReportedTypePost    ReportedType = "post"
	ReportedTypeUser    ReportedType = "user"
	ReportedTypeComment ReportedType = "comment"
)

type Report struct {
	gorm.Model
	ID         uint   `gorm:"primaryKey"`
	Content    string `gorm:"not null"`
	UserID     uint   `gorm:"not null"`
	ReportedID uint   `gorm:"not null"`
	Type       ReportedType
	User       User
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Role struct {
	gorm.Model
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"not null"`
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
