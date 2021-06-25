package models

import (
	"time"

	"gorm.io/gorm"
)

type MainComment struct {
	gorm.Model
	ID        int    `gorm:"primary_key;AUTO_INCREMENT;" json:"-"`
	VideoId   string `json:"video_id"`
	ChannelId string `json:"channel_id"`
	CommentId string `json:"comment_id"`
	UserName  string `json:"username"`
	Content   string `json:"content"`
	Thumbnail string `json:"thumbnail"`
	CreatedAt time.Time
	UpdatedAt time.Time
	VoteCount int64            `json:"vote_count"`
	Replies   []RepliesComment `gorm:"foreignKey:ReplyCommentId"`
}

type RepliesComment struct {
	gorm.Model
	ID             int    `gorm:"primary_key;AUTO_INCREMENT;" json:"-"`
	VideoId        string `json:"video_id"`
	ChannelId      string `json:"channel_id"`
	ReplyCommentId string `json:"reply_comment_id"`
	UserName       string `json:"username"`
	Content        string `json:"content"`
	Thumbnail      string `json:"thumbnail"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	VoteCount      int64 `json:"vote_count"`
}
