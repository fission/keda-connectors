Thanks for helping make Keda connectors better 😍!

There are many areas we can use contributions - ranging from code, documentation, feature proposals, issue triage, samples, and content creation.

* [Choose what to work on](#choose-what-to-work-on)
    * [Get Help.](#get-help)
    * [Contributing - building &amp; deploying](#contributing---building--deploying)
    * [rabbitmq-http-connector explained](#rabbitmq-http-connector-explained)
        * [The main()](#the-main)
        * [Consuming Messages](#consuming-messages)
        * [HandleHTTPRequest](#handlehttprequest)
        * [errorHandler](#errorhandler)
        * [responseHandler](#responsehandler)
        * [Dockerfile and plumbing](#dockerfile-and-plumbing)

## Choose what to work on

* The easiest way to start is to look at existing [issues](https://github.com/fission/keda-connectors/issues) and see if there's something there that you'd like to work on

* If you are going to pick up an issue, it would be good to add a comment stating the intention.

## Get Help.

Do reach out on Slack or Twitter and we are happy to help.

 * Drop by the [slack channel](http://slack.fission.io).
 * Say hi on [twitter](https://twitter.com/fissionio).

## Contributing - building & deploying

The job of the connector is to read messages from the event source, do some action, and write response or error in the respective queues/topics. The event source could be a queue/topic for a message queue and could be a table/row in case of a database. It will be best explained in context of one sample implementation and you can extrapolate that when writing a new connector.

### rabbitmq-http-connector explained

The rabbitmq-http-connector reads message from RabbitMQ (Source) and calls a HTTP endpoint where it send the message content as a HTTP request body. The code is rather simple as we will see in following sections. There are things which are applicable to every connector and there are things which are specific to that connector only, they will be called out as we explain.

#### The main()

The main method is reading a bunch of environment variables and forming two structs. 

The first set of environment variables is generic and applicable to all connectors and hence we are using the common package's method and simply getting the populated struct.

```
connectordata, err := common.ParseConnectorMetadata()

```

The fields in connector metadata are basic metadata which is not specific to a event source:

```
// ConnectorMetadata contains common fields used by connectors
type ConnectorMetadata struct {
	Topic         string
	ResponseTopic string
	ErrorTopic    string
	HTTPEndpoint  string
	MaxRetries    int
	ContentType   string
	SourceName    string
}
```

The second set of env variables we are getting is specific to RabbitMQ and how to connect to it and we populate the rabbitMQConnector struct with earlier created metadata and host on which to connect to for RabbitMQ and other things such as consumer channel and producer channel. For finding out which attributes are applicable for an event source, it is best to refer to Keda documentation for that event source. For example for [RabbitMQ the Keda documentation is here](https://keda.sh/docs/2.0/scalers/rabbitmq-queue/)

```
type rabbitMQConnector struct {
	host            string
	connectordata   common.ConnectorMetadata
	consumerChannel *amqp.Channel
	producerChannel *amqp.Channel
	logger          *zap.Logger
}
```

At end of main we are calling consume messages method, let's dive into that.

```
conn.consumeMessage()
```

#### Consuming Messages

The `consumeMessage()` the code is very specific to how you talk to RabbitMQ and fetch messages from it based on connection information we have. 

For each event/message we get from RabbitMQ, we do:

* Add following headers to the HTTP request's headers

```
headers := http.Header{
		"KEDA-Topic":          {conn.connectordata.Topic},
		"KEDA-Response-Topic": {conn.connectordata.ResponseTopic},
		"KEDA-Error-Topic":    {conn.connectordata.ErrorTopic},
		"Content-Type":        {conn.connectordata.ContentType},
		"KEDA-Source-Name":    {conn.connectordata.SourceName},
	}
```

* Form rest of HTTP request amd call HTTPHandler method from common package. This method is common for all connectors with destination as HTTPEndPoint.

* If the HTTP call errors with a HTTP code other than 200 then , call errorHandler. This logic is specific to the even source in this case the RabbitMQ.

* If the HTTP call succeeds with HTTP 200 then read the response body and use that to call responseHandler. This logic is specific to the even source in this case the RabbitMQ.

#### Handle HTTP-Request

The Handle HTTP-Request takes message and rest of information to make a HTTP call to HTTPEndpoint in ConnectorMetadata. This method is from common package, and we only use it.

#### errorHandler

If there is an error from the HTTP Endpoint then we send the message and error details to the ErrorTopic from ConnectorMetadata with appropriate logging.

#### responseHandler

If the request to HTTP Endpoint succeeded then the ResponseTopic of ConnectorMetadata will be used to drop a success message.

#### Dockerfile and plumbing

You need to write a Dockerfile which will use your go program with dependencies added and create an image. Now you can use this image to test by passing all environment variables appropriately.
