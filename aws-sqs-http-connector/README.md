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
- `QUEUE_URL`: AWS SQS full URL with account id, for example  https://sqs.ap-south-1.amazonaws.com/account_id/QueueName.  


## Ways to connect to AWS

This application uses the AWS SDK's default credential provider chain to authenticate with AWS services. The credential provider chain tries the following sources in order:

1. **Environment Variables**
   - `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`
   - Optional: `AWS_SESSION_TOKEN` (for temporary credentials)

2. **Shared Credential File** (`~/.aws/credentials`)
   - Default profile or profile specified by `AWS_PROFILE` environment variable

3. **IAM Role for Amazon EC2** or **ECS Task Role**
   - Used automatically when running in EC2 or ECS environments

4. **Web Identity Token** (for EKS and other Kubernetes environments)
   - Used automatically when `AWS_WEB_IDENTITY_TOKEN_FILE` is set
   - Supports IAM Roles for Service Accounts (IRSA) in EKS

### Required Environment Variables

- `AWS_REGION`: AWS region where your resources are located

### Optional Environment Variables

- `AWS_ENDPOINT`: Custom AWS endpoint URL (useful for local development or testing)
- `AWS_PROFILE`: Profile name from your shared credentials file
- `AWS_SDK_LOAD_CONFIG`: Set to "true" to load configuration from shared config file (~/.aws/config)
- `AWS_WEB_IDENTITY_TOKEN_FILE`: Path to the web identity token file (for EKS and Kubernetes pod-based authentication)
- `AWS_ROLE_ARN`: ARN of the role to assume when using web identity federation

For more information refer to:
- AWS authentication and configuration -> [AWS SDK for Go documentation](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html).
- Keda AWS SQS ScaledObject -> [AWS SQS scaler doc](https://keda.sh/docs/1.5/scalers/aws-sqs/).
