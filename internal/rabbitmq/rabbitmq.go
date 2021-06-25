package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/kurosaki/l1/internal/yt"
	"github.com/streadway/amqp"
	"gorm.io/gorm"
)

var ErrorDisconnected = errors.New("rabbitmq 연결 끊어짐, 재시도...")

const (
	reconnectDely = time.Second * 5
	resendDely    = time.Second * 5
)

type Client struct {
	Queue         string
	connection    *amqp.Connection
	channel       *amqp.Channel
	done          chan os.Signal
	notifyClose   chan *amqp.Error
	notifyConfirm chan amqp.Confirmation
	isConnected   bool
	alive         bool
	threads       int
	wg            *sync.WaitGroup
}

func New(Queue, addr string, done chan os.Signal) *Client {
	// threads := runtime.GOMAXPROCS(0)
	// if numCPU := runtime.NumCPU(); numCPU > threads {
	// 	threads = threads
	// }
	threads := 1
	client := Client{
		threads: threads,
		Queue:   Queue,
		done:    done,
		alive:   true,
		wg:      &sync.WaitGroup{},
	}

	client.wg.Add(1) // threads

	go client.reconnectHandler(addr)
	return &client
}

func (c *Client) reconnectHandler(addr string) {
	for c.alive {
		c.isConnected = false
		t := time.Now()
		fmt.Printf("rabbitMQ 연결시도 : %s\n", addr)
		var retryCount int
		for !c.connect(addr) {
			if !c.alive {
				return
			}
			select {
			case <-c.done:
				return
			case <-time.After(reconnectDely + time.Duration(retryCount)*time.Second):
				log.Println("rabbitMQ 연결 끊어짐")
				retryCount++
			}
		}
		log.Printf("rabbitMQ 연결성공: %vms", time.Since(t).Microseconds())
		select {
		case <-c.done:
			return
		case <-c.notifyClose:
		}
	}
}

func (c *Client) connect(addr string) bool {
	conn, err := amqp.Dial(addr)
	if err != nil {
		log.Fatalf("rabbitMQ 연결실패: %v", err)
		return false
	}
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("rabbitMQ 채널 연결 실패: %v", err)
		return false
	}
	ch.Confirm(false)

	_, err = ch.QueueDeclare(c.Queue, false, false, false, false, nil)
	if err != nil {
		log.Fatalf("rabbitMQ PUSH 대기열 선언 실패: %v", err)
		return false
	}

	c.changeConnection(conn, ch)
	c.isConnected = true
	return true
}

func (c *Client) changeConnection(connection *amqp.Connection, channel *amqp.Channel) {
	c.connection = connection
	c.channel = channel
	c.notifyClose = make(chan *amqp.Error)
	c.notifyConfirm = make(chan amqp.Confirmation)
	c.channel.NotifyClose(c.notifyClose)
	c.channel.NotifyPublish(c.notifyConfirm)
}

func (c *Client) Push(data []byte) error {
	if !c.isConnected {
		return errors.New("PUSH 보내기 실패, 연결되지않음")
	}
	for {
		err := c.UnsafePush(data)
		if err != nil {
			if err == ErrorDisconnected {
				continue
			}
			return err
		}
		select {
		case confirm := <-c.notifyConfirm:
			if confirm.Ack {
				return nil
			}
		case <-time.After(resendDely):
		}
	}
}

func (c *Client) UnsafePush(data []byte) error {
	if !c.isConnected {
		return ErrorDisconnected
	}
	return c.channel.Publish(
		"",
		c.Queue,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        data,
		},
	)
}

func (c *Client) Stream(cancelCtx context.Context, db *gorm.DB) error {
	for {
		if c.isConnected {
			break
		}
		time.Sleep(1 * time.Second)
	}
	err := c.channel.Qos(1, 0, false)
	if err != nil {
		return err
	}
	var connectionDropped bool

	for i := 1; i <= c.threads; i++ {
		msgs, err := c.channel.Consume(
			c.Queue,
			"",
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			return err
		}
		go func() {
			defer c.wg.Done()
			for {
				select {
				case <-cancelCtx.Done():
					return
				case msg, ok := <-msgs:
					if !ok {
						connectionDropped = true
						return
					}
					msg.Ack(false)
					c.crawlerEvent(msg, db)
				}
			}
		}()
	}
	c.wg.Wait()
	if connectionDropped {
		return ErrorDisconnected
	}
	return nil
}

func (c *Client) crawlerEvent(msg amqp.Delivery, db *gorm.DB) {
	yt.Run(db, string(msg.Body))
	// fmt.Println("소비됌")
}

// func consumerName(i int) string {
// 	return fmt.Sprintf("crawler-%v", i)
// }
