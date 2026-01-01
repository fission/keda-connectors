package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/fission/keda-connectors/common"
)

// comment
func main() {
	queueURL := os.Getenv("AWS_SQS_URL")
	if queueURL == "" {
		log.Fatal("AWS_SQS_URL is not set")
	}

	config, err := common.GetAwsConfig(context.TODO())
	if err != nil {
		log.Fatal("Error while getting AWS config", err)
	}
	svc := sqs.NewFromConfig(config)

	msg := "Hello Msg"
	url := queueURL + "/my_queue"
	log.Println("Sending message to queue", url)
	_, err = svc.SendMessage(context.TODO(), &sqs.SendMessageInput{
		DelaySeconds: *aws.Int32(10),
		MessageBody:  &msg,
		QueueUrl:     &url,
	})
	if err != nil {
		log.Fatal("Error while sending message", err)
	}
	time.Sleep(5 * time.Second)
	urlRep := queueURL + "/responseTopic"
	log.Println("Receiving message from queue", urlRep)
	var maxNumberOfMessages = int32(1)
	var waitTimeSeconds = int32(5)
	output, err := svc.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
		MaxNumberOfMessages: maxNumberOfMessages,
		WaitTimeSeconds:     waitTimeSeconds,
		QueueUrl:            &urlRep,
	})
	if err != nil {
		fmt.Printf("Error in processing message: %s", err)
	}

	for _, message := range output.Messages {
		if *message.Body != "Hello Msg" {
			fmt.Printf("Expected : Hello Msg, Got :%s ", *message.Body)
		} else {
			fmt.Printf("Done processing message %s", *message.Body)
			time.Sleep(30 * time.Second)
		}
	}
}
