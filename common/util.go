package common

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

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

// HandleHTTPRequest sends message and headers data to HTTP endpoint using POST method and returns error in case of failure
func HandleHTTPRequest(message string, headers map[string]string, data ConnectorMetadata, logger *zap.Logger) error {
	// Create request
	req, err := http.NewRequest("POST", data.HTTPEndpoint, strings.NewReader(message))
	if err != nil {
		return errors.Wrapf(err, "failed to create HTTP request to invoke function. http_endpoint: %v, source: %v", data.HTTPEndpoint, data.SourceName)
	}
	// Add headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	var resp *http.Response
	for attempt := 0; attempt <= data.MaxRetries; attempt++ {
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
		if err == nil && resp.StatusCode == http.StatusOK {
			// Success, quit retrying
			break
		}
	}

	if resp == nil {
		return fmt.Errorf("every function invocation retry failed; final retry gave empty response. http_endpoint: %v, source: %v", data.HTTPEndpoint, data.SourceName)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	logger.Debug("got response from function invocation",
		zap.String("http_endpoint", data.HTTPEndpoint),
		zap.String("source", data.SourceName),
		zap.String("body", string(body)))

	if err != nil {
		return errors.Wrapf(err, "request body error: %v. http_endpoint: %v, source: %v", string(body), data.HTTPEndpoint, data.SourceName)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("request returned failure: %v. http_endpoint: %v, source: %v", resp.StatusCode, data.HTTPEndpoint, data.SourceName)
	}
	return nil
}
