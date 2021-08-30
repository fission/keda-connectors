package main

import (
	"context"
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
)

func main() {

	/*address := os.Getenv("REDIS_ADDRESS")
	if address == "" {
		log.Fatalf("Empty address field")
	}
	password := os.Getenv("REDIS_PASSWORD")*/
	address := "127.0.0.1:6379"
	password := ""

	var ctx = context.Background()
	var i int64
	rdb := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
	})

	list_len, _ := rdb.LLen(ctx, "response-topic").Result()

	for i = 0; i < list_len; i++ {
		msg, err := rdb.LPop(ctx, "response-topic").Result()

		if err != nil {
			log.Fatalf("Error in consuming queue: %v", err)
			panic(err.Error())
		}
		fmt.Println(msg)
	}

}
