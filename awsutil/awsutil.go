package awsutil

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"

	"github.com/pkg/errors"
)

//GetAwsConfig return configuration for aws connection
func GetAwsConfig() (*aws.Config, error) {
	if os.Getenv("AWS_REGION") == "" {
		return nil, errors.New("aws region required")
	}
	config := &aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION")),
	}
	if os.Getenv("AWS_ENDPOINT") != "" {
		endpoint := os.Getenv("AWS_ENDPOINT")
		config.Endpoint = &endpoint
		return config, nil
	}
	if os.Getenv("AWS_ACCESS_KEY") != "" && os.Getenv("AWS_SECRET_KEY") != "" {
		config.Credentials = credentials.NewStaticCredentials(os.Getenv("AWS_ACCESS_KEY"), os.Getenv("AWS_SECRET_KEY"), "")
		return config, nil
	}
	if os.Getenv("AWS_CRED_PATH") != "" && os.Getenv("AWS_CRED_PROFILE") != "" {
		config.Credentials = credentials.NewSharedCredentials(os.Getenv("AWS_CRED_PATH"), os.Getenv("AWS_CRED_PROFILE"))
		return config, nil
	}
	return nil, errors.New("no aws configuration specified")
}
