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
	sc, err := stan.Connect("test-cluster", "stan-sub", stan.NatsConn(nc))
	if err != nil {
		log.Fatal(err)
	}
	_, err = sc.QueueSubscribe("response-topic", "grp1", func(m *stan.Msg) {
		msg := string(m.Data)
		log.Printf("%v", msg)
	}, stan.DurableName("ImDurable"), stan.DeliverAllAvailable())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("All messages consumed")
}
