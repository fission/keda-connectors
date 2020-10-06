package main

import (
	"fmt"
	"os"
        "log"
	"github.com/streadway/amqp"
)

func main() {
     var rabbit_host = os.Getenv("RABBIT_HOST")
     var rabbit_port = os.Getenv("RABBIT_PORT")
     var rabbit_user = os.Getenv("RABBIT_USERNAME")
     var rabbit_password = os.Getenv("RABBIT_PASSWORD")

     conn, err := amqp.Dial("amqp://" + rabbit_user + ":" + rabbit_password + "@" + rabbit_host + ":" + rabbit_port + "/")

     if err != nil {
	log.Fatalf("%s: %s", "Failed to connect to RabbitMQ", err)
	}

	defer conn.Close()

      ch, err := conn.Channel()

     if err != nil {
	log.Fatalf("%s: %s", "Failed to open a channel", err)
	}

	q, err := ch.QueueDeclare(
        "response-subscriber", // name
	true,   // durable
	false,   // delete when unused
	false,   // exclusive
	false,   // no-wait
        nil,     // arguments
        )
      defer ch.Close()

	// We consume data from the queue named Test using the channel we created in go.
	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)

	if err != nil {
		panic("error consuming the queue: " + err.Error())
	}

	// We loop through the messages in the queue and print them in the console.
	// The msgs will be a go channel, not an amqp channel
	for msg := range msgs {
		fmt.Println(string(msg.Body))
		msg.Ack(false)
	}
}
