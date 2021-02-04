module github.com/fission/keda-connectors/gcp-pubsub-http-connector

go 1.15

require (
	cloud.google.com/go/pubsub v1.8.1
	github.com/fission/keda-connectors/common v0.0.0-20201028072024-da094250dc08
	go.uber.org/zap v1.16.0
	google.golang.org/api v0.32.0
)
