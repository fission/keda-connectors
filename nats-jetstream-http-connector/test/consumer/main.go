package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

const (
	batchCount = 10
)

var (
	streamName        = "output"
	streamSubjects    = "output.response-topic"
	streamNameErr     = "errstream"
	errStreamSubjects = "errstream.error-topic"
)

func main() {

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}

	// Connect to NATS
	host := os.Getenv("NATS_SERVER")
	if host == "" {
		logger.Fatal("received empty host field")
	}
	nc, _ := nats.Connect(host)
	js, err := nc.JetStream()
	if err != nil {
		logger.Fatal("err: ", zap.Error(err))
	}
	createStream(logger, js, streamName, streamSubjects)
	go consumerMessage(logger, js, streamSubjects, streamName, "response_consumer")

	// handle error
	createStream(logger, js, streamNameErr, errStreamSubjects)
	consumerMessage(logger, js, errStreamSubjects, streamNameErr, "err_consumer")

	fmt.Println("All messages consumed")

}

func consumerMessage(logger *zap.Logger, js nats.JetStreamContext, topic, stream, consumer string) {
	sub, err := js.PullSubscribe(topic, consumer, nats.PullMaxWaiting(512))
	if err != nil {
		fmt.Printf("error occurred while consuming message:  %v", err.Error())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	for {
		select {
		case <-signalChan:
			ctx.Done()
			err = sub.Unsubscribe()
			if err != nil {
				logger.Error("error in unsubscribing: ",
					zap.Error(err),
				)
			}
			err = js.DeleteConsumer(stream, consumer)
			if err != nil {
				fmt.Errorf("error occurred while closing connection %s", err.Error())
			}
			return
		default:
		}
		msgs, _ := sub.Fetch(batchCount, nats.Context(ctx))
		for _, msg := range msgs {
			fmt.Println("consumed message: ", string(msg.Data))
			msg.Ack()

		}
	}

}

// createStream creates a stream by using JetStreamContext
func createStream(logger *zap.Logger, js nats.JetStreamContext, streamName string, streamSubjects string) error {
	stream, _ := js.StreamInfo(streamName)

	if stream == nil {

		_, err := js.AddStream(&nats.StreamConfig{
			Name:     streamName,
			Subjects: []string{streamSubjects},
		})
		if err != nil {
			logger.Error("Error: ",
				zap.Error(err),
			)
			return err
		}
	}
	return nil
}
