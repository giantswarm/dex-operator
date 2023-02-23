package provider

import (
	"context"
	"fmt"
	"giantswarm/dex-operator/pkg/dex"
	"os"
	"time"

	"github.com/giantswarm/microerror"
	"gopkg.in/yaml.v2"
)

type Provider interface {
	CreateOrUpdateApp(AppConfig, context.Context, dex.Connector) (ProviderApp, error)
	DeleteApp(string, context.Context) error
	GetCredentialsForAuthenticatedApp(AppConfig) (string, error)
	CleanCredentialsForAuthenticatedApp(AppConfig) error
	GetName() string
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

func GetConfigCredentialsForProviders(config AppConfig, providers []Provider) error {
	for _, p := range providers {
		credentials, err := p.GetCredentialsForAuthenticatedApp(config)
		if err != nil {
			return err
		}
		fmt.Print(credentials)
	}
	return nil
}
func CleanConfigCredentialsForProviders(config AppConfig, providers []Provider) error {
	for _, p := range providers {
		err := p.CleanCredentialsForAuthenticatedApp(config)
		if err != nil {
			return err
		}
	}
	return nil
}
