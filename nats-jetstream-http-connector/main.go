package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
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
	consumer        string
}

func main() {

	host := os.Getenv("NATS_SERVER")
	consumer := os.Getenv("CONSUMER")

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}

	nc, _ := nats.Connect(host)
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

	err = validateStream(js, connectordata.Topic, logger)
	if err != nil {
		logger.Fatal("error occurred while validating streams", zap.Error(err))
	}
	// input stream
	err = validateStream(js, connectordata.ResponseTopic, logger)
	if err != nil {
		logger.Fatal("error occurred while validating streams", zap.Error(err))
	}

	// output stream
	err = validateStream(js, connectordata.ErrorTopic, logger)
	if err != nil {
		logger.Fatal("error occurred while validating streams", zap.Error(err))
	}

	conn := jetstreamConnector{
		host:            host,
		fissionConsumer: consumer,
		connectordata:   connectordata,
		jsContext:       js,
		logger:          logger,
		consumer:        consumer,
	}
	err = conn.consumerMessage()
	if err != nil {
		conn.logger.Fatal("error occurred while parsing metadata", zap.Error(err))
	}
}

func validateStream(js nats.JetStreamContext, topic string, logger *zap.Logger) (err error) {

	// check if stream present in NATS server
	streamNTopic := strings.Split(topic, ".")
	stream := streamNTopic[0]
	info, err := js.StreamInfo(stream)
	if err != nil || info == nil {
		err = fmt.Errorf("stream not found: %s", stream)
		logger.Debug("stream not found, ", zap.Error(err))
	}
	return
}

func (conn jetstreamConnector) consumerMessage() error {
	// Create durable consumer monitor
	_, err := conn.jsContext.Subscribe(conn.connectordata.Topic, func(msg *nats.Msg) {
		conn.handleHTTPRequest(msg)
		// Durable is required because if we allow jetstream to create new consumer we
		// will be reading records from the start from the stream.
	}, nats.Durable(conn.consumer), nats.ManualAck())
	if err != nil {
		conn.logger.Debug("error occurred while parsing metadata", zap.Error(err))
		return err
	}
	runtime.Goexit()
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
	resp, err := common.HandleHTTPRequest(string(msg.Data), headers, conn.connectordata, conn.logger)
	if err != nil {
		conn.logger.Info(err.Error())
		conn.errorHandler(err)
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
