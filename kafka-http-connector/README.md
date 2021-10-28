# Kafka KEDA Connector

Kafka KEDA connector image can be used in the Kubernetes deployment as scaleTargetRef in scaledObject of [Apache Kafka scaler](https://keda.sh/docs/2.4/scalers/apache-kafka/).

The job of the connector is to read messages from the topic, call an HTTP endpoint with the body of the message, and write response or error in the respective topics.
Following environment variables are used by connector image as configuration to connect and authenticate with Apache Kafka cluster which should be defined in the Kubernetes deployment manifest.

- `TOPIC`: topic from which messages are read.
- `HTTP_ENDPOINT`: http endpoint to post request.
- `ERROR_TOPIC`: Optional. Topic to write errors on failure.
- `RESPONSE_TOPIC`: Optional. Topic to write responses on success response.
- `SOURCE_NAME`: Optional. Name of the Source. Default is "KEDAConnector".
- `MAX_RETRIES`: Maximum number of times an http endpoint will be retried upon failure.
- `CONTENT_TYPE`: Content type used while creating post request.
- `BROKER_LIST`: comma separated list of Kafka brokers “hostname:port” to connect to for bootstrap (DEPRECATED).
- `BOOTSTRAP_SERVERS`: Comma separated list of Kafka brokers “hostname:port” to connect to for bootstrap.
- `CONSUMER_GROUP`: Kafka consumer group.
- `SASL`: Kafka sasl auth mode. Optional. The default value is none. For now, it must be one of none, plaintext, sasl_ssl, scram_sha256, scram_sha512.
- `USERNAME`: Optional. If authmode is not none, this is required.
- `PASSWORD`: Optional. If authmode is not none, this is required.
- `TLS`: To enable SSL auth for Kafka, set this to enable. If not set, TLS for Kafka is not used. (Optional)
- `CA`: Certificate authority file for TLS client authentication. (Optional)
- `CERT`: Certificate for client authentication. (Optional)
- `KEY`: Key for client authentication. (Optional)

More information about the above parameters and how to define it scaledobject refer [Apache Kafka scaler doc](https://keda.sh/docs/1.5/scalers/apache-kafka/).
