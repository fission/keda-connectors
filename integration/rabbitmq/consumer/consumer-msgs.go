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

     defer ch.Close()

// Get the connection string from the environment variable
//	url := os.Getenv("AMQP_URL")
//
//	//If it doesnt exist, use the default connection string
//	if url == "" {
//		url = "amqp://guest:guest@localhost:5672"
//	}
//
//	// Connect to the rabbitMQ instance
//	connection, err := amqp.Dial(url)
//
//	if err != nil {
//		panic("could not establish connection with RabbitMQ:" + err.Error())
//	}
//
	// Create a channel from the connection. We'll use channels to access the data in the queue rather than the
	// connection itself
//	channel, err := conn.Channel()

//	if err != nil {
//		panic("could not open RabbitMQ channel:" + err.Error())
//	}
	// We consume data from the queue named Test using the channel we created in go.
	msgs, err := ch.Consume("response-consumer", "", false, false, false, false, nil)

	if err != nil {
		panic("error consuming the queue: " + err.Error())
	}

	// We loop through the messages in the queue and print them in the console.
	// The msgs will be a go channel, not an amqp channel
	for msg := range msgs {
		fmt.Println(string(msg.Body))
		msg.Ack(false)
	}

	// We close the connection after the operation has completed.
	//defer conn.Close()
}
