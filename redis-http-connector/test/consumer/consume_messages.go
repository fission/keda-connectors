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
	rdb := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
	})

	listLength, err := rdb.LLen(ctx, "response-topic").Result()
	if err != nil {
		log.Fatalf("Error in consuming queue: %v", err)
	}

	for range listLength {
		msg, err := rdb.LPop(ctx, "response-topic").Result()

		if err != nil {
			log.Fatalf("Error in consuming queue: %v", err)
			panic(err.Error())
		}
		fmt.Println(msg)
	}
	fmt.Println("Messages consumed!")
	select {}
}
