package provider

import (
	"context"
	"os"
	"time"

	"github.com/giantswarm/dex-operator/pkg/app"
	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/key"

	"github.com/giantswarm/microerror"
	"gopkg.in/yaml.v2"
)

type Provider interface {
	CreateOrUpdateApp(app.Config, context.Context, dex.Connector) (ProviderApp, error)
	DeleteApp(string, context.Context) error
	GetCredentialsForAuthenticatedApp(config app.Config) (map[string]string, error)
	CleanCredentialsForAuthenticatedApp(config app.Config) error
	DeleteAuthenticatedApp(config app.Config) error
	GetName() string
	GetProviderName() string
	GetOwner() string
	GetType() string
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

	file, err := os.ReadFile(fileLocation)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	if err := yaml.Unmarshal(file, credentials); err != nil {
		return nil, microerror.Mask(err)
	}

	return *credentials, nil
}
