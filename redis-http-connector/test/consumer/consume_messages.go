package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/go-redis/redis/v8"
)

func main() {

	address := os.Getenv("REDIS_ADDRESS")
	if address == "" {
		log.Fatalf("Empty address field")
	}
	password := os.Getenv("REDIS_PASSWORD")

	var ctx = context.Background()
	var listItr int64
	rdb := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
	})

	listLength, err := rdb.LLen(ctx, "response-topic").Result()
	if err != nil {
		log.Fatalf("Error in consuming queue: %v", err)
	}

	for listItr = 0; listItr < listLength; listItr++ {
		msg, err := rdb.LPop(ctx, "response-topic").Result()

		if err != nil {
			log.Fatalf("Error in consuming queue: %v", err)
			panic(err.Error())
		}
		fmt.Println(msg)
	}

}
