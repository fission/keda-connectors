package main

import (
	"context"
	"fmt"
	"log"
	"os"
    "encoding/json"
    "time"
    "strconv"

	"github.com/go-redis/redis/v8"
)

type publish_data struct{
    Sid int `json:"sid"`
    Data string `json:"data"`
    Time int64 `json:"time"`
}


func main(){

    address := os.Getenv("REDIS_ADDRESS")
	if address == "" {
		log.Fatalf("Empty address field")
	}
	password := os.Getenv("REDIS_PASSWORD")

    var ctx = context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
	})

    for i := 0;i<10;i++{
        current_time := time.Now()
        secs := current_time.Unix()
        resp := publish_data{
            Sid: i,
            Data: "Message number: "+strconv.Itoa(i+1),
            Time: secs,
        }
        resp_json,_ := json.Marshal(resp)
        _, err := rdb.RPush(ctx,"test_queue",resp_json).Result()
        if err != nil{
            log.Fatalf("Error publishing messages",err)
            panic(err.Error())
        }
    }
    fmt.Println("Message publishing successfull!")
}
