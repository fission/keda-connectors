package main

import (
	"fmt"
	"log"
        "os"
	"github.com/streadway/amqp"
)

var rabbit_host = os.Getenv("RABBIT_HOST")
var rabbit_port = os.Getenv("RABBIT_PORT")
var rabbit_user = os.Getenv("RABBIT_USERNAME")
var rabbit_password = os.Getenv("RABBIT_PASSWORD")

func main() {

	submit()
}

func submit() {

	results := []byte{'!', '!', 'A', 'B', 'C', 'D', 'E', 'F',
		'G', 'H', 'I', 'J', 'K', 'L', 'M', '2', '3', '#', '#'}

	for _, v := range results {

		conn, err := amqp.Dial("amqp://" + rabbit_user + ":" + rabbit_password + "@" + rabbit_host + ":" + rabbit_port + "/")

		if err != nil {
			log.Fatalf("%s: %s", "Failed to connect to RabbitMQ", err)
		}

		defer conn.Close()

		ch, err := conn.Channel()

		if err != nil {
			log.Fatalf("%s: %s", "Failed to open a channel", err)
		}

		defer ch.Close()

		q, err := ch.QueueDeclare(
			"publisher", // name
			true,        // durable
			false,       // delete when unused
			false,       // exclusive
			false,       // no-wait
			nil,         // arguments
		)

		if err != nil {
			log.Fatalf("%s: %s", "Failed to declare a queue", err)
		}

		fmt.Println(string(v))
		err = ch.Publish(
			"",     // exchange
			q.Name, // routing key
			false,  // mandatory
			false,  // immediate

			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(string(v)),
			})

		if err != nil {
			log.Fatalf("%s: %s", "Failed to publish a message in a queue", err)
		}

		fmt.Println("Publishing a message is  successful!")
	}
}

