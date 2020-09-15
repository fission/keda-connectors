//TODO: check for new shards if get added
//Records need to be passed
//Read number of records

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
)

var (
	stream          = flag.String("stream", "test", "Stream name")
	kinesisEndpoint = flag.String("endpoint", "http://localhost:4568", "Kinesis endpoint")
	awsRegion       = flag.String("region", "us-east-1", "AWS Region")
)

func listShards(kc *kinesis.Kinesis, streamName *string) ([]*kinesis.Shard, error) {

	stream, err := kc.DescribeStream(&kinesis.DescribeStreamInput{StreamName: streamName})
	if err != nil {
		fmt.Printf("received  err %v", err)
		return nil, err
	}
	return stream.StreamDescription.Shards, nil

}

//TODO: need to check about locks
func findNewShards(ctx context.Context, kc *kinesis.Kinesis, streamName *string, shardc chan *kinesis.Shard) {
	shards := make(map[string]*kinesis.Shard)

	var ticker = time.NewTicker(30 * time.Second)

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("Done close")
			ticker.Stop()
			return
		case <-ticker.C:
			shardList, err := listShards(kc, streamName)

			if err != nil {
				return
			}

			for _, s := range shardList {
				if _, ok := shards[*s.ShardId]; ok {
					continue
				}
				shards[*s.ShardId] = s
				shardc <- s
			}
		}
	}

}

func bytesToString(b []byte) string {
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh := reflect.StringHeader{bh.Data, bh.Len}
	return *(*string)(unsafe.Pointer(&sh))
}

func getIterator(ctx context.Context, kc *kinesis.Kinesis, streamName string, shardID string,
	checkpoint string) (*kinesis.GetShardIteratorOutput, error) {

	if checkpoint != "" {
		iteratorOutput, err := kc.GetShardIteratorWithContext(aws.Context(ctx), &kinesis.GetShardIteratorInput{
			ShardId:                &shardID,
			ShardIteratorType:      aws.String("AFTER_SEQUENCE_NUMBER"),
			StartingSequenceNumber: aws.String(checkpoint),
			StreamName:             &streamName,
		})
		if err != nil {
			return nil, err
		}
		return iteratorOutput, err
	}
	iteratorOutput, err := kc.GetShardIterator(&kinesis.GetShardIteratorInput{
		ShardId:           &shardID,
		ShardIteratorType: aws.String("TRIM_HORIZON"), //oldest data record in the shard
		StreamName:        &streamName,
	})
	if err != nil {
		return nil, err
	}
	return iteratorOutput, err
}

func getRecords(kc *kinesis.Kinesis, shardIterator *string) (*kinesis.GetRecordsOutput, error) {
	// get records use shard iterator for making request
	records, err := kc.GetRecords(&kinesis.GetRecordsInput{
		ShardIterator: shardIterator,
		Limit:         aws.Int64(1),
	})
	if err != nil {
		return nil, err
	}

	return records, nil
}
func isShardClosed(nextShardIterator, currentShardIterator *string) bool {
	return nextShardIterator == nil || currentShardIterator == nextShardIterator
}
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	flag.Parse()

	s, _ := session.NewSession(
		aws.NewConfig().
			WithEndpoint(*kinesisEndpoint).
			WithRegion(*awsRegion),
	)
	kc := kinesis.New(s)

	streamName := aws.String(*stream)

	if err := kc.WaitUntilStreamExists(&kinesis.DescribeStreamInput{StreamName: streamName}); err != nil {
		<-ctx.Done()
	}

	shardc := make(chan *kinesis.Shard, 1)

	go func() {
		findNewShards(ctx, kc, streamName, shardc)
		<-ctx.Done()
		close(shardc)
	}()

	checkpoints := make(map[string]string)
	deletedShards := make(map[string]bool)
	var wg sync.WaitGroup
	for s := range shardc {
		checkpoints[*s.ShardId] = ""
		wg.Add(1)
		go func(shardId string, cp map[string]string) {
			defer wg.Done()
			for {
				if deletedShards[shardId] {
					return
				}
				iteratorOutput, err := getIterator(ctx, kc, *streamName, shardId, cp[shardId])
				if err != nil {
					fmt.Printf("Error in iterator : %s : %s", shardId, err)
					return
				}
				iterator := iteratorOutput.ShardIterator
				if iterator != nil {
					records, err := getRecords(kc, iterator)
					if err != nil {
						fmt.Printf("Shard is closed : %s : %s", shardId, err)
						return
					}

					for _, v := range records.Records {
						fmt.Println(shardId + ":" + bytesToString(v.Data))
						cp[shardId] = *v.SequenceNumber
					}
					if isShardClosed(records.NextShardIterator, iterator) {
						if _, found := cp[shardId]; found {
							fmt.Printf("Closed Shard : %s : %s\n", shardId, err)
							fmt.Printf("found Shards : %s \n", cp)
							deletedShards[shardId] = true
							delete(cp, shardId)
							fmt.Printf("after delete Shards : %s \n", cp)
							return
						}
					}
				}
			}

		}(*s.ShardId, checkpoints)
	}
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-ctx.Done():
		fmt.Println("terminating: context cancelled")
	case <-sigterm:
		fmt.Println("terminating: via signal")
	}
	cancel()
	wg.Wait()
}
