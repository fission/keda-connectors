# JetStream NATS KEDA Connector

JetStream NATS KEDA connector image can be used in the Kubernetes deployment as scaleTargetRef in scaledObject of [NATs scaler](https://keda.sh/docs/2.8/scalers/nats-jetstream/).

The job of the connector is to read messages from the subject in the given stream, call an HTTP endpoint with the body of the message, and write response or error in the response_topic. Following enviornment variables are used by connector image as configuration to connect and authenticate with NATs server which should be defined in the Kubernetes deployment manifest.

- `TOPIC`: Subject from which messages are read. It is generally of form - `streamname.subjectname`
- `RESPONSE_TOPIC`: Subject to write responses on success response.  It is generally of form - `response_stream_name.response_subject_name` where streamname should be different then input stream. `response_stream_name` is output stream name. `response_subject_name` subject name where output is send
- `ERROR_TOPIC`: Subject to write errors on failure.  It is generally of form - `err_response_stream_name.error_subject_name` where streamname should be different then input stream. `err_response_stream_name` is error stream name. `error_subject_name` subject name where error output is send
- `MAX_RETRIES`: Maximum number of times an http endpoint will be retried upon failure
- `CONTENT_TYPE`: Content type used while creating post request
- `STREAM`: stream from which connector will read messages.
- `NATS_SERVER_MONITORING_ENDPOINT`: Location of the Nats Jetstream Monitoring
- `NATS_SERVER`: NATS server address. It can be a remote address `nats://127.0.0.1:4222` or in case deployed in Kubernetes, can reached using corresponding service name
- `CONSUMER`: this is the consumer which fission uses for monitoring and creating resources(eg, creating pods)
- `ACCOUNT` - Name of the NATS account. `$G` is default when no account is configured

## Resources

- To setup and run nats streaming server, reference <https://docs.nats.io/nats-server/installation#installing-on-kubernetes-with-nats-operator>
- For running the connecter with fission e.g.  

```fission mqt create --name jetstreamtest --function helloworld --mqtype nats-jetstream --mqtkind keda --topic input.created --resptopic output.response-topic --errortopic erroutput.error-topic --maxretries 3 --metadata stream=input --metadata natsServerMonitoringEndpoint=nats-jetstream.default.svc.cluster.local:8222 --metadata natsServer=nats://nats-jetstream.default.svc.cluster.local:4222 --metadata consumer=fission_consumer --metadata account=\$G```
