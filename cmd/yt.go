package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kurosaki/l1/internal/db"
	"github.com/kurosaki/l1/internal/models"
	"github.com/kurosaki/l1/internal/rabbitmq"
)

func main() {
	db := db.Connect()

	var VideoModel models.Video
	var CommentModel []models.MainComment
	var replies [][]models.RepliesComment
	db.AutoMigrate(&VideoModel)
	db.AutoMigrate(&CommentModel)
	db.AutoMigrate(&replies)
	amqpServerURL := os.Getenv("AMQP_SERVER_URL")
	// amqpServerURL := "amqp://guest:guest@localhost:5672/"
	var goChan chan os.Signal = make(chan os.Signal, 1)
	var client *rabbitmq.Client = rabbitmq.New("crawler", amqpServerURL, goChan)

	err := client.Stream(context.Background(), db)
	if err != nil {
		fmt.Println(err)
	}
}
