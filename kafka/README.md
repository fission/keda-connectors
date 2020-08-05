# Kafka KEDA Connector

Kafka KEDA connector image can be used in the Kubernetes deployment as scaleTargetRef in scaledObject of [Apache Kafka scaler](https://keda.sh/docs/1.5/scalers/apache-kafka/).

The job of the connector is to read messages from the topic, request an endpoint, and write response or error in the respective topics. Following enviornment variables are used by connector image as configuration to connect and authenticate with Apache Kafka cluster which should be defined in the Kubernetes deployment manifest.

- `TOPIC`: topic from which messages are read
- `FUNCTION_URL`: http endpoint to post request
- `ERROR_TOPIC`: Optional. Topic to write errors on failure.
- `RESPONSE_TOPIC`: Optional. Topic to write responses on success response.
- `TRIGGER_NAME`: Name of the Trigger
- `MAX_RETRIES`: Maximum number of times an http endpoint will be retried upon failure.
- `CONTENT_TYPE`: Content type used while creating post request
- `BROKER_LIST`: comma separated list of Kafka brokers “hostname:port” to connect to for bootstrap (DEPRECATED).
- `BOOTSTRAP_SERVERS`: Comma separated list of Kafka brokers “hostname:port” to connect to for bootstrap.
- `CONSUMER_GROUP`: Kafka consumer group
- `AUTH_MODE`: Kafka sasl auth mode. Optional. The default value is none. For now, it must be one of none, sasl_plaintext, sasl_ssl, sasl_ssl_plain, sasl_scram_sha256, sasl_scram_sha512.
- `USERNAME`: Optional. If authmode is not none, this is required.
- `PASSWORD`: Optional. If authmode is not none, this is required.
- `CA`: Certificate authority file for TLS client authentication. Optional. If authmode is sasl_ssl, this is required.
- `CERT`: Certificate for client authentication. Optional. If authmode is sasl_ssl, this is required.
- `KEY`: Key for client authentication. Optional. If authmode is sasl_ssl, this is required.

More information about the above parameters and how to define it scaledobject refer [Apache Kafka scaler doc](https://keda.sh/docs/1.5/scalers/apache-kafka/).
