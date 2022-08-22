# JetStream NATS KEDA Connector

JetStream NATS KEDA connector image can be used in the Kubernetes deployment as scaleTargetRef in scaledObject of [NATs scaler](https://keda.sh/docs/2.8/scalers/nats-jetstream/).

The job of the connector is to read messages from the subject in the given stream, call an HTTP endpoint with the body of the message, and write response or error in the response_topic. Following enviornment variables are used by connector image as configuration to connect and authenticate with NATs server which should be defined in the Kubernetes deployment manifest.

- `TOPIC`: Subject from which messages are read. It is generally of form - `streamname.subjectname`
- `HTTP_ENDPOINT`: http endpoint to post request
- `ERROR_TOPIC`: Subject to write errors on failure.
- `RESPONSE_TOPIC`: Subject to write responses on success response.  It is generally of form - `response_stream_name.response_subject_name` where streamname should be different then input stream //TODO: need to check this
- `MAX_RETRIES`: Maximum number of times an http endpoint will be retried upon failure.
- `CONTENT_TYPE`: Content type used while creating post request
- `NATS_SERVER`: NATS server address. It can be a remote address `nats://127.0.0.1:4222` or in case deployed in Kubernetes, can reached using corresponding service name
- `STREAM`: stream from which connector will read messages.
-  `CONSUMER`: consumer which would be created in the connector to read data from the stream
- `FISSION_CONSUMER`: this is the consumer which fission uses to pull data from stream, proccesses it and pushed it forward to `RESPONSE_TOPIC`



## Resources
* To setup and run nats streaming server, reference https://docs.nats.io/nats-server/installation#installing-on-kubernetes-with-nats-operator
* For running the connecter with fission e.g.  

``` fission mqt create --name jetstreamtest --function helloworld --mqtype nats-jetstream --mqtkind keda --topic input.created --resptopic output.response-topic --errortopic output.error-topic --maxretries 3 --metadata stream=input --metadata fissionConsumer= fission_consumer --metadata consumer=jetstream_consumer --metadata natsServerMonitoringEndpoint=nats-jetstream.default.svc.cluster.local:8222  --metadata natsServer=nats://nats-jetstream.default.svc.cluster.local:4222 ```

 
          

