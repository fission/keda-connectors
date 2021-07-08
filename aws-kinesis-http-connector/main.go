package main

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/fission/keda-connectors/common"

	"go.uber.org/zap"
)

type pullFunc func(*record) error
type record struct {
	*kinesis.Record
	shardID            string
	millisBehindLatest *int64
}
type awsKinesisConnector struct {
	ctx           context.Context
	client        *kinesis.Kinesis
	connectordata common.ConnectorMetadata
	logger        *zap.Logger
	shardc        chan *kinesis.Shard
	maxRecords    int64
}

//listShards get called every 30sec to get all the shards
func (conn *awsKinesisConnector) listShards() ([]*kinesis.Shard, error) {
	//call DescribeStream to get updated shards
	stream, err := conn.client.DescribeStream(&kinesis.DescribeStreamInput{
		StreamName: &conn.connectordata.Topic,
	})
	if err != nil {
		return nil, err
	}
	return stream.StreamDescription.Shards, nil
}

//findNewShards sends shards, it only sends newly added shards
func (conn *awsKinesisConnector) findNewShards() {
	var shards sync.Map
	var ticker = time.NewTicker(30 * time.Second)
	for {
		select {
		case <-conn.ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			//check if new shards are available in every 30 seconds
			shardList, err := conn.listShards()
			if err != nil {
				return
			}

			for _, s := range shardList {
				//send only new shards
				_, loaded := shards.LoadOrStore(*s.ShardId, s)
				if !loaded {
					conn.shardc <- s
				}
			}
		}
	}
}

//getIterator get's the iterator either from start or from where we left
func (conn *awsKinesisConnector) getIterator(shardID string, checkpoint string) (*kinesis.GetShardIteratorOutput, error) {
	params := &kinesis.GetShardIteratorInput{
		ShardId:    &shardID,
		StreamName: &conn.connectordata.Topic,
	}

	if checkpoint != "" {
		//Start from, where we left
		params.StartingSequenceNumber = aws.String(checkpoint)
		params.ShardIteratorType = aws.String(kinesis.ShardIteratorTypeAfterSequenceNumber)
		iteratorOutput, err := conn.client.GetShardIteratorWithContext(conn.ctx, params)
		if err != nil {
			return nil, err
		}
		return iteratorOutput, err
	}
	//Start from, oldest record in the shard
	params.ShardIteratorType = aws.String(kinesis.ShardIteratorTypeTrimHorizon)
	iteratorOutput, err := conn.client.GetShardIteratorWithContext(conn.ctx, params)
	if err != nil {
		return nil, err
	}
	return iteratorOutput, err
}

//getRecords get the data for the specific shard
func (conn *awsKinesisConnector) getRecords(shardIterator *string) (*kinesis.GetRecordsOutput, error) {
	// get records use shard iterator for making request
	records, err := conn.client.GetRecords(&kinesis.GetRecordsInput{
		ShardIterator: shardIterator,
		Limit:         &conn.maxRecords,
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

//Check if shards are closed, shards can be updated by using update-shard-count method
func isShardClosed(nextShardIterator, currentShardIterator *string) bool {
	//No new iterator is present, means it is closed
	return nextShardIterator == nil || currentShardIterator == nextShardIterator
}

//scan each shards for any new records, when found call the passed func
func (conn *awsKinesisConnector) pullRecords(fn pullFunc) {
	//checkpoints to identify how much read has happened
	var checkpoints sync.Map
	var wg sync.WaitGroup
	//get called when any new shards are added
	for s := range conn.shardc {
		//Start fresh
		checkpoints.Store(*s.ShardId, "")
		wg.Add(1)
		go func(shardID string) {
			defer wg.Done()
			//scan every 10 second
			scanTicker := time.NewTicker(10 * time.Second)
			defer scanTicker.Stop()
			for {
				//do noting if shard got deleted
				checkpoint, found := checkpoints.Load(shardID)
				if !found {
					conn.logger.Info("shard not found", zap.String("shardID", shardID))
					return
				}
				iteratorOutput, err := conn.getIterator(shardID, checkpoint.(string))
				if err != nil {
					conn.logger.Error("error in iterator",
						zap.String("shardID", shardID),
						zap.Error(err))
					return
				}
				iterator := iteratorOutput.ShardIterator
				if iterator != nil {
					resp, err := conn.getRecords(iterator)
					if err != nil {
						conn.logger.Error("error in getting records",
							zap.String("shardID", shardID),
							zap.Error(err))
						return
					}

					for _, r := range resp.Records {
						//send records
						err := fn(&record{r, shardID, resp.MillisBehindLatest})
						checkpoints.Store(shardID, *r.SequenceNumber)
						if err != nil {
							conn.logger.Error("error in processing records",
								zap.String("shardID", shardID),
								zap.Error(err))
						}
					}
					if isShardClosed(resp.NextShardIterator, iterator) {
						//when shards got deleted, remove it from checkpoints
						if _, found := checkpoints.Load(shardID); found {
							checkpoints.Delete(shardID)
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
		conn.logger.Error("error processing message",
			zap.String("shardID", r.shardID),
			zap.Error(err))
		conn.errorHandler(r, err.Error())
	} else {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			conn.logger.Error("error processing message",
				zap.String("shardID", r.shardID),
				zap.Error(err))
			conn.errorHandler(r, err.Error())
		} else {
			if err := conn.responseHandler(r, string(body)); err != nil {
				conn.logger.Error("failed to publish response body from http request to topic",
					zap.Error(err),
					zap.String("topic", conn.connectordata.ResponseTopic),
					zap.String("source", conn.connectordata.SourceName),
					zap.String("http endpoint", conn.connectordata.HTTPEndpoint))
			}
			conn.logger.Info("done processing message",
				zap.String("shardID", r.shardID),
				zap.String("message", string(body)))
		}
	}
}

func (conn *awsKinesisConnector) responseHandler(r *record, response string) error {
	if len(conn.connectordata.ResponseTopic) > 0 {
		params := &kinesis.PutRecordInput{
			Data:                      []byte(response),                             // Required
			PartitionKey:              aws.String(*r.PartitionKey),                  // Required
			StreamName:                aws.String(conn.connectordata.ResponseTopic), // Required
			SequenceNumberForOrdering: aws.String(*r.SequenceNumber),
		}
		_, err := conn.client.PutRecord(params)
		if err != nil {
			return err
		}
	}
	return nil
}

func (conn *awsKinesisConnector) errorHandler(r *record, errMsg string) {
	if len(conn.connectordata.ErrorTopic) > 0 {
		params := &kinesis.PutRecordInput{
			Data:                      []byte(errMsg),                            // Required
			PartitionKey:              aws.String(*r.PartitionKey),               // Required
			StreamName:                aws.String(conn.connectordata.ErrorTopic), // Required
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
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config, err := common.GetAwsConfig()
	if err != nil {
		logger.Error("failed to fetch aws config", zap.Error(err))
		return
	}

	s, err := session.NewSession(config)
	if err != nil {
		logger.Error("not able to create the session", zap.Error(err))
		return
	}
	kc := kinesis.New(s)
	connectordata, err := common.ParseConnectorMetadata()
	if err != nil {
		logger.Error("error while parsing metadata", zap.Error(err))
		return
	}
	if err := kc.WaitUntilStreamExists(&kinesis.DescribeStreamInput{StreamName: &connectordata.Topic}); err != nil {
		logger.Error("not able to connect to kinesis stream", zap.Error(err))
		return
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
		maxRecords:    10, //Read maximum 10 records
	}
	logger.Info("Starting aws kinesis connector")
	//Get the shards in shardc chan
	go func() {
		conn.findNewShards()
		cancel()
		close(shardc)
	}()

	conn.pullRecords(func(r *record) error {
		conn.consumeMessage(r)
		return nil // continue pulling
	})
	cancel()
}
