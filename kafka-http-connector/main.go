package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Shopify/sarama"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/fission/keda-connectors/common"
)

// Following code snippet is from KEDA project and adapted for Fission.
// Copyright 2020 The KEDA Authors.
// and others that have contributed code to the public domain.
// Licensed under the Apache License, Version 2.0 (the "License");
// https://github.com/kedacore/keda/blob/v1.5.0/LICENSE

// https://github.com/kedacore/keda/blob/v1.5.0/pkg/scalers/kafka_scaler.go#L28
type kafkaMetadata struct {
	bootstrapServers []string
	consumerGroup    string

	// auth
	authMode kafkaAuthMode
	username string
	password string

	// ssl
	cert string
	key  string
	ca   string
}

type kafkaAuthMode string

const (
	kafkaAuthModeNone            kafkaAuthMode = "none"
	kafkaAuthModeSaslPlaintext   kafkaAuthMode = "sasl_plaintext"
	kafkaAuthModeSaslScramSha256 kafkaAuthMode = "sasl_scram_sha256"
	kafkaAuthModeSaslScramSha512 kafkaAuthMode = "sasl_scram_sha512"
	kafkaAuthModeSaslSSL         kafkaAuthMode = "sasl_ssl"
	kafkaAuthModeSaslSSLPlain    kafkaAuthMode = "sasl_ssl_plain"
)

// https://github.com/kedacore/keda/blob/v1.5.0/pkg/scalers/kafka_scaler.go#L83
func parseKafkaMetadata(logger *zap.Logger) (kafkaMetadata, error) {
	meta := kafkaMetadata{}

	// brokerList marked as deprecated, bootstrapServers is the new one to use
	if os.Getenv("BROKER_LIST") != "" && os.Getenv("BOOTSTRAP_SERVERS") != "" {
		return meta, errors.New("cannot specify both bootstrapServers and brokerList (deprecated)")
	}
	if os.Getenv("BROKER_LIST") == "" && os.Getenv("BOOTSTRAP_SERVERS") == "" {
		return meta, errors.New("no bootstrapServers or brokerList (deprecated) given")
	}
	if os.Getenv("BOOTSTRAP_SERVERS") != "" {
		meta.bootstrapServers = strings.Split(os.Getenv("BOOTSTRAP_SERVERS"), ",")
	}
	if os.Getenv("BROKER_LIST") != "" {
		logger.Info("WARNING: usage of brokerList is deprecated. use bootstrapServers instead.")
		meta.bootstrapServers = strings.Split(os.Getenv("BROKER_LIST"), ",")
	}
	if os.Getenv("CONSUMER_GROUP") == "" {
		return meta, errors.New("No consumerGroup given")
	}
	meta.consumerGroup = os.Getenv("CONSUMER_GROUP")

	meta.authMode = kafkaAuthModeNone
	mode := kafkaAuthMode(strings.TrimSpace((os.Getenv("AUTH_MODE"))))
	if mode == "" {
		mode = kafkaAuthModeNone
	}

	if mode != kafkaAuthModeNone && mode != kafkaAuthModeSaslPlaintext && mode != kafkaAuthModeSaslSSL && mode != kafkaAuthModeSaslSSLPlain && mode != kafkaAuthModeSaslScramSha256 && mode != kafkaAuthModeSaslScramSha512 {
		return meta, fmt.Errorf("err auth mode %s given", mode)
	}

	meta.authMode = mode

	if meta.authMode != kafkaAuthModeNone && meta.authMode != kafkaAuthModeSaslSSL {
		if os.Getenv("USERNAME") == "" {
			return meta, errors.New("no username given")
		}
		meta.username = strings.TrimSpace(os.Getenv("USERNAME"))

		if os.Getenv("PASSWORD") == "" {
			return meta, errors.New("no password given")
		}
		meta.password = strings.TrimSpace(os.Getenv("PASSWORD"))
	}

	if meta.authMode == kafkaAuthModeSaslSSL {
		if os.Getenv("CA") == "" {
			return meta, errors.New("no ca given")
		}
		meta.ca = os.Getenv("CA")

		if os.Getenv("CERT") == "" {
			return meta, errors.New("no cert given")
		}
		meta.cert = os.Getenv("CERT")

		if os.Getenv("KEY") == "" {
			return meta, errors.New("no key given")
		}
		meta.key = os.Getenv("KEY")
	}

	return meta, nil
}

// End of code snippet from KEDA project.

func getConfig(metadata kafkaMetadata) (*sarama.Config, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V1_0_0_0

	if ok := metadata.authMode == kafkaAuthModeSaslPlaintext || metadata.authMode == kafkaAuthModeSaslSSLPlain || metadata.authMode == kafkaAuthModeSaslScramSha256 || metadata.authMode == kafkaAuthModeSaslScramSha512; ok {
		config.Net.SASL.Enable = true
		config.Net.SASL.User = metadata.username
		config.Net.SASL.Password = metadata.password
	}

	if metadata.authMode == kafkaAuthModeSaslSSLPlain {
		config.Net.SASL.Mechanism = sarama.SASLMechanism(sarama.SASLTypePlaintext)

		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			ClientAuth:         0,
		}

		config.Net.TLS.Enable = true
		config.Net.TLS.Config = tlsConfig
		config.Net.DialTimeout = 10 * time.Second
	}

	if metadata.authMode == kafkaAuthModeSaslSSL {
		cert, err := tls.X509KeyPair([]byte(metadata.cert), []byte(metadata.key))
		if err != nil {
			return nil, fmt.Errorf("error parse X509KeyPair: %s", err)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM([]byte(metadata.ca))

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCertPool,
		}

		config.Net.TLS.Enable = true
		config.Net.TLS.Config = tlsConfig
	}

	if metadata.authMode == kafkaAuthModeSaslScramSha256 {
		config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
		config.Net.SASL.Mechanism = sarama.SASLMechanism(sarama.SASLTypeSCRAMSHA256)
	}

	if metadata.authMode == kafkaAuthModeSaslScramSha512 {
		config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA512} }
		config.Net.SASL.Mechanism = sarama.SASLMechanism(sarama.SASLTypeSCRAMSHA512)
	}

	if metadata.authMode == kafkaAuthModeSaslPlaintext {
		config.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		config.Net.TLS.Enable = true
	}
	return config, nil
}

func extractControlHeaders(headers []sarama.RecordHeader) ([]byte, bool, []sarama.RecordHeader) {
	var (
		key []byte
	)
	tombstone := false
	var cleaned []sarama.RecordHeader
	for _, header := range headers {
		if strings.ToLower(string(header.Key)) == "keda-message-key" {
			key = header.Value
			continue
		}

		if strings.ToLower(string(header.Key)) == "keda-message-tombstone" {
			tombstone = true
			continue
		}
		cleaned = append(cleaned, header)
	}

	return key, tombstone, cleaned
}

// kafkaConnector represents a Sarama consumer group consumer
type kafkaConnector struct {
	ready         chan bool
	logger        *zap.Logger
	producer      sarama.SyncProducer
	connectorData common.ConnectorMetadata
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (conn *kafkaConnector) Setup(sarama.ConsumerGroupSession) error {
	close(conn.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (conn *kafkaConnector) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages()
func (conn *kafkaConnector) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {

	// NOTE:
	// Do not move the code below to a goroutine.
	// The `ConsumeClaim` itself is called within a goroutine, see:
	// https://github.com/Shopify/sarama/blob/master/consumer_group.go#L27-L29
	for message := range claim.Messages() {
		conn.logger.Info(fmt.Sprintf("Message claimed: key = %s, value = %s, timestamp = %v, topic = %s", string(message.Key), string(message.Value), message.Timestamp, message.Topic))
		msg := string(message.Value)

		headers := http.Header{
			"KEDA-Topic":          {conn.connectorData.Topic},
			"KEDA-Response-Topic": {conn.connectorData.ResponseTopic},
			"KEDA-Error-Topic":    {conn.connectorData.ErrorTopic},
			"Content-Type":        {conn.connectorData.ContentType},
			"KEDA-Source-Name":    {conn.connectorData.SourceName},
		}

		// Add the message key, if it's been set.
		if message.Key != nil {
			headers.Add("KEDA-Message-Key", string(message.Key))
		}

		// Indicate that this is a tombstone, not a empty message.
		// Normally indicative of a deletion request
		if message.Value == nil {
			headers.Add("KEDA-Message-Tombstone", "true")
		}

		// Set the headers came from Kafka record
		for _, h := range message.Headers {
			headers.Add(string(h.Key), string(h.Value))
		}

		resp, err := common.HandleHTTPRequest(msg, headers, conn.connectorData, conn.logger)
		if err != nil {
			conn.errorHandler(resp, err)
		} else {
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				conn.errorHandler(nil, err)
			} else {
				// Generate Kafka record headers
				kafkaRecordHeaders := mapHeaders(resp)
				if success := conn.responseHandler(string(body), kafkaRecordHeaders); success {
					session.MarkMessage(message, "")
				}
			}
		}
	}
	return nil
}

func mapHeaders(resp *http.Response) []sarama.RecordHeader {
	var kafkaRecordHeaders []sarama.RecordHeader

	for k, v := range resp.Header {
		// One key may have multiple values
		for _, v := range v {
			kafkaRecordHeaders = append(kafkaRecordHeaders, sarama.RecordHeader{Key: []byte(k), Value: []byte(v)})
		}
	}
	return kafkaRecordHeaders
}

func (conn *kafkaConnector) errorHandler(resp *http.Response, err error) {
	if len(conn.connectorData.ErrorTopic) > 0 {
		message := &sarama.ProducerMessage{
			Topic: conn.connectorData.ErrorTopic,
			Value: sarama.StringEncoder(err.Error()),
		}
		if resp != nil {
			kafkaRecordHeaders := mapHeaders(resp)
			key, _, headers := extractControlHeaders(kafkaRecordHeaders)
			message.Headers = headers
			if key != nil {
				message.Key = sarama.StringEncoder(key)
			}
		}
		_, _, e := conn.producer.SendMessage(message)
		if e != nil {
			conn.logger.Error("failed to publish message to error topic",
				zap.Error(e),
				zap.String("source", conn.connectorData.SourceName),
				zap.String("message", err.Error()),
				zap.String("topic", conn.connectorData.Topic))
		}
	} else {
		conn.logger.Error("message received to publish to error topic, but no error topic was set",
			zap.String("message", err.Error()),
			zap.String("source", conn.connectorData.SourceName),
			zap.String("http endpoint", conn.connectorData.HTTPEndpoint),
		)
	}
}

func (conn *kafkaConnector) responseHandler(msg string, headers []sarama.RecordHeader) bool {

	// extract the key and tombstone should they exist.
	key, tombstone, headers := extractControlHeaders(headers)

	if len(conn.connectorData.ResponseTopic) > 0 {
		message := &sarama.ProducerMessage{
			Topic:   conn.connectorData.ResponseTopic,
			Headers: headers,
		}

		if key != nil {
			message.Key = sarama.StringEncoder(key)
		}

		if len(msg) > 0 || !tombstone {
			message.Value = sarama.StringEncoder(msg)
		}

		_, _, err := conn.producer.SendMessage(message)
		if err != nil {
			conn.logger.Warn("failed to publish response body from http request to topic",
				zap.Error(err),
				zap.String("topic", conn.connectorData.Topic),
				zap.String("source", conn.connectorData.SourceName),
				zap.String("http endpoint", conn.connectorData.HTTPEndpoint))
			return false
		}
	}

	return true
}

func getProducer(metadata kafkaMetadata) (sarama.SyncProducer, error) {
	config, err := getConfig(metadata)
	if err != nil {
		return nil, err
	}

	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 10
	config.Producer.Return.Successes = true
	producer, err := sarama.NewSyncProducer(metadata.bootstrapServers, config)
	if err != nil {
		return nil, err
	}
	return producer, nil
}

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()

	metadata, err := parseKafkaMetadata(logger)
	if err != nil {
		logger.Error("Failed to fetch kafka metadata", zap.Error(err))
		return
	}

	connData, err := common.ParseConnectorMetadata()
	if err != nil {
		logger.Error("Failed to parse connector meta data", zap.Error(err))
		return
	}

	config, err := getConfig(metadata)
	if err != nil {
		logger.Error("Failed to create kafka config", zap.Error(err))
		return
	}

	producer, err := getProducer(metadata)
	if err != nil {
		logger.Error("Failed to create kafka producer", zap.Error(err))
		return
	}
	defer producer.Close()

	conn := kafkaConnector{
		ready:         make(chan bool),
		logger:        logger,
		producer:      producer,
		connectorData: connData,
	}

	ctx, cancel := context.WithCancel(context.Background())
	client, err := sarama.NewConsumerGroup(metadata.bootstrapServers, metadata.consumerGroup, config)
	if err != nil {
		logger.Error("Error creating consumer group client", zap.Error(err))
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		for {
			if err := client.Consume(ctx, []string{connData.Topic}, &conn); err != nil {
				logger.Error("Error from consumer", zap.Error(err))
			}
			// check if context was cancelled, signaling that the consumer should stop
			if ctx.Err() != nil {
				return
			}
			conn.ready = make(chan bool)
		}
	}()

	<-conn.ready // Await till the consumer has been set up
	logger.Info("Sarama consumer up and running!...")
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-ctx.Done():
		logger.Info("terminating: context cancelled")
	case <-sigterm:
		logger.Info("terminating: via signal")
	}
	cancel()
	wg.Wait()
	if err = client.Close(); err != nil {
		logger.Error("Error closing client", zap.Error(err))
	}
}
