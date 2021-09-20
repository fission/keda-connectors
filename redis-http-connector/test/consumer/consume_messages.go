package main

import (
	"context"
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
)

func main() {

	address := "redis-headless.ot-operators.svc.cluster.local:6379"
	password := ""

	var ctx = context.Background()
	var listItr int64
	rdb := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
	})

	listLength, err := rdb.LLen(ctx, "response-topic").Result()
	if err != nil {
		fmt.Println(rdb.Keys(ctx, "*"))
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
	fmt.Println("Messages consumed!")
}
