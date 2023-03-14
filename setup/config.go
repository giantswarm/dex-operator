package setup

import (
	"os"

	"github.com/giantswarm/microerror"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Oidc Oidc `json:"oidc"`
}
type Oidc struct {
	Giantswarm OidcOwner `json:"giantswarm"`
	Customer   OidcOwner `json:"customer"`
}
type OidcOwner struct {
	Providers []OidcOwnerProvider `json:"providers"`
}
type OidcOwnerProvider struct {
	Name        string            `json:"name"`
	Credentials map[string]string `json:"credentials"`
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
