package main

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/fission/keda-connectors/common"
)

type awsSQSConnector struct {
	host          string
	connectordata common.ConnectorMetadata
	logger        *zap.Logger
}

func pollSqs(chn chan<- *sqs.Message, sess *session.Session) {
	queueURL := "http://localhost:4576/000000000000/my_queue"
	svc := sqs.New(sess)

	for {
		output, err := svc.ReceiveMessage(&sqs.ReceiveMessageInput{
			QueueUrl:            &queueURL,
			MaxNumberOfMessages: aws.Int64(1),
			WaitTimeSeconds:     aws.Int64(5),
		})

		if err != nil {
			fmt.Printf("failed to fetch sqs message %v", err)
		}

		for _, message := range output.Messages {
			chn <- message
		}

	}

}

func deleteMessage(rID string, sess *session.Session) {
	qURL := "http://localhost:4576/000000000000/my_queue"
	svc := sqs.New(sess)
	resultDelete, err := svc.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      &qURL,
		ReceiptHandle: &rID,
	})

	if err != nil {
		fmt.Println("Delete Error", err)
		return
	}

	fmt.Println("Message Deleted", resultDelete)
}

func main() {

	chnMessages := make(chan *sqs.Message, 1)
	url := "http://localhost:4576"
	sess, _ := session.NewSession(&aws.Config{
		Region:   aws.String("us-east-1"),
		Endpoint: &url,
	})
	go pollSqs(chnMessages, sess)

	fmt.Printf("Listening on stack queue: ")

	for message := range chnMessages {
		fmt.Println("Message Handle: " + *message.Body)
		// handleMessage(message)
		deleteMessage(*message.ReceiptHandle, sess)
	}
}
