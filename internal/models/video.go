package models

import (
	"gorm.io/gorm"
)

type Video struct {
	gorm.Model
	ID             int    `gorm:"primary_key;AUTO_INCREMENT;" json:"-"`
	VideoID        string `gorm:"uniqueIndex:video_id" json:"video_id"`
	Title          string `json:"title"`
	Keyword        string `json:"string"`
	VideoThumbnail string `json:"video_thumbnail"`
	Description    string `json:"description"`
	ChannelName    string `json:"channel_name"`
	ChannelImage   string `json:"channel_image"`
	ChannelId      string `json:"channel_id"`
}
