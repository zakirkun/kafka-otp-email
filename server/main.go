package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/joho/godotenv"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

func main() {

	// load env
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	kafkaHost := os.Getenv("KafkaHost")
	port := os.Getenv("PORT")
	topic := os.Getenv("TOPIC")

	srv := echo.New()

	// init kafka
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":                     kafkaHost,
		"acks":                                  -1,
		"retries":                               3,
		"retry.backoff.ms":                      100,
		"max.in.flight.requests.per.connection": 5,
		"enable.idempotence":                    true,
	})
	if err != nil {
		fmt.Printf("Failed to create producer: %s\n", err)
		os.Exit(1)
	}

	defer p.Close()

	srv.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	srv.POST("/otp", func(c echo.Context) error {

		type RequestOtp struct {
			Email string `json:"email" validate:"required,email"`
			Otp   string `json:"otp" validate:"required"`
		}

		request := new(RequestOtp)
		if err := c.Bind(&request); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		type ResponseOtp struct {
			Message string `json:"message"`
		}

		messages, err := json.Marshal(request)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		deliveryChan := make(chan kafka.Event)
		err = p.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Value:          messages,
			Key:            []byte(request.Email),
		}, deliveryChan)

		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		e := <-deliveryChan
		m := e.(*kafka.Message)

		if m.TopicPartition.Error != nil {
			c.Logger().Fatalf("Delivery failed: %v\n", m.TopicPartition.Error)
		} else {
			c.Logger().Infof("Delivered msg %s\n", messages)
		}

		close(deliveryChan)

		response := ResponseOtp{
			Message: "Success",
		}

		return c.JSON(http.StatusOK, response)
	})

	srv.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "method=${method}, uri=${uri}, status=${status}\n",
	}))

	srv.Logger.Fatal(srv.Start(":" + port))
}
