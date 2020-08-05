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
	FunctionURL   string
	MaxRetries    int
	ContentType   string
	TriggerName   string
}

// ParseConnectorMetadata parses connector side common fields and returns as ConnectorMetadata or returns error
func ParseConnectorMetadata() (ConnectorMetadata, error) {
	for _, envVars := range []string{"TOPIC", "FUNCTION_URL", "MAX_RETRIES", "CONTENT_TYPE", "TRIGGER_NAME"} {
		if os.Getenv(envVars) == "" {
			return ConnectorMetadata{}, fmt.Errorf("environment variable not found: %v", envVars)
		}
	}
	meta := ConnectorMetadata{
		Topic:         os.Getenv("TOPIC"),
		ResponseTopic: os.Getenv("RESPONSE_TOPIC"),
		ErrorTopic:    os.Getenv("ERROR_TOPIC"),
		FunctionURL:   os.Getenv("FUNCTION_URL"),
		ContentType:   os.Getenv("CONTENT_TYPE"),
		TriggerName:   os.Getenv("TRIGGER_NAME"),
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
	req, err := http.NewRequest("POST", data.FunctionURL, strings.NewReader(message))
	if err != nil {
		return errors.Wrapf(err, "failed to create HTTP request to invoke function. function_url: %v, trigger %v", data.FunctionURL, data.TriggerName)
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
				zap.String("function_url", data.FunctionURL),
				zap.String("trigger", data.TriggerName))
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
		return fmt.Errorf("every function invocation retry failed; final retry gave empty response. function_url: %v, trigger %v", data.FunctionURL, data.TriggerName)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	logger.Debug("got response from function invocation",
		zap.String("function_url", data.FunctionURL),
		zap.String("trigger", data.TriggerName),
		zap.String("body", string(body)))

	if err != nil {
		return errors.Wrapf(err, "request body error: %v. function_url: %v, trigger %v", string(body), data.FunctionURL, data.TriggerName)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("request returned failure: %v. function_url: %v, trigger %v", resp.StatusCode, data.FunctionURL, data.TriggerName)
	}
	return nil
}
