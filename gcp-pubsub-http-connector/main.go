package main

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"

	"cloud.google.com/go/pubsub"
	"go.uber.org/zap"
	"google.golang.org/api/option"

	"github.com/fission/keda-connectors/common"
)

type pubsubConnector struct {
	pubsubInfo    GCPPubsubConnInfo
	connectordata common.ConnectorMetadata
	logger        *zap.Logger
}

// GCPPubsubConnInfo contains the fields needed to connect to the GCP's pub sub topic queue.
type GCPPubsubConnInfo struct {
	ProjectID      string
	SubscriptionID string
	// Creds points to the json value(not file) which has the service account key
	Creds string
}

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()

	connectordata, err := common.ParseConnectorMetadata()
	if err != nil {
		logger.Error("Environment variable is missing ", zap.Error(err))
		os.Exit(127)
	}

	pubsubInfo, err := GetGCPInfo()
	if err != nil {
		logger.Error("failed to find GCP creds or project name or subscription name", zap.Error(err))
		return
	}
	conn := pubsubConnector{
		pubsubInfo:    *pubsubInfo,
		connectordata: connectordata,
		logger:        logger,
	}

	logger.Info("Conn: %s", zap.String("Response topic", conn.connectordata.ResponseTopic))
	conn.consumeMessage()
}

func (conn pubsubConnector) consumeMessage() {

	creds := []byte(conn.pubsubInfo.Creds)
	headers := http.Header{
		"KEDA-Topic":          {conn.connectordata.Topic},
		"KEDA-Response-Topic": {conn.connectordata.ResponseTopic},
		"KEDA-Error-Topic":    {conn.connectordata.ErrorTopic},
		"Content-Type":        {conn.connectordata.ContentType},
		"KEDA-Source-Name":    {conn.connectordata.SourceName},
	}
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, conn.pubsubInfo.ProjectID, option.WithCredentialsJSON(creds))
	if err != nil {
		conn.logger.Error(err.Error())
	}

	var mu sync.Mutex
	sub := client.Subscription(conn.pubsubInfo.SubscriptionID)
	cctx, _ := context.WithCancel(ctx)

	err = sub.Receive(cctx, func(ctx context.Context, msg *pubsub.Message) {
		mu.Lock()
		defer mu.Unlock()

		// Set the attributes as message header came from PubSub record
		for k, v := range msg.Attributes {
			headers.Add(k, v)
		}

		// Push the message to the endpoint
		resp, err := common.HandleHTTPRequest(string(msg.Data), headers, conn.connectordata, conn.logger)
		if err != nil {
			if conn.connectordata.ErrorTopic != "" {
				conn.responseOrErrorHandler(conn.connectordata.ErrorTopic, "Error", headers)
			}
			conn.logger.Error("Error sending the message to the endpoint %v", zap.Error(err))
		} else {
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				if conn.connectordata.ErrorTopic != "" {
					conn.responseOrErrorHandler(conn.connectordata.ErrorTopic, string(body), headers)
				}
				conn.logger.Error("Error reading body", zap.Error(err))

			} else {
				msg.Ack()
				if conn.connectordata.ResponseTopic != "" {
					conn.responseOrErrorHandler(conn.connectordata.ResponseTopic, string(body), headers)
				}
				conn.logger.Info("Success in sending the message", zap.Any("Messsage sent:  ", msg))
			}
		}

	})

}

func (conn pubsubConnector) responseOrErrorHandler(topicID string, response string, headers http.Header) {

	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, conn.pubsubInfo.ProjectID, option.WithCredentialsJSON([]byte(conn.pubsubInfo.Creds)))
	if err != nil {
		conn.logger.Error("pubsub.NewClient: %v", zap.Error(err))
	}

	t := client.Topic(topicID)
	result := t.Publish(ctx, &pubsub.Message{
		Data:       []byte(response),
		Attributes: convHeadersToAttr(headers),
	})

	go func(res *pubsub.PublishResult) {
		// The Get method blocks until a server-generated ID or
		// an error is returned for the published message.
		_, err := res.Get(ctx)
		if err != nil {
			conn.logger.Error("Failed to publish: %v", zap.Error(err))
			return
		}
	}(result)
	conn.logger.Info("Published message , topic name: %v\n", zap.String("Topic", topicID))

}

// convHeadersToAttr converts the headers to attributes which can be published by pubsub
func convHeadersToAttr(headers http.Header) map[string]string {
	var attr = make(map[string]string)
	for k, v := range headers {
		for _, d := range v {
			attr[k] = d
		}
	}
	return attr
}

//GetGCPInfo gets the configuration required to connect to GCP
func GetGCPInfo() (*GCPPubsubConnInfo, error) {

	creds := os.Getenv("CREDENTIALS_FROM_ENV")
	projectID := os.Getenv("PROJECT_ID")
	subscriptionID := os.Getenv("SUBSCRIPTION_NAME")
	triggerCreds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

	if projectID != "" && subscriptionID != "" && triggerCreds != "" {
		connInfo := &GCPPubsubConnInfo{
			ProjectID:      projectID,
			SubscriptionID: subscriptionID,
			Creds:          triggerCreds,
		}
		return connInfo, nil
	}
	if subscriptionID != "" && creds != "" {
		connInfo := &GCPPubsubConnInfo{
			ProjectID:      projectID,
			SubscriptionID: subscriptionID,
			Creds:          creds,
		}
		return connInfo, nil
	}

	return nil, errors.New("Provide credentials ")

}
