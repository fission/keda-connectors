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
	saslType string
	username string
	password string

	// ssl
	tls                string
	cert               string
	key                string
	ca                 string
	InsecureSkipVerify bool
}

const (
	kafkaAuthModeNone            string = ""
	kafkaAuthModeSaslPlaintext   string = "plaintext"
	kafkaAuthModeSaslScramSha256 string = "scram_sha256"
	kafkaAuthModeSaslScramSha512 string = "scram_sha512"
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

	meta.InsecureSkipVerify = true
	if os.Getenv("TLS_INSECURE_SKIP_VERIFY") == "false" {
		meta.InsecureSkipVerify = false
	}
	meta.tls = os.Getenv("TLS")
	if meta.tls == "" {
		meta.tls = "disabled"
	}

	meta.saslType = os.Getenv("SASL")

	if meta.saslType != kafkaAuthModeSaslPlaintext && meta.saslType != kafkaAuthModeNone && meta.saslType != kafkaAuthModeSaslScramSha256 && meta.saslType != kafkaAuthModeSaslScramSha512 {
		return meta, fmt.Errorf("Incorrect value for sasl authentication %s given", meta.saslType)
	}

	if meta.saslType != kafkaAuthModeNone {
		if os.Getenv("USERNAME") == "" {
			return meta, errors.New("no username given")
		}
		meta.username = strings.TrimSpace(os.Getenv("USERNAME"))

		if os.Getenv("PASSWORD") == "" {
			return meta, errors.New("no password given")
		}
		meta.password = strings.TrimSpace(os.Getenv("PASSWORD"))
	}

	if meta.tls == "enable" {
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
	config.Version = sarama.V2_0_0_0
	if ok := metadata.saslType == kafkaAuthModeSaslPlaintext || metadata.saslType == kafkaAuthModeSaslScramSha256 || metadata.saslType == kafkaAuthModeSaslScramSha512; ok {
		config.Net.SASL.Enable = true
		config.Net.SASL.User = metadata.username
		config.Net.SASL.Password = metadata.password
	}

	if metadata.saslType == kafkaAuthModeSaslPlaintext {
		config.Net.SASL.Mechanism = sarama.SASLMechanism(sarama.SASLTypePlaintext)
	}

	if metadata.saslType == kafkaAuthModeSaslScramSha256 {
		config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
		config.Net.SASL.Mechanism = sarama.SASLMechanism(sarama.SASLTypeSCRAMSHA256)
	}

	if metadata.saslType == kafkaAuthModeSaslScramSha512 {
		config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA512} }
		config.Net.SASL.Mechanism = sarama.SASLMechanism(sarama.SASLTypeSCRAMSHA512)
	}

	if metadata.tls == "enable" {
		config.Net.TLS.Enable = true
		tlsConfig, err := NewTLSConfig(metadata.cert, metadata.key, metadata.ca)
		if err != nil {
			return nil, err
		}
		config.Net.TLS.Config.InsecureSkipVerify = metadata.InsecureSkipVerify
		config.Net.TLS.Config = tlsConfig
	}

	return config, nil
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
		conn.logger.Info(fmt.Sprintf("Message claimed: value = %s, timestamp = %v, topic = %s", string(message.Value), message.Timestamp, message.Topic))
		msg := string(message.Value)

		headers := http.Header{
			"KEDA-Topic":          {conn.connectorData.Topic},
			"KEDA-Response-Topic": {conn.connectorData.ResponseTopic},
			"KEDA-Error-Topic":    {conn.connectorData.ErrorTopic},
			"Content-Type":        {conn.connectorData.ContentType},
			"KEDA-Source-Name":    {conn.connectorData.SourceName},
		}

		// Set the headers came from Kafka record
		for _, h := range message.Headers {
			headers.Add(string(h.Key), string(h.Value))
		}

		resp, err := common.HandleHTTPRequest(msg, headers, conn.connectorData, conn.logger)
		if err != nil {
			conn.errorHandler(err)
		} else {
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				conn.errorHandler(err)
			} else {
				// Generate Kafka record headers
				var kafkaRecordHeaders []sarama.RecordHeader

				for k, v := range resp.Header {
					// One key may have multiple values
					for _, v := range v {
						kafkaRecordHeaders = append(kafkaRecordHeaders, sarama.RecordHeader{Key: []byte(k), Value: []byte(v)})
					}
				}
				if success := conn.responseHandler(string(body), kafkaRecordHeaders); success {
					session.MarkMessage(message, "")
				}
			}
		}
	}
	return nil
}

func (conn *kafkaConnector) errorHandler(err error) {
	if len(conn.connectorData.ErrorTopic) > 0 {
		_, _, e := conn.producer.SendMessage(&sarama.ProducerMessage{
			Topic: conn.connectorData.ErrorTopic,
			Value: sarama.StringEncoder(err.Error()),
		})
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
	if len(conn.connectorData.ResponseTopic) > 0 {
		_, _, err := conn.producer.SendMessage(&sarama.ProducerMessage{
			Topic:   conn.connectorData.ResponseTopic,
			Value:   sarama.StringEncoder(msg),
			Headers: headers,
		})
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

	config.Producer.RequiredAcks = sarama.WaitForLocal
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

// NewTLSConfig returns a *tls.Config using the given ceClient cert, ceClient key,
// and CA certificate. If none are appropriate, a nil *tls.Config is returned.
// Ref: https://github.com/kedacore/keda/blob/154364276402783c08fa24e7968fd31b9f89b6a6/pkg/util/tls_config.go
// TODO: Move this to common package as other connectors might need this
func NewTLSConfig(clientCert, clientKey, caCert string) (*tls.Config, error) {
	valid := false

	config := &tls.Config{}

	if clientCert != "" && clientKey != "" {
		cert, err := tls.X509KeyPair([]byte(clientCert), []byte(clientKey))
		if err != nil {
			return nil, fmt.Errorf("error parse X509KeyPair: %s", err)
		}
		config.Certificates = []tls.Certificate{cert}
		valid = true
	}

	if caCert != "" {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM([]byte(caCert))
		config.RootCAs = caCertPool
		config.InsecureSkipVerify = true
		valid = true
	}

	if !valid {
		config = nil
	}

	return config, nil
}
