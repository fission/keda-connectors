package main

import (
	"log"
	"os"
	"strconv"

	"github.com/nats-io/nats.go"
)

const (
	streamName     = "input"
	streamSubjects = "input.*"
	subjectName    = "input.created"
)

func main() {

	// Connect to NATS
	host := os.Getenv("NATS_SERVER")
	if host == "" {
		log.Fatal("received empty host field")
	}
	nc, err := nats.Connect(host)
	checkErr(err)
	// Creates JetStreamContext
	js, err := nc.JetStream()
	checkErr(err)
	// Creates stream
	err = createStream(js)
	checkErr(err)
	// Create records by publishing messages
	err = publishdata(js)
	checkErr(err)
}

// publishdata publishes data to input stream
func publishdata(js nats.JetStreamContext) error {

	no, err := strconv.Atoi(os.Getenv("COUNT"))
	if err != nil {
		log.Println("invalid count provided. Err: ", err)
		return err
	}
	for i := 1; i <= no; i++ {
		_, err := js.Publish(subjectName, []byte("Test"+strconv.Itoa(i)))
		if err != nil {
			return err
		}
		log.Printf("Order with OrderID:%d has been published\n", i)
	}
	return nil
}

// createStream creates a stream by using JetStreamContext
func createStream(js nats.JetStreamContext) error {
	stream, err := js.StreamInfo(streamName)
	if err != nil {
		log.Println(err)
	}
	if stream == nil {
		log.Printf("creating stream %q and subjects %q", streamName, streamSubjects)
		_, err = js.AddStream(&nats.StreamConfig{
			Name:     streamName,
			Subjects: []string{streamSubjects},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
