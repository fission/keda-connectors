# Keda Connectors

[Keda](https://keda.sh/) is a Kubernetes-based Event Driven Autoscaler which enables you to scale containers for processing events based on number of events.

Keda Connectors provide you readymade generic images to process events for standard tasks. For example if you are going to read messages from RabbitMQ and then call an HTTP endpoint then you can use [RabbitMQ HTTP Connector](./rabbitmq-http-connector/README.md). You just have to create a standard deployment manifest with all env variables needed for the pod.

These connectors are used in [Fission](http://github.com/fission/fission) project for integrating events to call functions via MQTrigger CRD.

# Connector List

|Connectot Name | Description |
|---|---|
|[Kafka HTTP Connector](./kafka-http-connector/README.md)| Consumes messages from Kafka topics and posts the message to an HTTP endpoint.|
|[RabbitMQ HTTP Connector](./rabbitmq-http-connector/README.md)|Reads message from RabbitMQ and posts to a HTTP endpoint. Currently only the AMQP protocol is supported for consuming RabbitMQ messages.|

# Contributing

## Connector Development Guide

The job of the connector is to read messages from the topic, invoke a HTTP endpoint, and write response or error in the respective queues/topics.

### Following are steps required to write a connector:

1. [Message Queue Trigger Spec](https://github.com/fission/fission/blob/master/pkg/mqtrigger/scalermanager.go#L163) fields are exposed as environment variables while creating deployment which will be utilized while creating consumer, producer, and during function invocation. Parse all required parameters required by the connector.

2. Create a Consumer for the queue.

3. Create a Producer for the queue.

4. Read messages from the queue using the consumer.

5. Create a POST request with the following headers using values specified during message queue trigger creation and headers from the consumed message.

```
{
   "Topic": Topic,
   "RespTopic": ResponseTopic,
   "ErrorTopic": ErrorTopic,
   "Content-Type": ContentType
}
```

6. Invoke HTTP endpoint per consumed message in a retry loop using max retries parameter specified. You can reuse the already available method for calling HTTP endpoints: https://github.com/fission/keda-connectors/blob/master/common/util.go#L52

7. Write the response in response queue, if the reponse topic is specified.

8. If applicable then write error in error queue, if error topic is specified.

9. The final step would be to write a dockerfile to create docker image of the code.

Refer to [Kafka HTTP Connector](./kafka-http-connector/README.md) or [RabbitMQ HTTP Connector](./rabbitmq-http-connector/README.md) for sample implementations.
