package main

import (
	"fmt"
	"log"
	"os"

	"github.com/pkg/errors"
	"github.com/streadway/amqp"
	"go.uber.org/zap"

	"github.com/fission/keda-connectors/common"
)

type rabbitMQConnector struct {
	queueName       string
	host            string
	connectordata   common.ConnectorMetadata
	consumerChannel *amqp.Channel
	producerChannel *amqp.Channel
	logger          *zap.Logger
}

func newRabbitMQConnector() (*rabbitMQConnector, error) {
	connectordata, err := common.ParseConnectorMetadata()
	if err != nil {
		return &rabbitMQConnector{}, err
	}

	host := os.Getenv("HOST")
	if os.Getenv("INCLUDE_UNACKED") == "true" {
		return &rabbitMQConnector{}, fmt.Errorf("only amqp protocol host is supported")
	}
	if host == "" {
		return &rabbitMQConnector{}, fmt.Errorf("received empty host field")
	}

	queueName := os.Getenv("QUEUE_NAME")
	if queueName == "" {
		return &rabbitMQConnector{}, fmt.Errorf("received empty queue name")
	}

	connection, err := amqp.Dial(host)
	if err != nil {
		return &rabbitMQConnector{}, errors.Wrapf(err, "failed to establish connection with RabbitMQ")
	}
	defer connection.Close()

	producerChannel, err := connection.Channel()
	if err != nil {
		return &rabbitMQConnector{}, errors.Wrapf(err, "failed to open RabbitMQ channel for producer")
	}
	defer producerChannel.Close()

	consumerChannel, err := connection.Channel()
	if err != nil {
		return &rabbitMQConnector{}, errors.Wrapf(err, "failed to open RabbitMQ channel for consumer")
	}
	defer consumerChannel.Close()

	return &rabbitMQConnector{
		queueName:       queueName,
		host:            host,
		connectordata:   connectordata,
		consumerChannel: consumerChannel,
		producerChannel: producerChannel,
	}, nil
}

func (conn rabbitMQConnector) consumeMessage() {
	msgs, err := conn.consumerChannel.Consume(
		conn.queueName, // queue
		"",             // consumer
		false,          // auto-ack
		false,          // exclusive
		false,          // no-local
		false,          // no-wait
		nil,            // args
	)

	if err != nil {
		conn.logger.Fatal("error occurred while consuming message", zap.Error(err))
	}

	headers := map[string]string{
		"Topic":        conn.connectordata.Topic,
		"RespTopic":    conn.connectordata.ResponseTopic,
		"ErrorTopic":   conn.connectordata.ErrorTopic,
		"Content-Type": conn.connectordata.ContentType,
		"Source-Name":  conn.connectordata.SourceName,
	}

	forever := make(chan bool)

	for d := range msgs {
		go func(d amqp.Delivery) {
			msg := string(d.Body)
			respBody, err := common.HandleHTTPRequest(msg, headers, conn.connectordata, conn.logger)
			if err != nil {
				conn.errorHandler(err)
			} else {
				if success := conn.responseHandler(respBody); success {
					d.Ack(false)
				}
			}
		}(d)
	}

	conn.logger.Info("RabbitMQ consumer up and running!...")
	<-forever
}

func (conn rabbitMQConnector) errorHandler(err error) {
	if len(conn.connectordata.ErrorTopic) > 0 {
		err = conn.producerChannel.Publish(
			"",                            // exchange
			conn.connectordata.ErrorTopic, // routing key
			false,                         // mandatory
			false,                         // immediate
			amqp.Publishing{
				ContentType: conn.connectordata.ContentType,
				Body:        []byte(err.Error()),
			})
		if err != nil {
			conn.logger.Error("failed to publish message to error topic",
				zap.Error(err),
				zap.String("source", conn.connectordata.SourceName),
				zap.String("message", err.Error()),
				zap.String("topic", conn.connectordata.ErrorTopic))
		}
	} else {
		conn.logger.Error("message received to publish to error topic, but no error topic was set",
			zap.String("message", err.Error()),
			zap.String("source", conn.connectordata.SourceName),
			zap.String("http endpoint", conn.connectordata.HTTPEndpoint),
		)
	}
}

func (conn rabbitMQConnector) responseHandler(response string) bool {
	if len(conn.connectordata.ResponseTopic) > 0 {
		err := conn.producerChannel.Publish(
			"",                               // exchange
			conn.connectordata.ResponseTopic, // routing key
			false,                            // mandatory
			false,                            // immediate
			amqp.Publishing{
				ContentType: conn.connectordata.ContentType,
				Body:        []byte(response),
			})
		if err != nil {
			conn.logger.Error("failed to publish response body from http request to topic",
				zap.Error(err),
				zap.String("topic", conn.connectordata.ResponseTopic),
				zap.String("source", conn.connectordata.SourceName),
				zap.String("http endpoint", conn.connectordata.HTTPEndpoint),
			)
			return false
		}
	}
	return true
}

func main() {
	conn, err := newRabbitMQConnector()
	if err != nil {
		log.Fatalf("failed to create connection %s", err)
	}
	conn.consumeMessage()
}
