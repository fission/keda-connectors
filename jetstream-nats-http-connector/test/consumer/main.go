package main

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/nats-io/nats.go"
)

var (
	streamName     = "output"
	streamSubjects = "output.response-topic"
)

func main() {
	// Connect to NATS

	host := os.Getenv("NATS_SERVER")
	if host == "" {
		log.Fatal("received empty host field")
	}
	nc, _ := nats.Connect(host)
	js, err := nc.JetStream()
	if err != nil {
		log.Fatal(err)
	}
	createStream(js, streamName, streamSubjects)
	// Create durable consumer monitor
	_, err = js.Subscribe(streamSubjects, func(msg *nats.Msg) {
		msg.Ack()
		m := string(msg.Data)
		fmt.Println(m)
	}, nats.Durable("output_consumer"), nats.ManualAck())

	fmt.Println(err)
	fmt.Println("All messages consumed")
	runtime.Goexit()

}

// createStream creates a stream by using JetStreamContext
func createStream(js nats.JetStreamContext, streamName string, streamSubjects string) error {
	stream, _ := js.StreamInfo(streamName)

	if stream == nil {

		_, err := js.AddStream(&nats.StreamConfig{
			Name:     streamName,
			Subjects: []string{streamSubjects},
		})
		if err != nil {
			log.Println("Error: ", err)
			return err
		}
	}
	return nil
}
