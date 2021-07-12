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
				conn.errorHandler(err)
			} else {
				defer response.Body.Close()
				body, err := ioutil.ReadAll(response.Body)
				if err != nil {
					conn.errorHandler(err)
				} else {
					if success := conn.responseHandler(string(body)); success {
						//Ack
					}
				}
			}
		}
	}()
	conn.logger.Info("Redis consumer up and running!")
	<-forever
}

func (conn redisListConnector) errorHandler(err error) {
	//WIP
}

func (conn redisListConnector) responseHandler(response string) bool {

	//WIP
	return true //temporary
}
func main() {

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Can't initiate zap logger :%v", err)
	}
	defer logger.Sync()

	connectordata, err := common.ParseConnectorMetadata()

	address := os.Getenv("ADDRESS")
	if address == "" {
		logger.Fatal("Empty address field")
	}
	password := os.Getenv("PASSWORD")
	if password == "" {
		logger.Fatal("Empty password field")
	}

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
