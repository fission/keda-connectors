# AWS SQS KEDA Connector

AWS SQS KEDA connector image can be used in the Kubernetes deployment as scaleTargetRef in scaledObject of [AWS SQS scaler](https://keda.sh/docs/1.5/scalers/aws-sqs/).

The job of the connector is to read messages from the queue, call an HTTP endpoint with the body of the message, and write response or error in the respective queues. Following enviornment variables are used by connector image as configuration to connect and authenticate with AWS SQS cluster which should be defined in the Kubernetes deployment manifest.

- `TOPIC`: Queue from which messages are read.
- `HTTP_ENDPOINT`: http endpoint to post request.
- `ERROR_TOPIC`: Queue to write errors on failure.
- `RESPONSE_TOPIC`: Queue to write responses on success response.
- `SOURCE_NAME`: Optional. Name of the Source. Default is "KEDAConnector".
- `MAX_RETRIES`: Maximum number of times an http endpoint will be retried upon failure.
- `CONTENT_TYPE`: Content type used while creating post request.
- `AWS_SQS_URL`: AWS SQS full URL with account id, for example http://localhost:4576/000000000000/.  


#### Ways to connect to AWS
- `AWS_REGION`: Region is mandatory for any aws connection.
  
1) Through AWS endpoint  
- `AWS_ENDPOINT` : SQS endpoint on which it is running, for local it can be http://localhost:4576.  

2) Through AWS aws key and secret
- `AWS_ACCESS_KEY_ID`: aws access key of your account.
- `AWS_SECRET_ACCESS_KEY`: aws secret key got from your account.  

3) Through AWS credentials
- `AWS_CRED_PATH`: Path where aws credentials are present, ex ~/.aws/credentials.
- `AWS_CRED_PROFILE`: Profile With which to connect to AWS, present in  ~/.aws/credentials file.


More information about the above parameters and how to define it scaledobject refer [AWS SQS scaler doc](https://keda.sh/docs/1.5/scalers/aws-sqs/).
