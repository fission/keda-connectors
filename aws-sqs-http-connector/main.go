package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"log"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/fission/keda-connectors/common"
)

type awsSQSConnector struct {
	sqsURL        string
	sqsClient     *sqs.SQS
	connectordata common.ConnectorMetadata
	logger        *zap.Logger
}

func (conn awsSQSConnector) consumeMessage() {
	headers := http.Header{
		"KEDA-Topic":          {conn.connectordata.Topic},
		"KEDA-Response-Topic": {conn.connectordata.ResponseTopic},
		"KEDA-Error-Topic":    {conn.connectordata.ErrorTopic},
		"Content-Type":        {conn.connectordata.ContentType},
		"KEDA-Source-Name":    {conn.connectordata.SourceName},
	}
	consQueueURL := conn.sqsURL + os.Getenv("TOPIC")
	for {
		output, err := conn.sqsClient.ReceiveMessage(&sqs.ReceiveMessageInput{
			QueueUrl:            &consQueueURL,
			MaxNumberOfMessages: aws.Int64(1),
			WaitTimeSeconds:     aws.Int64(5),
		})

		if err != nil {
			fmt.Printf("failed to fetch sqs message %v", err)
		}

		for _, message := range output.Messages {
			resp, err := common.HandleHTTPRequest(*message.Body, headers, conn.connectordata, conn.logger)
			if err != nil {
				conn.errorHandler(err)
				fmt.Println("Error")
			} else {
				defer resp.Body.Close()
				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					conn.errorHandler(err)
					fmt.Println("Error")
				} else {
					if success := conn.responseHandler(string(body)); success {
						conn.deleteMessage(*message.ReceiptHandle, consQueueURL)
						fmt.Println("Done")
					}
				}
			}
		}
	}
}

func (conn awsSQSConnector) responseHandler(response string) bool {
	respQueueURL := conn.sqsURL + os.Getenv("RESPONSE_TOPIC")
	if len(conn.connectordata.ResponseTopic) > 0 {
		_, err := conn.sqsClient.SendMessage(&sqs.SendMessageInput{
			DelaySeconds: aws.Int64(10),
			MessageAttributes: map[string]*sqs.MessageAttributeValue{
				"Title": &sqs.MessageAttributeValue{
					DataType:    aws.String("String"),
					StringValue: aws.String("The Whistler"),
				},
				"Author": &sqs.MessageAttributeValue{
					DataType:    aws.String("String"),
					StringValue: aws.String("John Grisham"),
				},
				"WeeksOn": &sqs.MessageAttributeValue{
					DataType:    aws.String("Number"),
					StringValue: aws.String("6"),
				},
			},
			MessageBody: &response,
			QueueUrl:    &respQueueURL,
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

func (conn *awsSQSConnector) errorHandler(err error) {
	errorQueueURL := conn.sqsURL + os.Getenv("ERROR_TOPIC")

	if len(conn.connectordata.ErrorTopic) > 0 {
		errMsg := err.Error()
		result, err := conn.sqsClient.SendMessage(&sqs.SendMessageInput{
			DelaySeconds: aws.Int64(10),
			MessageAttributes: map[string]*sqs.MessageAttributeValue{
				"Title": &sqs.MessageAttributeValue{
					DataType:    aws.String("String"),
					StringValue: aws.String("The Whistler"),
				},
				"Author": &sqs.MessageAttributeValue{
					DataType:    aws.String("String"),
					StringValue: aws.String("John Grisham"),
				},
				"WeeksOn": &sqs.MessageAttributeValue{
					DataType:    aws.String("Number"),
					StringValue: aws.String("6"),
				},
			},
			MessageBody: &errMsg,
			QueueUrl:    &errorQueueURL,
		})
		if err != nil {
			conn.logger.Error("failed to publish message to error topic",
				zap.Error(err),
				zap.String("source", conn.connectordata.SourceName),
				zap.String("message", err.Error()),
				zap.String("topic", conn.connectordata.ErrorTopic))
		}

		fmt.Println("Success", *result.MessageId)
	} else {
		conn.logger.Error("message received to publish to error topic, but no error topic was set",
			zap.String("message", err.Error()),
			zap.String("source", conn.connectordata.SourceName),
			zap.String("http endpoint", conn.connectordata.HTTPEndpoint),
		)
	}
}

func (conn *awsSQSConnector) deleteMessage(id string, queueURL string) {
	resultDelete, err := conn.sqsClient.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      &queueURL,
		ReceiptHandle: &id,
	})

	if err != nil {
		fmt.Println("Delete Error", err)
		return
	}

	fmt.Println("Message Deleted", resultDelete)
}

func main() {
	//TODO: this need to be removed
	err := godotenv.Load()
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()

	connectordata, err := common.ParseConnectorMetadata()

	url := "http://localhost:4576"
	sess, _ := session.NewSession(&aws.Config{
		Region:   aws.String("us-east-1"),
		Endpoint: &url,
	})
	svc := sqs.New(sess)
	conn := awsSQSConnector{
		sqsURL:        "http://localhost:4576/000000000000/",
		sqsClient:     svc,
		connectordata: connectordata,
		logger:        logger,
	}
	conn.consumeMessage()
}
