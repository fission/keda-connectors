// Following code snippet is from KEDA project and adapted for Fission.
// Copyright 2020 The KEDA Authors.
// and others that have contributed code to the public domain.
// Licensed under the Apache License, Version 2.0 (the "License");
// https://github.com/kedacore/keda/blob/v1.5.0/LICENSE

// https://github.com/kedacore/keda/blob/master/pkg/scalers/kafka_scram_client.go

package main

import (
	"crypto/sha256"
	"crypto/sha512"

	"github.com/xdg/scram"
)

var SHA256 scram.HashGeneratorFcn = sha256.New
var SHA512 scram.HashGeneratorFcn = sha512.New

type XDGSCRAMClient struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

func (x *XDGSCRAMClient) Begin(userName, password, authzID string) (err error) {
	x.Client, err = x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	x.ClientConversation = x.Client.NewConversation()
	return nil
}

func (x *XDGSCRAMClient) Step(challenge string) (response string, err error) {
	response, err = x.ClientConversation.Step(challenge)
	return
}

func (x *XDGSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}
