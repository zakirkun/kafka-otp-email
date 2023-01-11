package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/joho/godotenv"
	mail "github.com/xhit/go-simple-mail/v2"
)

var wg sync.WaitGroup

func main() {

	// load env
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	kafkaHost := os.Getenv("KafkaHost")
	group := os.Getenv("GROUP")
	topic := os.Getenv("TOPIC")

	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":             kafkaHost,
		"group.id":                      group,
		"enable.auto.commit":            false,
		"partition.assignment.strategy": "cooperative-sticky", // c.IncrementalAssign() must be implemented
		//"go.application.rebalance.enable": true, // c.Assign() c.Unassign() must be implemented
		"auto.offset.reset": "latest",
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	serviceCh := make(chan struct{})

	err = consumer.SubscribeTopics([]string{topic}, rebalanceCallback)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	wg.Add(1)

	go func() {
	loopConsume:
		for {
			select {
			case <-serviceCh:
				break loopConsume
			default:
				ev := consumer.Poll(0)
				switch e := ev.(type) {
				case *kafka.Message:
					wg.Add(1)

					_, err := consumer.CommitMessage(e)
					if err == nil {
						SendEmail(e.Value)
					}
					wg.Done()

				case kafka.Error:
					fmt.Fprintf(os.Stderr, "%% Error: %v\n", e)
					break loopConsume
				}
			}
		}

		err = consumer.Close()
		if err != nil {
			fmt.Println(err)
		}

		fmt.Println("Consumer Closed!")
		wg.Done()

		return
	}()

	<-stopSignal
	close(serviceCh)
	wg.Wait()

	fmt.Println("Shutdown gracefully!")
}

func rebalanceCallback(c *kafka.Consumer, event kafka.Event) error {

	switch ev := event.(type) {
	case kafka.AssignedPartitions:
		if len(ev.Partitions) > 0 {
			// err := c.Assign(ev.Partitions)
			err := c.IncrementalAssign(ev.Partitions)
			if err != nil {
				fmt.Println(err)
			}

			fmt.Println("--ASSIGNED---")
			for _, v := range ev.Partitions {
				fmt.Printf("Partition %v \n", v.Partition)
			}
			fmt.Println("-------------")
		}
	case kafka.RevokedPartitions:
		// err := c.Unassign()
		err := c.IncrementalUnassign(ev.Partitions)
		if err != nil {
			fmt.Println(err)
		}

		fmt.Println("\n---REVOKED---")
		for _, v := range ev.Partitions {
			fmt.Printf("Partition %v \n", v.Partition)
		}
		fmt.Println("-------------")
	}

	return nil
}

func SendEmail(msg []byte) error {

	type PayloadOtp struct {
		Email string `json:"email:"`
		Otp   string `json:"otp"`
	}

	// decode messages
	r := new(PayloadOtp)
	if err := json.Unmarshal(msg, &r); err != nil {
		log.Fatal("Error : " + err.Error())
	}

	// create teamplate messages
	htmlBody := fmt.Sprintf("Dear %s, Your OTP CODE <b>%s</b>", r.Email, r.Otp)

	// sending email
	smtp := mail.NewSMTPClient()

	// setup smtp
	smtp.Host = "smtp.hostinger.com"
	smtp.Port = 465
	smtp.Username = "zakir@bekasidev.org"
	smtp.Password = "Bekasidev-2022"
	smtp.Encryption = mail.EncryptionSTARTTLS

	// Variable to keep alive connection
	smtp.KeepAlive = true

	// Timeout for connect to SMTP Server
	smtp.ConnectTimeout = 10 * time.Second

	// Timeout for send the data and wait respond
	smtp.SendTimeout = 10 * time.Second

	// test connection
	smtpClient, err := smtp.Connect()
	if err != nil {
		log.Fatal("Failed Connect SMTP")
		log.Fatal(err)
	}

	// New email simple html with inline and CC
	email := mail.NewMSG()
	email.SetFrom("From BekasiDev <zakir@bekasidev.org>").
		AddTo(r.Email).
		SetSubject("Your OTP")

	email.SetBody(mail.TextHTML, htmlBody)

	// always check error after send
	if email.Error != nil {
		log.Fatal("Failed Sending Email")
		log.Fatal(email.Error)
	}

	err = email.Send(smtpClient)
	if err != nil {
		log.Println(err)
	} else {
		log.Println("Email Sent to " + r.Email)
	}

	fmt.Println("-> Processed!")

	return nil
}
