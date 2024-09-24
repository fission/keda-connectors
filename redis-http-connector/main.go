package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"github.com/fission/keda-connectors/common"
)

type redisConnector struct {
	rdbConnection *redis.Client
	connectordata common.ConnectorMetadata
	logger        *zap.Logger
}

func (conn redisConnector) consumeMessage(ctx context.Context) {

	headers := http.Header{
		"KEDA-Topic":          {conn.connectordata.Topic},
		"KEDA-Response-Topic": {conn.connectordata.ResponseTopic},
		"KEDA-Error-Topic":    {conn.connectordata.ErrorTopic},
		"Content-Type":        {conn.connectordata.ContentType},
		"KEDA-Source-Name":    {conn.connectordata.SourceName},
	}

	forever := make(chan bool)

	go func() {
		for {
			// BLPop will block and wait for a new message if the list is empty
			msg, err := conn.rdbConnection.BLPop(ctx, 0, conn.connectordata.Topic).Result()
			if err != nil {
				conn.logger.Fatal("Error in consuming queue: ", zap.Error((err)))
			}

			if len(msg) > 1 {
				// BLPop returns a slice with topic and message, we need the second item
				message := msg[1]
				response, err := common.HandleHTTPRequest(message, headers, conn.connectordata, conn.logger)
				if err != nil {
					conn.errorHandler(ctx, err)
				} else {
					body, err := io.ReadAll(response.Body)
					if err != nil {
						conn.errorHandler(ctx, err)
					} else {
						if success := conn.responseHandler(ctx, string(body)); success {
							conn.logger.Info("Message sending to response successful")
						}
					}
					err = response.Body.Close()
					if err != nil {
						conn.logger.Error(err.Error())
					}
				}
			}
		}
	}()
	conn.logger.Info("Redis consumer up and running!")
	<-forever
}

func (conn redisConnector) errorHandler(ctx context.Context, err error) {
	if len(conn.connectordata.ErrorTopic) > 0 {
		err = conn.rdbConnection.RPush(ctx, conn.connectordata.ErrorTopic, err.Error()).Err()
		if err != nil {
			conn.logger.Error("Failed to add message in error topic",
				zap.Error(err),
				zap.String("source", conn.connectordata.SourceName),
				zap.String("message", err.Error()),
				zap.String("topic", conn.connectordata.ErrorTopic))
		}
	} else {
		conn.logger.Error("message received to add to error topic list, but no error topic was set",
			zap.String("message", err.Error()),
			zap.String("source", conn.connectordata.SourceName),
			zap.String("http endpoint", conn.connectordata.HTTPEndpoint))
	}
}

func (conn redisConnector) responseHandler(ctx context.Context, response string) bool {
	if len(conn.connectordata.ResponseTopic) > 0 {
		err := conn.rdbConnection.RPush(ctx, conn.connectordata.ResponseTopic, response).Err()
		if err != nil {
			conn.logger.Error("failed to push response to from http request to topic",
				zap.Error(err),
				zap.String("topic", conn.connectordata.ResponseTopic),
				zap.String("source", conn.connectordata.SourceName),
				zap.String("http endpoint", conn.connectordata.HTTPEndpoint))
			return false
		}
	}
	return true
}

func main() {

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Can't initiate zap logger :%v", err)
	}
	defer logger.Sync()

	connectordata, err := common.ParseConnectorMetadata()
	if err != nil {
		logger.Error("Error while parsing connector metadata", zap.Error(err))
	}

	address := os.Getenv("ADDRESS")
	if address == "" {
		logger.Fatal("Empty address field")
	}
	password := os.Getenv("PASSWORD_FROM_ENV")

	var ctx = context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
	})

	conn := redisConnector{
		rdbConnection: rdb,
		connectordata: connectordata,
		logger:        logger,
	}
	conn.consumeMessage(ctx)
}
