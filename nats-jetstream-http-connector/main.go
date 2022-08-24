package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"

	"github.com/fission/keda-connectors/common"
)

const (
	batchCount = 10
)

type jetstreamConnector struct {
	host            string
	fissionConsumer string
	connectordata   common.ConnectorMetadata
	jsContext       nats.JetStreamContext
	logger          *zap.Logger
}

func (conn jetstreamConnector) consumeMessage() {

	sub, err := conn.jsContext.PullSubscribe(os.Getenv("TOPIC"), conn.fissionConsumer, nats.PullMaxWaiting(512))
	if err != nil {
		conn.logger.Fatal("error occurred while consuming message", zap.Error(err))
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	for {
		select {
		case <-signalChan:
			ctx.Done()
			conn.logger.Info("received an interrupt, unsubscribing and closing connection...")
			err = sub.Unsubscribe()
			if err != nil {
				conn.logger.Error("error occurred while unsubscribing", zap.Error(err))
			}
			err = conn.jsContext.DeleteConsumer(os.Getenv("STREAM"), conn.fissionConsumer)
			if err != nil {
				conn.logger.Error("error occurred while closing connection", zap.Error(err))
			}
			return
		default:
		}
		msgs, _ := sub.Fetch(batchCount, nats.Context(ctx))
		for _, msg := range msgs {
			conn.handleHTTPRequest(msg)

		}
	}

}

func (conn jetstreamConnector) handleHTTPRequest(msg *nats.Msg) {

	headers := http.Header{
		"Topic":        {conn.connectordata.Topic},
		"RespTopic":    {conn.connectordata.ResponseTopic},
		"ErrorTopic":   {conn.connectordata.ErrorTopic},
		"Content-Type": {conn.connectordata.ContentType},
		"Source-Name":  {conn.connectordata.SourceName},
	}
	resp, err := common.HandleHTTPRequest(string(msg.Data), headers, conn.connectordata, conn.logger)
	if err != nil {
		conn.logger.Info(err.Error())
		conn.errorHandler(err)
		// mqtpod
	} else {
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			conn.logger.Info(err.Error())
			conn.errorHandler(err)
		} else {
			if success := conn.responseHandler(body); success {
				err = msg.Ack()
				if err != nil {
					conn.logger.Info(err.Error())
					conn.errorHandler(err)
				}
				conn.logger.Info("done processing message",
					zap.String("messsage", string(body)))
			}
		}
	}

}

func (conn jetstreamConnector) errorHandler(err error) {

	if len(conn.connectordata.ErrorTopic) == 0 {
		conn.logger.Warn("error topic not set in, ", zap.String("ERROR_STREAM: ", os.Getenv("ERROR_STREAM")))
		return
	}

	// we consider the subject is of form - stream.subjectName.
	streamCreationErr := conn.getStream(conn.jsContext, os.Getenv("ERROR_STREAM"))
	if streamCreationErr != nil {
		conn.logger.Error("failed to find error output stream",
			zap.Error(streamCreationErr),
			zap.String("topic", conn.connectordata.ResponseTopic),
			zap.String("error stream", os.Getenv("Error_STREAM")),
		)
		return
	}

	_, publishErr := conn.jsContext.Publish(conn.connectordata.ErrorTopic, []byte(err.Error()))

	if publishErr != nil {
		conn.logger.Error("failed to publish message to error topic",
			zap.Error(publishErr),
			zap.String("source", conn.connectordata.SourceName),
			zap.String("message", publishErr.Error()),
			zap.String("topic", conn.connectordata.ErrorTopic))
	}
}

func (conn jetstreamConnector) responseHandler(response []byte) bool {
	if len(conn.connectordata.ResponseTopic) == 0 {
		conn.logger.Warn("Response topic not set")
		return false
	}

	// we consider the subject is of form - stream.subjectName.
	// Push the responses to the above created stream.
	responseStream := os.Getenv("RESPONSE_STREAM")
	err := conn.getStream(conn.jsContext, responseStream)
	if err != nil {
		conn.logger.Error("failed to find response stream",
			zap.Error(err),
			zap.String("topic", conn.connectordata.ResponseTopic),
			zap.String("stream", responseStream),
		)
		return false
	}
	_, publishErr := conn.jsContext.Publish(conn.connectordata.ResponseTopic, response)
	if publishErr != nil {
		conn.logger.Error("failed to publish response body from http request to topic",
			zap.Error(publishErr),
			zap.String("topic", conn.connectordata.ResponseTopic),
			zap.String("source", conn.connectordata.SourceName),
			zap.String("http endpoint", conn.connectordata.HTTPEndpoint),
		)
		return false
	}
	return true
}

// createStream creates a stream by using JetStreamContext
func (conn jetstreamConnector) getStream(js nats.JetStreamContext, streamName string) error {
	stream, err := js.StreamInfo(streamName)
	if err != nil {
		conn.logger.Info("stream not present,",
			zap.String("name", streamName))
	}
	if stream == nil {
		return fmt.Errorf("output stream not found, streamName: %s", streamName)
	}
	return nil
}

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}

	var nc *nats.Conn
	defer func() {
		logger.Sync()
		nc.Close()
	}()

	connectordata, err := common.ParseConnectorMetadata()
	if err != nil {
		logger.Fatal("error occurred while parsing metadata", zap.Error(err))
	}
	host := os.Getenv("NATS_SERVER")

	if host == "" {
		logger.Fatal("received empty host field")
	}

	// Connect to NATS
	nc, err = nats.Connect(host)
	if err != nil {
		logger.Fatal("err while connecting: ", zap.Error(err))
	}
	js, err := nc.JetStream()
	if err != nil {
		logger.Fatal("err while getting jetstream context: ", zap.Error(err))
	}

	if len(os.Getenv("FISSION_CONSUMER")) <= 0 {
		logger.Fatal("err fission consumer not provided")
	}
	_, err = js.AddConsumer(os.Getenv("STREAM"), &nats.ConsumerConfig{
		Durable:   os.Getenv("FISSION_CONSUMER"),
		AckPolicy: nats.AckExplicitPolicy,
	})

	if err != nil {
		logger.Fatal("err while creating consumer: ", zap.Error(err))
	}
	conn := jetstreamConnector{
		host:            host,
		jsContext:       js,
		connectordata:   connectordata,
		logger:          logger,
		fissionConsumer: os.Getenv("FISSION_CONSUMER"),
	}

	conn.consumeMessage()
}
