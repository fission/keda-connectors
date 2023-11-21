package main

import (
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/stan.go"
)

func main() {
	nc, err := nats.Connect("nats://nats:4222")
	if err != nil {
		log.Fatal(err)
	}
	sc, err := stan.Connect("my-stan", "stan-sub", stan.NatsConn(nc))
	if err != nil {
		log.Fatal(err)
	}
	_, err = sc.Subscribe("response-topic", func(m *stan.Msg) {
		msg := string(m.Data)
		fmt.Println(msg)
	}, stan.DeliverAllAvailable())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("All messages consumed")
	select {}
}
