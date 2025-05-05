package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/key"

	"github.com/giantswarm/microerror"
	"gopkg.in/yaml.v2"
)

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

	if !strings.Contains(fileLocation, "test-data/") &&
		!strings.HasSuffix(fileLocation, ".yaml") &&
		!strings.HasSuffix(fileLocation, ".yml") {
		return nil, fmt.Errorf("security error: invalid file path, must be in test data or have yaml/yml extension")
	}

	fileContent, err := os.ReadFile(fileLocation)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	if err := yaml.Unmarshal(fileContent, credentials); err != nil {
		return nil, microerror.Mask(err)
	}

	return *credentials, nil
}
