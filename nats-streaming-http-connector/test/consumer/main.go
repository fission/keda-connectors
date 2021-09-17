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
	sub, err := sc.QueueSubscribe("response-topic", "grp1", func(m *stan.Msg) {
		msg := string(m.Data)
		fmt.Println(msg)
	}, stan.DurableName("ImDurable"), stan.DeliverAllAvailable())
	if err != nil {
		log.Fatal(err)
	}
	err = sub.Unsubscribe()
	if err != nil {
		log.Fatal(err)
	}
	sc.Close()
	fmt.Println("All messages consumed")
	select {}
}
