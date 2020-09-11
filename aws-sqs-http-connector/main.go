package main

import (
	"errors"
	"io/ioutil"
	"log"

	"net/http"
	"os"

	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
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
	var maxNumberOfMessages = int64(10) // Process maximum 10 messages concurrently
	var waitTimeSeconds = int64(5)      //Wait 5 sec to process another message

	headers := http.Header{
		"KEDA-Topic":          {conn.connectordata.Topic},
		"KEDA-Response-Topic": {conn.connectordata.ResponseTopic},
		"KEDA-Error-Topic":    {conn.connectordata.ErrorTopic},
		"Content-Type":        {conn.connectordata.ContentType},
		"KEDA-Source-Name":    {conn.connectordata.SourceName},
	}

	consQueueURL := conn.sqsURL + os.Getenv("TOPIC")
	respQueueURL := conn.sqsURL + os.Getenv("RESPONSE_TOPIC")
	errorQueueURL := conn.sqsURL + os.Getenv("ERROR_TOPIC")

	for {
		output, err := conn.sqsClient.ReceiveMessage(&sqs.ReceiveMessageInput{
			QueueUrl:            &consQueueURL,
			MaxNumberOfMessages: &maxNumberOfMessages,
			WaitTimeSeconds:     &waitTimeSeconds,
		})

		if err != nil {
			conn.logger.Error("failed to fetch sqs message", zap.Error(err))
		}

		for _, message := range output.Messages {
			resp, err := common.HandleHTTPRequest(*message.Body, headers, conn.connectordata, conn.logger)
			if err != nil {
				conn.errorHandler(errorQueueURL, err)
			} else {
				defer resp.Body.Close()
				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					conn.errorHandler(errorQueueURL, err)
				} else {
					//Generating SQS Message attribute
					var sqsMessageAttValue = make(map[string]*sqs.MessageAttributeValue)
					for k, v := range resp.Header {
						for _, d := range v {
							sqsMessageAttValue[k] = &sqs.MessageAttributeValue{
								DataType:    aws.String("String"),
								StringValue: aws.String(d),
							}
						}
					}
					if success := conn.responseHandler(respQueueURL, string(body), sqsMessageAttValue); success {
						conn.deleteMessage(*message.ReceiptHandle, consQueueURL)
					}
				}
			}
		}
	}
}

func (conn awsSQSConnector) responseHandler(queueURL string, response string, messageAttValue map[string]*sqs.MessageAttributeValue) bool {
	if queueURL != "" {
		_, err := conn.sqsClient.SendMessage(&sqs.SendMessageInput{
			DelaySeconds:      aws.Int64(10),
			MessageAttributes: messageAttValue,
			MessageBody:       &response,
			QueueUrl:          &queueURL,
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

func (conn *awsSQSConnector) errorHandler(queueURL string, err error) {
	if queueURL != "" {
		errMsg := err.Error()
		_, err := conn.sqsClient.SendMessage(&sqs.SendMessageInput{
			DelaySeconds: aws.Int64(10),
			//MessageAttributes: messageAttValue,
			MessageBody: &errMsg,
			QueueUrl:    &queueURL,
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

func (conn *awsSQSConnector) deleteMessage(id string, queueURL string) {
	_, err := conn.sqsClient.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      &queueURL,
		ReceiptHandle: &id,
	})

	if err != nil {
		conn.logger.Error("Delete Error", zap.Error(err))
		return
	}

	conn.logger.Info("Message Deleted")
}

func getAwsConfig() (*aws.Config, error) {
	if os.Getenv("AWS_REGION") == "" {
		return nil, errors.New("aws region required")
	}
	config := &aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION")),
	}
	if os.Getenv("AWS_ENDPOINT") != "" {
		endpoint := os.Getenv("AWS_ENDPOINT")
		config.Endpoint = &endpoint
		return config, nil
	}
	if os.Getenv("AWS_ACCESS_KEY") != "" && os.Getenv("AWS_SECRET_KEY") != "" {
		config.Credentials = credentials.NewStaticCredentials(os.Getenv("AWS_ACCESS_KEY"), os.Getenv("AWS_SECRET_KEY"), "")
		return config, nil
	}
	if os.Getenv("AWS_CRED_PATH") != "" && os.Getenv("AWS_CRED_PROFILE") != "" {
		config.Credentials = credentials.NewSharedCredentials(os.Getenv("AWS_CRED_PATH"), os.Getenv("AWS_CRED_PROFILE"))
		return config, nil
	}
	return nil, errors.New("no aws configuration specified")
}

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()

	connectordata, err := common.ParseConnectorMetadata()

	config, err := getAwsConfig()
	if err != nil {
		logger.Error("Failed to fetch aws config", zap.Error(err))
		return
	}

	sess, _ := session.NewSession(config)
	svc := sqs.New(sess)
	conn := awsSQSConnector{
		sqsURL:        os.Getenv("AWS_SQS_URL"),
		sqsClient:     svc,
		connectordata: connectordata,
		logger:        logger,
	}
	conn.consumeMessage()
}
