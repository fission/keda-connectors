package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/fission/keda-connectors/common"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/stan.go"
	"go.uber.org/zap"
)

type natsConnector struct {
	host           string
	connectordata  common.ConnectorMetadata
	stanConnection stan.Conn
	logger         *zap.Logger
}

func (conn natsConnector) consumeMessage() {

	headers := http.Header{
		"Topic":        {conn.connectordata.Topic},
		"RespTopic":    {conn.connectordata.ResponseTopic},
		"ErrorTopic":   {conn.connectordata.ErrorTopic},
		"Content-Type": {conn.connectordata.ContentType},
		"Source-Name":  {conn.connectordata.SourceName},
	}
	forever := make(chan bool)
	_, err := conn.stanConnection.QueueSubscribe(os.Getenv("TOPIC"), os.Getenv("QUEUE_GROUP"), func(m *stan.Msg) {
		msg := string(m.Data)
		conn.logger.Info(msg)
		_, resp, err := common.HandleHTTPRequest(msg, headers, conn.connectordata, conn.logger)
		if err != nil {
			conn.logger.Info(err.Error())
			conn.errorHandler(err)
		} else {
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				conn.logger.Info(err.Error())
				conn.errorHandler(err)
			} else {
				if success := conn.responseHandler(body); success {
					conn.logger.Info("Done processing message",
						zap.String("messsage", string(body)))
				}
			}
		}
	}, stan.DurableName(os.Getenv("DURABLE_NAME")), stan.DeliverAllAvailable())

	if err != nil {
		conn.logger.Fatal("error occurred while consuming message", zap.Error(err))
	}

	conn.logger.Info("NATs consumer up and running!...")
	<-forever
}

func (conn natsConnector) errorHandler(err error) {
	publishErr := conn.stanConnection.Publish(conn.connectordata.ErrorTopic, []byte(err.Error()))

	if publishErr != nil {
		conn.logger.Error("failed to publish message to error topic",
			zap.Error(publishErr),
			zap.String("source", conn.connectordata.SourceName),
			zap.String("message", publishErr.Error()),
			zap.String("topic", conn.connectordata.ErrorTopic))
	}
}

func (conn natsConnector) responseHandler(response []byte) bool {

	if len(conn.connectordata.ResponseTopic) > 0 {

		publishErr := conn.stanConnection.Publish(conn.connectordata.ResponseTopic, response)

		if publishErr != nil {
			conn.logger.Error("failed to publish response body from http request to topic",
				zap.Error(publishErr),
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

	host := os.Getenv("NATS_SERVER")

	if host == "" {
		logger.Fatal("received empty host field")
	}

	nc, err := nats.Connect(host)

	if err != nil {
		logger.Fatal("failed to establish connection with NATS", zap.Error(err))
	}

	sc, err := stan.Connect(os.Getenv("CLUSTER_ID"), os.Getenv("CLIENT_ID"), stan.NatsConn(nc))
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	conn := natsConnector{
		host:           host,
		stanConnection: sc,
		connectordata:  connectordata,
		logger:         logger,
	}
	conn.consumeMessage()
}
