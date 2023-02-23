package setup

import (
	"fmt"
	"giantswarm/dex-operator/controllers"
	"giantswarm/dex-operator/pkg/idp/provider"
	"os"

	"github.com/giantswarm/microerror"
	"gopkg.in/yaml.v2"
)

const (
	IncludeAll   = "all"
	CleanAction  = "clean"
	CreateAction = "create"
)

type Config struct {
	Oidc Oidc `json:"oidc"`
}
type Oidc struct {
	Giantswarm *OidcOwner `json:"giantswarm,omitempty"`
	Customer   *OidcOwner `json:"customer,omitempty"`
}
type OidcOwner struct {
	Providers []OidcOwnerProvider `json:"providers,omitempty"`
}
type OidcOwnerProvider struct {
	Name        string `yaml:"name"`
	Credentials string `yaml:"credentials"`
}

type Setup struct {
	Installation   string
	CredentialFile string
	Provider       string
	Action         string
	AppName        string
}

func getProvidersFromConfig(fileLocation string, include string) ([]provider.Provider, error) {
	credentials := &Config{}

	file, err := os.ReadFile(fileLocation)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	if err := yaml.Unmarshal(file, credentials); err != nil {
		return nil, microerror.Mask(err)
	}

	providers := []provider.Provider{}
	{
		for _, p := range credentials.Oidc.Giantswarm.Providers {
			if include == IncludeAll || include == p.Name {
				c := map[string]string{}
				if err := yaml.Unmarshal([]byte(p.Credentials), &c); err != nil {
					return nil, microerror.Mask(err)
				}
				provider, err := controllers.NewProvider(provider.ProviderCredential{Name: p.Name, Owner: "giantswarm", Credentials: c}, provider.GetTestLogger())
				if err != nil {
					return nil, microerror.Mask(err)
				}
				providers = append(providers, provider)
			}
		}
	}
	return providers, nil
}

func CredentialSetup(setup Setup) error {
	providers, err := getProvidersFromConfig(setup.CredentialFile, setup.Provider)
	if err != nil {
		return err
	}
	config := getAppConfig(setup.AppName, setup.Installation)
	switch setup.Action {
	case CleanAction:
		err = provider.CleanConfigCredentialsForProviders(config, providers)
		if err != nil {
			return err
		}
		return nil
	case CreateAction:
		err = provider.GetConfigCredentialsForProviders(config, providers)
		if err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("action %s is not known.", setup.Action)
	}
}

func getAppConfig(appName string, installation string) provider.AppConfig {
	return provider.AppConfig{
		Name:                 fmt.Sprintf("%s-%s", appName, installation),
		SecretValidityMonths: 6,
	}
}
