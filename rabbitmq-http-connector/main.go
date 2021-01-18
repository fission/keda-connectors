package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/streadway/amqp"
	"go.uber.org/zap"

	"github.com/fission/keda-connectors/common"
)

type rabbitMQConnector struct {
	host            string
	connectordata   common.ConnectorMetadata
	consumerChannel *amqp.Channel
	producerChannel *amqp.Channel
	logger          *zap.Logger
}

func (conn rabbitMQConnector) consumeMessage() {
	msgs, err := conn.consumerChannel.Consume(
		conn.connectordata.Topic, // queue
		"",                       // consumer
		false,                    // auto-ack
		false,                    // exclusive
		false,                    // no-local
		false,                    // no-wait
		nil,                      // args
	)

	if err != nil {
		conn.logger.Fatal("error occurred while consuming message", zap.Error(err))
	}

	headers := http.Header{
		"KEDA-Topic":          {conn.connectordata.Topic},
		"KEDA-Response-Topic": {conn.connectordata.ResponseTopic},
		"KEDA-Error-Topic":    {conn.connectordata.ErrorTopic},
		"Content-Type":        {conn.connectordata.ContentType},
		"KEDA-Source-Name":    {conn.connectordata.SourceName},
	}

	forever := make(chan bool)
	sem := make(chan int, 10) // Process maximum 10 messages concurrently

	go func() {
		for d := range msgs {
			sem <- 1
			go func(d amqp.Delivery) {
				msg := string(d.Body)
				_, resp, err := common.HandleHTTPRequest(msg, headers, conn.connectordata, conn.logger)
				if err != nil {
					conn.errorHandler(err)
				} else {
					defer resp.Body.Close()
					body, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						conn.errorHandler(err)
					} else {
						if success := conn.responseHandler(string(body)); success {
							d.Ack(false)
						}
					}
				}
				<-sem
			}(d)
		}
	}()

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
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()

	connectordata, err := common.ParseConnectorMetadata()

	host := os.Getenv("HOST")
	if os.Getenv("INCLUDE_UNACKED") == "true" {
		logger.Fatal("only amqp protocol host is supported")
	}
	if host == "" {
		logger.Fatal("received empty host field")
	}

	connection, err := amqp.Dial(host)
	if err != nil {
		logger.Fatal("failed to establish connection with RabbitMQ", zap.Error(err))
	}
	defer connection.Close()

	producerChannel, err := connection.Channel()
	if err != nil {
		logger.Fatal("failed to open RabbitMQ channel for producer", zap.Error(err))
	}
	defer producerChannel.Close()

	consumerChannel, err := connection.Channel()
	if err != nil {
		logger.Fatal("failed to open RabbitMQ channel for consumer", zap.Error(err))
	}
	defer consumerChannel.Close()

	conn := rabbitMQConnector{
		host:            host,
		connectordata:   connectordata,
		consumerChannel: consumerChannel,
		producerChannel: producerChannel,
		logger:          logger,
	}
	conn.consumeMessage()
}
