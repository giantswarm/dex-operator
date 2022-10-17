package provider

import (
	"giantswarm/dex-operator/pkg/dex"
	"os"

	"gopkg.in/yaml.v2"
)

type Provider interface {
	CreateApp(AppConfig) (dex.Connector, error)
	DeleteApp(string) error
}

type AppConfig struct {
	RedirectURI string
	Name        string
}

type ProviderCredential struct {
	Name        string            `yaml:"name"`
	Credentials map[string]string `yaml:"credentials"`
}

func ReadCredentials(fileLocation string) ([]ProviderCredential, error) {
	credentials := &[]ProviderCredential{}

	file, err := os.ReadFile(fileLocation)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(file, credentials); err != nil {
		return nil, err
	}

	return *credentials, nil
}
