package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/fission/keda-connectors/common"
)

type redisConnector struct {
	rdbConnection *redis.Client
	connectordata common.ConnectorMetadata
	logger        *zap.Logger
}

func (conn redisConnector) consumeMessage(ctx context.Context) error {
	headers := http.Header{
		"KEDA-Topic":          {conn.connectordata.Topic},
		"KEDA-Response-Topic": {conn.connectordata.ResponseTopic},
		"KEDA-Error-Topic":    {conn.connectordata.ErrorTopic},
		"Content-Type":        {conn.connectordata.ContentType},
		"KEDA-Source-Name":    {conn.connectordata.SourceName},
	}

	for {
		// Check if the context is done
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// BLPop will block and wait for a new message if the list is empty
		msg, err := conn.rdbConnection.BLPop(ctx, 0, conn.connectordata.Topic).Result()
		if err != nil {
			return fmt.Errorf("error in consuming queue: %w", err)
		}

		if len(msg) > 1 {
			// BLPop returns a slice with topic and message, we need the second item
			message := msg[1]
			response, err := common.HandleHTTPRequest(message, headers, conn.connectordata, conn.logger)
			if err != nil {
				conn.errorHandler(ctx, err)
				continue // Skip to the next iteration
			}

			body, err := io.ReadAll(response.Body)
			if err != nil {
				conn.errorHandler(ctx, err)
				response.Body.Close() // Close the body even if ReadAll fails
				continue              // Skip to the next iteration
			}

			if success := conn.responseHandler(ctx, string(body)); success {
				conn.logger.Info("Message sending to response successful")
			}

			if err := response.Body.Close(); err != nil {
				conn.logger.Warn("Error closing response body", zap.Error(err))
			}
		}
	}
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

func (conn redisConnector) close() error {
	return conn.rdbConnection.Close()
}

func main() {

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Can't initiate zap logger :%v", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	connectordata, err := common.ParseConnectorMetadata()
	if err != nil {
		logger.Fatal("Error while parsing connector metadata", zap.Error(err))
	}

	address := os.Getenv("ADDRESS")
	if address == "" {
		logger.Fatal("Empty address field")
	}
	password := os.Getenv("PASSWORD_FROM_ENV")

	rdb := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
	})

	ctx, cancel := context.WithCancel(signals.SetupSignalHandler())
	defer cancel()

	wg := sync.WaitGroup{}
	conn := redisConnector{
		rdbConnection: rdb,
		connectordata: connectordata,
		logger:        logger,
	}
	defer func() {
		if err := conn.close(); err != nil {
			logger.Error("Error closing Redis connection", zap.Error(err))
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if err := conn.consumeMessage(ctx); err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					conn.logger.Info("Context cancelled, stopping consumer")
					return
				}
				logger.Error("Error in consuming message", zap.Error(err))
			}
			// Remove the context check here, as it's now handled in consumeMessage
			conn.logger.Info("Restarting consumer")
		}
	}()
	wg.Wait()
	logger.Info("Terminating: Redis consumer")
}
