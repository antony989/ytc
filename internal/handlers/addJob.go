package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/kurosaki/l1/internal/rabbitmq"
	"github.com/labstack/echo/v4"
)

type Jobmodel struct {
	URL string `json:"url"`
}

var goChan chan os.Signal = make(chan os.Signal, 1)

var amqpServerURL string = os.Getenv("AMQP_SERVER_URL")

// var amqpServerURL string = "amqp://guest:guest@localhost:5672/"
var client *rabbitmq.Client = rabbitmq.New("crawler", amqpServerURL, goChan)

func ResponseJob(c echo.Context) error {
	var model Jobmodel
	c.Bind(&model)
	uri := model.URL
	u, err := url.ParseRequestURI(uri)

	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"code":    "fail",
			"message": fmt.Sprintf("잘못된 URL 요청 입니다."),
		})
	} else {
		err := client.Push([]byte(uri))
		if err != nil {
			fmt.Println(err)
		}

		// conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
		// yt.HandlerError(err)
		// ch, err := conn.Channel()
		// yt.HandlerError(err)

		return c.JSON(http.StatusOK, map[string]string{
			"code":    "success",
			"message": fmt.Sprintf("%v 크롤링 작업 추가 완료", u),
		})
	}
}
