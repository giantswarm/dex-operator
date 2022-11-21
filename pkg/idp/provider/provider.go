package provider

import (
	"context"
	"giantswarm/dex-operator/pkg/dex"
	"os"

	"github.com/giantswarm/microerror"
	"gopkg.in/yaml.v2"
)

type Provider interface {
	CreateOrUpdateApp(AppConfig, context.Context) (dex.Connector, error)
	DeleteApp(string, context.Context) error
	GetName() string
	GetOwner() string
	GetType() string
}

type AppConfig struct {
	RedirectURI   string
	Name          string
	IdentifierURI string
}

type ProviderCredential struct {
	Name        string            `yaml:"name"`
	Owner       string            `yaml:"owner"`
	Credentials map[string]string `yaml:"credentials"`
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
