package main

import (
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
)

//Test
func main() {
	queueURL := "http://localstack:31000/queue/"
	region := "us-east-1"
	endpoint := "http://localstack:31000"
	config := &aws.Config{
		Region:   &region,
		Endpoint: &endpoint,
	}

	sess, err := session.NewSession(config)
	if err != nil {
		log.Panic("Error while creating session")
	}
	svc := sqs.New(sess)

	msg := fmt.Sprintf("Hello Msg")
	url := queueURL + "my_queue"
	_, err = svc.SendMessage(&sqs.SendMessageInput{
		DelaySeconds: aws.Int64(10),
		MessageBody:  &msg,
		QueueUrl:     &url,
	})
	time.Sleep(5 * time.Second)
	urlRep := queueURL + "responseTopic"
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
