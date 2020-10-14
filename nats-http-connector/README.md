NATs KEDA connector image can be used in the Kubernetes deployment as scaleTargetRef in scaledObject of [NATs scaler](https://keda.sh/docs/1.5/scalers/nats-streaming/).

The job of the connector is to read messages from the subject, call an HTTP endpoint with the body of the message, and write response or error in the respective subject. Following enviornment variables are used by connector image as configuration to connect and authenticate with NATs server which should be defined in the Kubernetes deployment manifest.

- `TOPIC`: Subject from which messages are read
- `HTTP_ENDPOINT`: http endpoint to post request
- `ERROR_TOPIC`: Subject to write errors on failure.
- `RESPONSE_TOPIC`: Subject to write responses on success response.
- `SOURCE_NAME`: Optional. Name of the Source. Default is "KEDAConnector"
- `MAX_RETRIES`: Maximum number of times an http endpoint will be retried upon failure.
- `CONTENT_TYPE`: Content type used while creating post request
- `HOST`: NATs connection string, like `nats://127.0.0.1:4222`

