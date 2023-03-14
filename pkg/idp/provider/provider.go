package provider

import (
	"context"
	"giantswarm/dex-operator/pkg/dex"
	"os"
	"time"

	"github.com/giantswarm/microerror"
	"gopkg.in/yaml.v2"
)

type Provider interface {
	CreateOrUpdateApp(AppConfig, context.Context, dex.Connector) (ProviderApp, error)
	DeleteApp(string, context.Context) error
	GetCredentialsForAuthenticatedApp(AppConfig) (map[string]string, error)
	CleanCredentialsForAuthenticatedApp(AppConfig) error
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
