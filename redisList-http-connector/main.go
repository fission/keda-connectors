package main

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/fission/keda-connectors/common"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type redisListConnector struct {
	rdbConnection *redis.Client
	connectordata common.ConnectorMetadata
	logger        *zap.Logger
}

func (conn redisListConnector) consumeMessage(ctx context.Context) {

	messages, err := conn.rdbConnection.LRange(ctx, conn.connectordata.Topic, 0, -1).Result()
	if err != nil {
		conn.logger.Fatal("Error in redis list consumer", zap.Error(err))
	}
	headers := http.Header{
		"KEDA-Topic":          {conn.connectordata.Topic},
		"KEDA-Response-Topic": {conn.connectordata.ResponseTopic},
		"KEDA-Error-Topic":    {conn.connectordata.ErrorTopic},
		"Content-Type":        {conn.connectordata.ContentType},
		"KEDA-Source-Name":    {conn.connectordata.SourceName},
	}

	forever := make(chan bool)
	go func() {
		for message := range messages {
			response, err := common.HandleHTTPRequest(string(message), headers, conn.connectordata, conn.logger)
			if err != nil {
				conn.errorHandler(ctx, err)
			} else {
				defer response.Body.Close()
				body, err := ioutil.ReadAll(response.Body)
				if err != nil {
					conn.errorHandler(ctx, err)
				} else {
					if success := conn.responseHandler(ctx, string(body)); success {
						//Ack
					}
				}
			}
		}
	}()
	conn.logger.Info("Redis consumer up and running!")
	<-forever
}

func (conn redisListConnector) errorHandler(ctx context.Context, err error) {
    if len(conn.connectordata.ErrorTopic) > 0 {
        err = conn.rdbConnection.RPush(ctx,conn.connectordata.ErrorTopic,err.Error()).Err()
        if err != nil{
            conn.logger.Error("Failed to add message in error topic list",
                zap.Error(err),
                zap.String("source",conn.connectordata.SourceName),
                zap.String("message",err.Error()),
                zap.String("topic",conn.connectordata.ErrorTopic))
            }
    } else{
        conn.logger.Error("message received to add to error topic list, but no error topic was set",
                zap.String("message",err.Error()),
                zap.String("source",conn.connectordata.SourceName),
                zap.String("http endpoint",conn.connectordata.HTTPEndpoint))
    }
}

func (conn redisListConnector) responseHandler(ctx context.Context,response string) bool {
    if len(conn.connectordata.ResponseTopic) > 0{
        err := conn.rdbConnection.RPush(ctx, conn.connectordata.ResponseTopic,response).Err()
        if err != nil{
            conn.logger.Error("failed to push response to from http request to topic list",
            zap.Error(err),
            zap.String("topic",conn.connectordata.ResponseTopic),
            zap.String("source",conn.connectordata.SourceName),
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

	address := os.Getenv("REDIS_ADDRESS")
	if address == "" {
		logger.Fatal("Empty address field")
	}
	password := os.Getenv("REDIS_PASSWORD")

	var ctx = context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
	})

	conn := redisListConnector{
		rdbConnection: rdb,
		connectordata: connectordata,
		logger:        logger,
	}
	conn.consumeMessage(ctx)
}
