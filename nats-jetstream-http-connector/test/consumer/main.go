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
	streamNameErr     = "output"
	errStreamSubjects = "output.error-topic"
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
	go consumerMessage(logger, js, streamName, streamSubjects, "response_consumer1")
	consumerMessage(logger, js, streamNameErr, errStreamSubjects, "err_consumer1")

	fmt.Println("All messages consumed")

}

func consumerMessage(logger *zap.Logger, js nats.JetStreamContext, stream, topic, consumer string) {

	fmt.Println(topic)

	/*
		// Push subscriber
			js.Subscribe(topic, func(msg *nats.Msg) {
				msg.Ack()

				log.Println(string(msg.Data))
			}, nats.Durable(consumer), nats.ManualAck()) // Durable is required because if we allow jetstream to create new consumer we will be reading records from the start from the stream.
			runtime.Goexit()
	*/
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
				fmt.Errorf("error occurred while closing connection %v", err.Error())
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
