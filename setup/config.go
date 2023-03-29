package setup

import (
	"os"

	"github.com/giantswarm/microerror"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Oidc Oidc `yaml:"oidc,omitempty"`
}
type Oidc struct {
	Giantswarm OidcOwner `yaml:"giantswarm,omitempty"`
	Customer   OidcOwner `yaml:"customer,omitempty"`
}
type OidcOwner struct {
	Providers []OidcOwnerProvider `json:"providers,omitempty"`
}
type OidcOwnerProvider struct {
	Name        string `yaml:"name"`
	Credentials string `yaml:"credentials"`
}

func GetConfigFromFile(fileLocation string) (Config, error) {
	credentials := &Config{}

	file, err := os.ReadFile(fileLocation)
	if err != nil {
		return *credentials, microerror.Maskf(invalidConfigError, "Failed to get config from file: %s", err)
	}

	if err := yaml.Unmarshal(file, credentials); err != nil {
		return *credentials, microerror.Maskf(invalidConfigError, "Failed to get config from file: %s", err)
	}

	return *credentials, nil
}
