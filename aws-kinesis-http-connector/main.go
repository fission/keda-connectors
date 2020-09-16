package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/fission/keda-connectors/awsutil"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/fission/keda-connectors/common"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type pullFunc func(*record) error
type record struct {
	*kinesis.Record
	shardID            string
	millisBehindLatest *int64
}
type awsKinesisConnector struct {
	streamName    *string
	ctx           context.Context
	client        *kinesis.Kinesis
	connectordata common.ConnectorMetadata
	logger        *zap.Logger
	shardc        chan *kinesis.Shard
}

func listShards(kc *kinesis.Kinesis, streamName *string) ([]*kinesis.Shard, error) {
	stream, err := kc.DescribeStream(&kinesis.DescribeStreamInput{StreamName: streamName})
	if err != nil {
		fmt.Printf("\nreceived  err %v", err)
		return nil, err
	}
	return stream.StreamDescription.Shards, nil
}

//TODO: need to check about locks
func (conn *awsKinesisConnector) findNewShards() {
	shards := make(map[string]*kinesis.Shard)

	var ticker = time.NewTicker(30 * time.Second)
	for {
		select {
		case <-conn.ctx.Done():
			fmt.Println("Done close")
			ticker.Stop()
			return
		case <-ticker.C:
			shardList, err := listShards(conn.client, conn.streamName)

			if err != nil {
				return
			}

			for _, s := range shardList {
				if _, ok := shards[*s.ShardId]; ok {
					continue
				}
				shards[*s.ShardId] = s
				conn.shardc <- s
			}
		}
	}

}

func (conn *awsKinesisConnector) getIterator(shardID string, checkpoint string) (*kinesis.GetShardIteratorOutput, error) {

	if checkpoint != "" {
		iteratorOutput, err := conn.client.GetShardIteratorWithContext(aws.Context(conn.ctx), &kinesis.GetShardIteratorInput{
			ShardId:                &shardID,
			ShardIteratorType:      aws.String(kinesis.ShardIteratorTypeAfterSequenceNumber),
			StartingSequenceNumber: aws.String(checkpoint),
			StreamName:             conn.streamName,
		})
		if err != nil {
			return nil, err
		}
		return iteratorOutput, err
	}
	iteratorOutput, err := conn.client.GetShardIterator(&kinesis.GetShardIteratorInput{
		ShardId:           &shardID,
		ShardIteratorType: aws.String(kinesis.ShardIteratorTypeTrimHorizon), //oldest data record in the shard
		StreamName:        conn.streamName,
	})
	if err != nil {
		return nil, err
	}
	return iteratorOutput, err
}

func (conn *awsKinesisConnector) getRecords(shardIterator *string) (*kinesis.GetRecordsOutput, error) {
	// get records use shard iterator for making request
	records, err := conn.client.GetRecords(&kinesis.GetRecordsInput{
		ShardIterator: shardIterator,
		Limit:         aws.Int64(3),
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

func isShardClosed(nextShardIterator, currentShardIterator *string) bool {
	return nextShardIterator == nil || currentShardIterator == nextShardIterator
}

func (conn *awsKinesisConnector) pullRecords(fn pullFunc) {
	checkpoints := make(map[string]string)
	var wg sync.WaitGroup
	for s := range conn.shardc {
		checkpoints[*s.ShardId] = ""
		wg.Add(1)
		go func(shardID string) {
			defer wg.Done()
			scanTicker := time.NewTicker(1000)
			defer scanTicker.Stop()
			for {
				if _, found := checkpoints[shardID]; !found {
					return
				}
				iteratorOutput, err := conn.getIterator(shardID, checkpoints[shardID])
				if err != nil {
					fmt.Printf("Error in iterator : %s : %s", shardID, err)
					return
				}
				iterator := iteratorOutput.ShardIterator
				if iterator != nil {
					resp, err := conn.getRecords(iterator)
					if err != nil {
						fmt.Printf("Shard is closed : %s : %s", shardID, err)
						return
					}

					for _, r := range resp.Records {
						err := fn(&record{r, shardID, resp.MillisBehindLatest})
						checkpoints[shardID] = *r.SequenceNumber
						if err != nil {
							fmt.Printf("Error in processing records : %s : %s", shardID, err)
						}
					}
					if isShardClosed(resp.NextShardIterator, iterator) {
						if _, found := checkpoints[shardID]; found {
							delete(checkpoints, shardID)
							return
						}
					}
				}
				select {
				case <-conn.ctx.Done():
					return
				case <-scanTicker.C:
					continue
				}
			}

		}(*s.ShardId)
	}
	wg.Wait()
}

func (conn *awsKinesisConnector) consumeMessage(r *record) {
	headers := http.Header{
		"KEDA-Topic":          {conn.connectordata.Topic},
		"KEDA-Response-Topic": {conn.connectordata.ResponseTopic},
		"KEDA-Error-Topic":    {conn.connectordata.ErrorTopic},
		"Content-Type":        {conn.connectordata.ContentType},
		"KEDA-Source-Name":    {conn.connectordata.SourceName},
	}

	resp, err := common.HandleHTTPRequest(string(r.Data), headers, conn.connectordata, conn.logger)
	if err != nil {
		conn.errorHandler(r, err.Error())
	} else {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			conn.errorHandler(r, err.Error())
		} else {
			if success := conn.responseHandler(r, string(body)); success {
				fmt.Printf("Done processing message for shard : %s with message %s", r.shardID, string(body))
			}
		}
	}
}

func (conn *awsKinesisConnector) responseHandler(r *record, response string) bool {
	if len(conn.connectordata.ResponseTopic) > 0 {
		params := &kinesis.PutRecordInput{
			Data:                      []byte(response),                        // Required
			PartitionKey:              aws.String(*r.PartitionKey),             // Required
			StreamName:                aws.String(os.Getenv("RESPONSE_TOPIC")), // Required
			SequenceNumberForOrdering: aws.String(*r.SequenceNumber),
		}

		_, err := conn.client.PutRecord(params)
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

func (conn *awsKinesisConnector) errorHandler(r *record, errMsg string) {
	if len(conn.connectordata.ErrorTopic) > 0 {
		params := &kinesis.PutRecordInput{
			Data:                      []byte(errMsg),                       // Required
			PartitionKey:              aws.String(*r.PartitionKey),          // Required
			StreamName:                aws.String(os.Getenv("ERROR_TOPIC")), // Required
			SequenceNumberForOrdering: aws.String(*r.SequenceNumber),
		}

		_, err := conn.client.PutRecord(params)
		if err != nil {
			conn.logger.Error("failed to publish message to error topic",
				zap.Error(err),
				zap.String("source", conn.connectordata.SourceName),
				zap.String("message", err.Error()),
				zap.String("topic", conn.connectordata.ErrorTopic))
		}
	} else {
		conn.logger.Error("message received to publish to error topic, but no error topic was set",
			zap.String("message", errMsg),
			zap.String("source", conn.connectordata.SourceName),
			zap.String("http endpoint", conn.connectordata.HTTPEndpoint),
		)
	}
}

func main() {
	//TODO: need to remove this
	err := godotenv.Load()

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config, err := awsutil.GetAwsConfig()
	if err != nil {
		logger.Error("failed to fetch aws config", zap.Error(err))
		return
	}

	s, err := session.NewSession(config)
	kc := kinesis.New(s)

	if err != nil {
		fmt.Println("Not able to create the session")
	}
	connectordata, err := common.ParseConnectorMetadata()
	if err != nil {
		fmt.Println("error while parsing metadata")
	}
	if err := kc.WaitUntilStreamExists(&kinesis.DescribeStreamInput{StreamName: &connectordata.Topic}); err != nil {
		fmt.Println("Not able to connect to kinesis stream")
		cancel()
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	go func() {
		<-signals
		cancel() // call cancellation
	}()

	shardc := make(chan *kinesis.Shard, 1)
	conn := awsKinesisConnector{
		ctx:           ctx,
		client:        kc,
		connectordata: connectordata,
		logger:        logger,
		shardc:        shardc,
	}
	go func() {
		conn.findNewShards()
		fmt.Println("canceling findNewShards")
		cancel()
		close(shardc)
	}()

	conn.pullRecords(func(r *record) error {
		fmt.Println(r.shardID + ":" + string(r.Data))
		conn.consumeMessage(r)
		return nil // continue pulling
	})
	fmt.Println("Done terminating => via signal")
	cancel()
}
