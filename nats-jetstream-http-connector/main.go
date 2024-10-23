package main

import (
	"context"
	"io"
	"log"
	"maps"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

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
	consumer        string
	nc              *nats.Conn
	ackwait         string
	concurrentSem   chan int
}

func main() {
	host := os.Getenv("NATS_SERVER")
	consumer := os.Getenv("CONSUMER")
	ackwait := os.Getenv("ACKWAIT")

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}

	nc, err := nats.Connect(host)
	if err != nil {
		logger.Fatal("error while connecting to NATS:", zap.Error(err))
	}

	js, err := nc.JetStream()
	if err != nil {
		logger.Fatal("error while getting jetstream context:", zap.Error(err))
	}

	defer func() {
		logger.Sync()
	}()

	connectordata, err := common.ParseConnectorMetadata()
	if err != nil {
		logger.Fatal("error occurred while parsing metadata", zap.Error(err))
	}

	conn := jetstreamConnector{
		host:            host,
		fissionConsumer: consumer,
		connectordata:   connectordata,
		jsContext:       js,
		logger:          logger,
		consumer:        consumer,
		nc:              nc,
		ackwait:         ackwait,
		concurrentSem:   initialiseConcurrency(),
	}

	err = conn.consumeMessage()
	if err != nil {
		conn.logger.Fatal("error occurred while consuming messages", zap.Error(err))
	}
}

func initialiseConcurrency() chan int {
	concurrent := os.Getenv("CONCURRENT")
	concurrency := 1
	if concurrent != "" {
		var err error
		concurrency, err = strconv.Atoi(concurrent)
		if err != nil {
			concurrency = 1
		}
	}
	if concurrency < 1 {
		concurrency = 1
	}
	return make(chan int, concurrency)
}

func (conn jetstreamConnector) getAckwait() (time.Duration, error) {
	ackwait := 30 * time.Second
	if conn.ackwait != "" {
		var err error
		ackwait, err = time.ParseDuration(conn.ackwait)
		if err != nil {
			conn.logger.Debug("error occurred while parsing ackwait", zap.Error(err))
			return ackwait, err
		}
	}
	return ackwait, nil
}

func (conn jetstreamConnector) consumeMessage() error {
	// Establish ackwait
	ackwait, err := conn.getAckwait()
	if err != nil {
		return err
	}

	// Create durable consumer monitor
	sub, err := conn.jsContext.Subscribe(conn.connectordata.Topic, func(msg *nats.Msg) {
		conn.concurrentSem <- 1
		go conn.handleHTTPRequest(msg)
		// Durable is required because if we allow jetstream to create new consumer we
		// will be reading records from the start from the stream.
	}, nats.Durable(conn.consumer), nats.ManualAck(), nats.AckWait(ackwait))
	if err != nil {
		conn.logger.Fatal("error occurred while subscribing to topic", zap.Error(err))
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	signalChan := make(chan os.Signal, 2)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		sig := <-signalChan
		conn.logger.Info("received signal", zap.String("signal", sig.String()))
		cancel()
		<-signalChan
		conn.logger.Warn("double signal received, shutting down gracefully")
	}()

	<-ctx.Done()
	conn.logger.Info("unsubscribing and closing connection...")
	err = sub.Unsubscribe()
	if err != nil {
		conn.logger.Error("error while unsubscribing", zap.Error(err))
	}

	close(conn.concurrentSem)

	return nil
}

func (conn jetstreamConnector) handleHTTPRequest(msg *nats.Msg) {
	headers := http.Header{
		"Topic":        {conn.connectordata.Topic},
		"RespTopic":    {conn.connectordata.ResponseTopic},
		"ErrorTopic":   {conn.connectordata.ErrorTopic},
		"Content-Type": {conn.connectordata.ContentType},
		"Source-Name":  {conn.connectordata.SourceName},
	}

	maps.Copy(headers, msg.Header) // Add and overwrite headers from Jetstream

	resp, err := common.HandleHTTPRequest(string(msg.Data), headers, conn.connectordata, conn.logger)
	if err != nil {
		conn.logger.Error("error handling HTTP request", zap.Error(err))
		conn.errorHandler(err)
		conn.acknowledgeMsg(msg)
	} else {
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			conn.logger.Error("error reading response body", zap.Error(err))
			conn.errorHandler(err)
			conn.acknowledgeMsg(msg)
		} else {
			if success := conn.responseHandler(body); success {
				conn.acknowledgeMsg(msg)
				conn.logger.Info("done processing message", zap.String("message", string(body)))
			}
		}
	}

	<-conn.concurrentSem
}

func (conn jetstreamConnector) responseHandler(response []byte) bool {
	if len(conn.connectordata.ResponseTopic) == 0 {
		conn.logger.Warn("Response topic not set")
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

func (conn jetstreamConnector) errorHandler(err error) {
	if len(conn.connectordata.ErrorTopic) == 0 {
		conn.logger.Warn("error topic not set")
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

func (conn jetstreamConnector) acknowledgeMsg(msg *nats.Msg) {
	err := msg.Ack()
	if err != nil {
		conn.logger.Error("error acknowledging message", zap.Error(err))
	}
}
