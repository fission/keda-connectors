# NATS Streaming KEDA Connector

NATs streaming KEDA connector image can be used in the Kubernetes deployment as scaleTargetRef in scaledObject of [NATs scaler](https://keda.sh/docs/1.5/scalers/nats-streaming/).

The job of the connector is to read messages from the subject, call an HTTP endpoint with the body of the message, and write response or error in the respective subject. Following enviornment variables are used by connector image as configuration to connect and authenticate with NATs server which should be defined in the Kubernetes deployment manifest.

- `TOPIC`: Subject from which messages are read
- `HTTP_ENDPOINT`: http endpoint to post request
- `ERROR_TOPIC`: Subject to write errors on failure.
- `RESPONSE_TOPIC`: Subject to write responses on success response.
- `SOURCE_NAME`: Optional. Name of the Source. Default is "KEDAConnector"
- `MAX_RETRIES`: Maximum number of times an http endpoint will be retried upon failure.
- `CONTENT_TYPE`: Content type used while creating post request
- `NATS_SERVER`: NATs connection string, like `nats://127.0.0.1:4222`
- `CLUSTER_ID`: StanClusterID to form a connection to the NATS Streaming subsystem
- `DURABLE_NAME`: Durable name to resume message consumption from where it previously stopped



## Resources
* To setup and run nats streaming server, reference https://github.com/nats-io/nats-streaming-server
* For running the connecter with fission e.g.  
 ```fission mqt create --name natstest --function helloworld --mqtype stan --topic hello --resptopic response --mqtkind keda --errortopic error --maxretries 3 --metadata subject=hello --metadata queueGroup=grp1 --metadata durableName=due --metadata natsServerMonitoringEndpoint=nats.default.svc.cluster.local:8222 --metadata clusterId=test-cluster --metadata natsServer=nats://nats:4222```