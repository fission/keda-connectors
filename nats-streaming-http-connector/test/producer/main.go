package main

import (
	"log"
	"strconv"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/stan.go"
)

func main() {
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		log.Fatal(err)
	}
	sc, err := stan.Connect("test-cluster", "stan-sub", stan.NatsConn(nc))
	if err != nil {
		log.Fatal(err)
	}
	for i := 100; i < 102; i++ {
		sc.Publish("hello", []byte("Test"+strconv.Itoa(i)))
	}

	// sc.QueueSubscribe("response", "grp1", func(m *stan.Msg) {
	// 	log.Printf("[Received] %+v", m)
	// }, stan.DurableName("due"), stan.DeliverAllAvailable())

	select {}
}
