/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package rabbitmqclient

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Client
type Client interface {
	PutUser(string, rabbithole.UserSettings) (*http.Response, error)
	DeleteUser(string) (*http.Response, error)
	DeclareBinding(string, rabbithole.BindingInfo) (*http.Response, error)
	DeleteBinding(string, rabbithole.BindingInfo) (*http.Response, error)
	ListQueueBindingsBetween(string, string, string) ([]rabbithole.BindingInfo, error)
	ListExchangeBindingsBetween(string, string, string) ([]rabbithole.BindingInfo, error)
	UpdatePermissionsIn(string, string, rabbithole.Permissions) (*http.Response, error)
	ClearPermissionsIn(string, string) (*http.Response, error)
	PutPolicy(string, string, rabbithole.Policy) (*http.Response, error)
	DeletePolicy(string, string) (*http.Response, error)
	DeclareQueue(string, string, rabbithole.QueueSettings) (*http.Response, error)
	DeleteQueue(string, string, ...rabbithole.QueueDeleteOptions) (*http.Response, error)
	DeclareExchange(string, string, rabbithole.ExchangeSettings) (*http.Response, error)
	DeleteExchange(string, string) (*http.Response, error)
	PutVhost(string, rabbithole.VhostSettings) (*http.Response, error)
	DeleteVhost(string) (*http.Response, error)
	PutGlobalParameter(name string, value interface{}) (*http.Response, error)
	DeleteGlobalParameter(name string) (*http.Response, error)
	PutFederationUpstream(vhost, name string, def rabbithole.FederationDefinition) (res *http.Response, err error)
	DeleteFederationUpstream(vhost, name string) (res *http.Response, err error)
	DeclareShovel(vhost, shovel string, info rabbithole.ShovelDefinition) (res *http.Response, err error)
	DeleteShovel(vhost, shovel string) (res *http.Response, err error)
	GetVhost(vhost string) (rec *rabbithole.VhostInfo, err error)
	PutOperatorPolicy(string, string, rabbithole.OperatorPolicy) (*http.Response, error)
	DeleteOperatorPolicy(vhost, name string) (res *http.Response, err error)
	UpdateTopicPermissionsIn(vhost, username string, TopicPermissions rabbithole.TopicPermissions) (res *http.Response, err error)
	DeleteTopicPermissionsIn(vhost, username string, exchange string) (res *http.Response, err error)
}

type Factory func(connectionCreds map[string]string, tlsEnabled bool, certPool *x509.CertPool) (Client, error)

var RabbitholeClientFactory Factory = func(connectionCreds map[string]string, tlsEnabled bool, certPool *x509.CertPool) (Client, error) {
	return generateRabbitholeClient(connectionCreds, tlsEnabled, certPool)
}

// generateRabbitholeClient returns a http client for a given creds
// if provided RabbitmqCluster is nil, generateRabbitholeClient uses username, passwords, and uri
// information from connectionCreds to generate a rabbit client
func generateRabbitholeClient(connectionCreds map[string]string, tlsEnabled bool, certPool *x509.CertPool) (rabbitmqClient Client, err error) {
	defaultUser, found := connectionCreds["username"]
	if !found {
		return nil, keyMissingErr("username")
	}

	defaultUserPass, found := connectionCreds["password"]
	if !found {
		return nil, keyMissingErr("password")
	}

	uri, found := connectionCreds["uri"]
	if !found {
		return nil, keyMissingErr("uri")
	}

	if tlsEnabled {
		// create TLS config for https request
		cfg := new(tls.Config)
		cfg.RootCAs = certPool

		transport := &http.Transport{TLSClientConfig: cfg}
		rabbitmqClient, err = rabbithole.NewTLSClient(fmt.Sprintf("%s", string(uri)), string(defaultUser), string(defaultUserPass), transport)
		if err != nil {
			return nil, fmt.Errorf("failed to instantiate rabbit rabbitmqClient: %v", err)
		}
	} else {
		rabbitmqClient, err = rabbithole.NewClient(fmt.Sprintf("%s", string(uri)), string(defaultUser), string(defaultUserPass))
		if err != nil {
			return nil, fmt.Errorf("failed to instantiate rabbit rabbitmqClient: %v", err)
		}
	}
	return rabbitmqClient, nil
}

func keyMissingErr(key string) error {
	return errors.New(fmt.Sprintf("failed to retrieve %s: key %s missing from credentials", key, key))
}
