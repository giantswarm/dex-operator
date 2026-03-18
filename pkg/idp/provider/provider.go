package provider

import (
	"context"
	"os"
	"time"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/key"

	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
)

type ProviderConfig struct {
	Credential            ProviderCredential
	Log                   logr.Logger
	ManagementClusterName string
}

type Provider interface {
	CreateOrUpdateApp(AppConfig, context.Context, dex.Connector) (ProviderApp, error)
	DeleteApp(string, context.Context) error
	GetCredentialsForAuthenticatedApp(AppConfig) (map[string]string, error)
	CleanCredentialsForAuthenticatedApp(AppConfig) error
	DeleteAuthenticatedApp(AppConfig) error
	GetName() string
	GetProviderName() string
	GetOwner() string
	GetType() string

	// Self-renewal methods - all providers must implement these
	// Providers that don't support renewal should return false from SupportsServiceCredentialRenewal()
	SupportsServiceCredentialRenewal() bool
	ShouldRotateServiceCredentials(ctx context.Context, config AppConfig) (bool, error)
	RotateServiceCredentials(ctx context.Context, config AppConfig) (map[string]string, error)
}

type AppConfig struct {
	RedirectURI          string
	Name                 string
	IdentifierURI        string
	SecretValidityMonths int
}

type ProviderCredential struct {
	Name        string            `yaml:"name"`
	Owner       string            `yaml:"owner"`
	Credentials map[string]string `yaml:"credentials"`
	Description string            `yaml:"description"`
}

func (c ProviderCredential) GetConnectorDescription(providerDisplayName string) string {
	if c.Description != "" {
		return c.Description
	}
	return key.GetDefaultConnectorDescription(providerDisplayName, c.Owner)
}

type ProviderApp struct {
	Connector         dex.Connector
	SecretEndDateTime time.Time
}

type ProviderSecret struct {
	ClientId     string
	ClientSecret string
	EndDateTime  time.Time
}

func ReadCredentials(fileLocation string) ([]ProviderCredential, error) {
	credentials := &[]ProviderCredential{}

	file, err := os.ReadFile(fileLocation) //nolint:gosec,G304
	if err != nil {
		return nil, microerror.Mask(err)
	}

	if err := yaml.Unmarshal(file, credentials); err != nil {
		return nil, microerror.Mask(err)
	}

	return *credentials, nil
}
