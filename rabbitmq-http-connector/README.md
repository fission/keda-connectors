Kafka KEDA connector image can be used in the Kubernetes deployment as scaleTargetRef in scaledObject of [RabbitMQ scaler](https://keda.sh/docs/1.5/scalers/rabbitmq-queue/).

The job of the connector is to read messages from the queue, call an HTTP endpoint with the body of the message, and write response or error in the respective queue. Following enviornment variables are used by connector image as configuration to connect and authenticate with RabbitMQ cluster which should be defined in the Kubernetes deployment manifest.

- `TOPIC`: Queue from which messages are read
- `HTTP_ENDPOINT`: http endpoint to post request
- `ERROR_TOPIC`: Optional. Queue to write errors on failure.
- `RESPONSE_TOPIC`: Optional. Queue to write responses on success response.
- `SOURCE_NAME`: Optional. Name of the Source. Default is "KEDAConnector"
- `MAX_RETRIES`: Maximum number of times an http endpoint will be retried upon failure.
- `CONTENT_TYPE`: Content type used while creating post request
- `HOST`: AMQP URI connection string, like `amqp://guest:password@localhost:5672/vhost`
- `CONCURRENT`: The maximum concurrent message to process at the same time.

More information about the above parameters and how to define it scaledobject refer [RabbitMQ scaler doc](https://keda.sh/docs/1.5/scalers/rabbitmq-queue/).
