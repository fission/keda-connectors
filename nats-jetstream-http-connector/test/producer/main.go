package main

import (
	"log"
	"os"
	"strconv"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

const (
	streamName     = "input"
	streamSubjects = "input.*"
	subjectName    = "input.created"
	outputStream   = "output"
	outputSubject  = "output.*"
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
	nc, err := nats.Connect(host)
	checkErr(logger, err)
	// Creates JetStreamContext
	js, err := nc.JetStream()
	checkErr(logger, err)
	// Creates stream
	err = createStream(logger, js, streamName, streamSubjects)
	checkErr(logger, err)
	// Create records by publishing messages
	err = publishdata(logger, js)
	checkErr(logger, err)

	// Creates stream for output
	err = createStream(logger, js, outputStream, outputSubject)
	checkErr(logger, err)

	// // Creates stream for output error
	// err = createStream(logger, js, "errstream", "errstream.*")
	// checkErr(logger, err)

	// This is to run the process forever and presents container to get restarted
	select {}
}

// publishdata publishes data to input stream
func publishdata(logger *zap.Logger, js nats.JetStreamContext) error {

	no, err := strconv.Atoi(os.Getenv("COUNT"))
	if err != nil {
		logger.Error("invalid count provided. Err: ", zap.Error(err))
		no = 3
		err = nil
	}
	for i := 1; i <= no; i++ {
		_, err := js.Publish(subjectName, []byte("Test"+strconv.Itoa(i)))
		if err != nil {
			logger.Error("Error found: ", zap.Error(err))
			return err
		}
		logger.Info("Order with OrderID:", zap.Int("%d has been published\n", i))
	}
	return nil
}

// createStream creates a stream by using JetStreamContext
func createStream(logger *zap.Logger, js nats.JetStreamContext, streamName, streamSubjects string) error {
	stream, err := js.StreamInfo(streamName)
	if err != nil {
		logger.Info("stream not found: ", zap.Error(err))
	}
	if stream == nil {
		logger.Info("creating stream:", zap.String("%q and subjects", streamName), zap.String("%v", streamSubjects))
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

func checkErr(logger *zap.Logger, err error) {
	if err != nil {
		logger.Fatal("err: ", zap.Error(err))
	}
}
