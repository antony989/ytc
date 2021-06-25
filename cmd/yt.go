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
	var goChan chan os.Signal = make(chan os.Signal, 1)
	var client *rabbitmq.Client = rabbitmq.New("crawler", amqpServerURL, goChan)

	err := client.Stream(context.Background(), db)
	if err != nil {
		fmt.Println(err)
	}

	// wg := &sync.WaitGroup{}
	// amqpServerURL := os.Getenv("AMQP_SERVER_URL")
	// conn, err := amqp.Dial(amqpServerURL)
	// yt.HandlerError(err)
	// defer conn.Close()
	// ch, err := conn.Channel()
	// yt.HandlerError(err)
	// defer ch.Close()
	// q, err := ch.QueueDeclare("crawler", false, false, false, false, nil)
	// yt.HandlerError(err)
	// messages, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	// yt.HandlerError(err)

	// forever := make(chan bool)
	// wg.Add(1)
	// go func() {
	// 	for d := range messages {
	// 		url := d.Body
	// 		log.Printf("[*] Recieved Message: %s\n", d.Body)
	// 		go yt.Run(db, string(url), wg)
	// 		d.Ack(false)
	// 	}
	// }()
	// wg.Wait()
	// log.Printf("[*] Waiting for messages!!")
	// <-forever

}

// func run(db *gorm.DB, url string, wg *sync.WaitGroup) {
// 	// opts := append(chromedp.DefaultExecAllocatorOptions[:]) // chromedp.DisableGPU,
// 	// chromedp.Flag("headless", true),

// 	// opts := append(chromedp.DefaultExecAllocatorOptions[:],
// 	// 	chromedp.DisableGPU,
// 	// 	chromedp.Flag("headless", true),
// 	// 	chromedp.
// 	// )

// 	ctx1, cancel1 := chromedp.NewRemoteAllocator(context.Background(), "http://chromedp:9222")
// 	defer cancel1()
// 	ctx, cancel := chromedp.NewContext(ctx1)
// 	defer cancel()

// 	videoIdRegex := regexp.MustCompile(`https.+?\.?v=([^\s"\'<>?&]+)`)
// 	videoId := videoIdRegex.FindStringSubmatch(url)[1]

// 	chromedp.ListenTarget(ctx, func(event interface{}) {
// 		switch ev := event.(type) {
// 		case *network.EventResponseReceived:
// 			if ev.Type != "XHR" {
// 				return
// 			}
// 			match, _ := regexp.MatchString(`youtube.com\/comment\_service\_ajax`, ev.Response.URL)
// 			if match {
// 				go func() {
// 					comment := yt.GetComment(ctx, chromedp.FromContext(ctx), ev, videoId)
// 					for idx := range comment {
// 						db.Create(&comment[idx])
// 					}
// 				}()
// 			}
// 		}
// 	})

// 	err := chromedp.Run(
// 		ctx,
// 		network.Enable(),
// 		chromedp.Navigate(url),
// 		chromedp.Sleep(time.Second*5),
// 		chromedp.ActionFunc(func(ctx context.Context) error {
// 			videoInfo := yt.GetVideoInfo(ctx, videoId)
// 			db.Create(&videoInfo)
// 			return nil
// 		}),
// 	)

// 	if err != nil {
// 		log.Println(err)
// 	}

// 	height, _ := yt.GetScrollHeight(ctx)
// 	var currentHeight int

// 	for {
// 		err = chromedp.Run(
// 			ctx,
// 			yt.Tasks(url),
// 			chromedp.ActionFunc(func(c context.Context) error {
// 				_, exp, err := runtime.Evaluate(`window.scrollTo(0, document.documentElement.scrollHeight)`).Do(c)
// 				if err != nil {
// 					return err
// 				}
// 				if exp != nil {
// 					return exp
// 				}
// 				return nil
// 			}),
// 			chromedp.Sleep(time.Second*4),
// 			chromedp.Evaluate(`document.documentElement.scrollHeight`, &currentHeight),
// 		)
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		if height == currentHeight {
// 			wg.Done()
// 			break
// 		}
// 		height = currentHeight
// 	}
// }
