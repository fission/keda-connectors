package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
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
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithBaseEndpoint(endpoint),
	)
	if err != nil {
		log.Fatal("Error while loading config", err)
	}

	svc := sqs.NewFromConfig(cfg)

	msg := "Hello Msg"
	url := queueURL + "/my_queue"
	log.Println("Sending message to queue", url)
	_, err = svc.SendMessage(ctx, &sqs.SendMessageInput{
		DelaySeconds: 10,
		MessageBody:  &msg,
		QueueUrl:     &url,
	})
	if err != nil {
		log.Fatal("Error while sending message", err)
	}
	time.Sleep(5 * time.Second)
	urlRep := queueURL + "/responseTopic"
	log.Println("Receiving message from queue", urlRep)
	output, err := svc.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     5,
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
