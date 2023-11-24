package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
)

// comment
func main() {
	queueURL := os.Getenv("AWS_SQS_URL")
	if queueURL == "" {
		log.Fatal("AWS_SQS_URL is not set")
	}
	region := os.Getenv("AWS_REGION")
	if region == "" {
		log.Fatal("AWS_REGION is not set")
	}
	endpoint := os.Getenv("AWS_ENDPOINT")
	if endpoint == "" {
		log.Fatal("AWS_ENDPOINT is not set")
	}
	config := &aws.Config{
		Region:   &region,
		Endpoint: &endpoint,
	}

	sess, err := session.NewSession(config)
	if err != nil {
		log.Fatal("Error while creating session", err)
	}
	svc := sqs.New(sess)

	msg := "Hello Msg"
	url := queueURL + "/my_queue"
	log.Println("Sending message to queue", url)
	_, err = svc.SendMessage(&sqs.SendMessageInput{
		DelaySeconds: aws.Int64(10),
		MessageBody:  &msg,
		QueueUrl:     &url,
	})
	if err != nil {
		log.Fatal("Error while sending message", err)
	}
	time.Sleep(5 * time.Second)
	urlRep := queueURL + "/responseTopic"
	log.Println("Receiving message from queue", urlRep)
	var maxNumberOfMessages = int64(1)
	var waitTimeSeconds = int64(5)
	output, err := svc.ReceiveMessage(&sqs.ReceiveMessageInput{
		MaxNumberOfMessages: &maxNumberOfMessages,
		WaitTimeSeconds:     &waitTimeSeconds,
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
