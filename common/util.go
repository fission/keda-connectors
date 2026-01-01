package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"go.uber.org/zap"
)

type (
	// ConnectorMetadata contains common fields used by connectors
	ConnectorMetadata struct {
		Topic         string
		ResponseTopic string
		ErrorTopic    string
		HTTPEndpoint  string
		MaxRetries    int
		ContentType   string
		SourceName    string
	}

	FunctionHTTPRequest struct {
		Message      string
		HTTPEndpoint string
		Headers      http.Header
	}

	FunctionHTTPResponse struct {
		ResponseBody string
		StatusCode   int
		ErrorString  string
	}

	FunctionErrorDetails struct {
		FunctionHTTPRequest  FunctionHTTPRequest
		FunctionHTTPResponse FunctionHTTPResponse
	}
)

// ParseConnectorMetadata parses connector side common fields and returns as ConnectorMetadata or returns error
func ParseConnectorMetadata() (ConnectorMetadata, error) {
	for _, envVars := range []string{"TOPIC", "HTTP_ENDPOINT", "MAX_RETRIES", "CONTENT_TYPE"} {
		if os.Getenv(envVars) == "" {
			return ConnectorMetadata{}, fmt.Errorf("environment variable not found: %v", envVars)
		}
	}
	meta := ConnectorMetadata{
		Topic:         os.Getenv("TOPIC"),
		ResponseTopic: os.Getenv("RESPONSE_TOPIC"),
		ErrorTopic:    os.Getenv("ERROR_TOPIC"),
		HTTPEndpoint:  os.Getenv("HTTP_ENDPOINT"),
		ContentType:   os.Getenv("CONTENT_TYPE"),
		SourceName:    os.Getenv("SOURCE_NAME"),
	}
	if meta.SourceName == "" {
		meta.SourceName = "KEDAConnector"
	}
	val, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("MAX_RETRIES")), 0, 64)
	if err != nil {
		return ConnectorMetadata{}, fmt.Errorf("failed to parse value from MAX_RETRIES environment variable %v", err)
	}
	meta.MaxRetries = int(val)
	return meta, nil
}

// HandleHTTPRequest sends message and headers data to HTTP endpoint using POST method and returns response on success or error in case of failure
func HandleHTTPRequest(message string, headers http.Header, data ConnectorMetadata, logger *zap.Logger) (*http.Response, error) {

	var resp *http.Response
	for attempt := 0; attempt <= data.MaxRetries; attempt++ {
		// Create request
		req, err := http.NewRequest("POST", data.HTTPEndpoint, strings.NewReader(message))
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request to invoke function. http_endpoint: %s, source: %s: %w", data.HTTPEndpoint, data.SourceName, err)
		}

		// Add headers
		for key, vals := range headers {
			for _, val := range vals {
				req.Header.Add(key, val)
			}
		}

		// Make the request
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			logger.Error("sending function invocation request failed",
				zap.Error(err),
				zap.String("http_endpoint", data.HTTPEndpoint),
				zap.String("source", data.SourceName))
			continue
		}
		if resp == nil {
			continue
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// Success, quit retrying
			return resp, nil
		}
	}

	if resp == nil || resp.StatusCode < 200 || resp.StatusCode > 300 {
		errResp := NewFunctionErrorDetails(message, data.HTTPEndpoint, headers)
		err := errResp.UpdateResponseDetails(resp, data)
		if err != nil {
			return nil, err
		}
	}

	return resp, nil
}

/*
The config will be automatically loaded by the AWS SDK.
The order configuration is loaded in is:
- Environment Variables
- Shared Credentials file
- Shared Configuration file (if SharedConfig is enabled)
- EC2 Instance Metadata (credentials only)
GetAwsConfig allows override of aws region & endpoint
*/
func GetAwsConfig(ctx context.Context) (aws.Config, error) {
	options := []func(*config.LoadOptions) error{}
	if os.Getenv("AWS_REGION") != "" {
		options = append(options, config.WithRegion(os.Getenv("AWS_REGION")))
	}
	if os.Getenv("AWS_ENDPOINT") != "" {
		options = append(options, config.WithBaseEndpoint(os.Getenv("AWS_ENDPOINT")))
	}
	return config.LoadDefaultConfig(ctx, options...)

}

func NewFunctionErrorDetails(message, httpEndpoint string, headers http.Header) FunctionErrorDetails {
	return FunctionErrorDetails{
		FunctionHTTPRequest: FunctionHTTPRequest{
			Message:      message,
			HTTPEndpoint: httpEndpoint,
			Headers:      headers,
		},
		FunctionHTTPResponse: FunctionHTTPResponse{
			ResponseBody: "",
			StatusCode:   http.StatusInternalServerError,
			ErrorString:  "",
		},
	}
}

func (errResp *FunctionErrorDetails) UpdateResponseDetails(resp *http.Response, data ConnectorMetadata) error {
	if resp == nil {
		errResp.FunctionHTTPResponse.ErrorString = fmt.Sprintf("every function invocation retry failed; final retry gave empty response. http_endpoint: %s, source: %s", data.HTTPEndpoint, data.SourceName)
		errorBytes, err := json.Marshal(errResp)
		if err != nil {
			return fmt.Errorf("failed marshalling error response. http_endpoint: %s, source: %s", data.HTTPEndpoint, data.SourceName)
		}
		return errors.New(string(errorBytes))
	}

	if resp.StatusCode < 200 || resp.StatusCode > 300 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed reading response body. http_endpoint: %s, source: %s", data.HTTPEndpoint, data.SourceName)
		}
		errResp.FunctionHTTPResponse.ResponseBody = string(body)
		errResp.FunctionHTTPResponse.StatusCode = resp.StatusCode
		errResp.FunctionHTTPResponse.ErrorString = fmt.Sprintf("request returned failure: %d. http_endpoint: %s, source: %s", resp.StatusCode, data.HTTPEndpoint, data.SourceName)
		errorBytes, err := json.Marshal(errResp)
		if err != nil {
			return fmt.Errorf("failed marshalling error response. http_endpoint: %s, source: %s", data.HTTPEndpoint, data.SourceName)
		}
		return errors.New(string(errorBytes))
	}

	return nil
}
