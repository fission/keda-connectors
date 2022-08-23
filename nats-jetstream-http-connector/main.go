package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"

	"github.com/fission/keda-connectors/common"
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
			conn.logger.Info("Received an interrupt, unsubscribing and closing connection...")
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
		msgs, _ := sub.Fetch(10, nats.Context(ctx))
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
				conn.logger.Info("Done processing message",
					zap.String("messsage", string(body)))
			}
		}
	}

}

func (conn jetstreamConnector) errorHandler(err error) {

	if len(conn.connectordata.ErrorTopic) == 0 {
		conn.logger.Warn("Error topic not set")
		return
	}

	// we consider the subject is of form - stream.subjectName.
	// We split it and use the first word to create and stream if not found.
	// Push the responses to the above created stream.
	errorTopic := strings.Split(conn.connectordata.ErrorTopic, ".")
	conn.logger.Debug("errortopic:", zap.String("topic: ", errorTopic[0]))

	streamCreationErr := conn.createStream(conn.jsContext, errorTopic[0], conn.connectordata.ErrorTopic)
	if streamCreationErr != nil {
		conn.logger.Error("failed to publish response body from http request to topic",
			zap.Error(streamCreationErr),
			zap.String("topic", conn.connectordata.ResponseTopic),
			zap.String("source", conn.connectordata.SourceName),
			zap.String("http endpoint", conn.connectordata.HTTPEndpoint),
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
	// We split it and use the first word to create and stream if not found.
	// Push the responses to the above created stream.
	responseTopic := strings.Split(conn.connectordata.ResponseTopic, ".")
	err := conn.createStream(conn.jsContext, responseTopic[0], conn.connectordata.ResponseTopic)
	if err != nil {
		conn.logger.Error("failed to publish response body from http request to topic",
			zap.Error(err),
			zap.String("topic", conn.connectordata.ResponseTopic),
			zap.String("source", conn.connectordata.SourceName),
			zap.String("http endpoint", conn.connectordata.HTTPEndpoint),
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
func (conn jetstreamConnector) createStream(js nats.JetStreamContext, streamName string, streamSubjects string) error {
	stream, err := js.StreamInfo(streamName)
	if err != nil {
		conn.logger.Info("stream not present will create one",
			zap.String("name", streamName))
	}
	if stream == nil {
		conn.logger.Info("creating stream",
			zap.String("name", streamName),
			zap.String("and subjects ", streamSubjects))
		_, err = js.AddStream(&nats.StreamConfig{
			Name:     streamName,
			Subjects: []string{streamSubjects},
		})
		if err != nil {
			conn.logger.Error("failed to publish response body from http request to topic",
				zap.Error(err))
			return err
		}
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
		logger.Fatal("err fission consumer mot provided")
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
