package setup

import (
	"fmt"
	"giantswarm/dex-operator/controllers"
	"giantswarm/dex-operator/pkg/idp/provider"

	"os"

	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

const (
	IncludeAll   = "all"
	CleanAction  = "clean"
	UpdateAction = "update"
	CreateAction = "create"
)

type SetupConfig struct {
	Installation   string
	CredentialFile string
	OutputFile     string
	Provider       string
	Action         string
}

type Setup struct {
	providers  []provider.Provider
	appConfig  provider.AppConfig
	config     Config
	action     string
	outputFile string
	log        logr.Logger
}

func New(setup SetupConfig) (*Setup, error) {
	zapLogger, err := zap.NewProduction()
	if err != nil {
		return nil, microerror.Mask(err)
	}
	log := zapr.NewLogger(zapLogger)

	config, err := GetConfigFromFile(setup.CredentialFile)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	providers, err := getProvidersFromConfig(config, setup.Provider, log)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	appConfig := getAppConfigForInstallation(setup.Installation)

	return &Setup{
		providers:  providers,
		appConfig:  appConfig,
		action:     setup.Action,
		config:     config,
		outputFile: setup.OutputFile,
		log:        log,
	}, nil

}

func (s *Setup) Run() error {
	switch s.action {
	case CleanAction:
		err := s.CleanConfigCredentialsForProviders()
		if err != nil {
			return microerror.Mask(err)
		}
		return nil
	case UpdateAction:
		err := s.GetConfigCredentialsForProviders()
		if err != nil {
			return microerror.Mask(err)
		}
		err = s.WriteToFile()
		if err != nil {
			return microerror.Mask(err)
		}
	case CreateAction:
		err := s.GetConfigCredentialsForProviders()
		if err != nil {
			return microerror.Mask(err)
		}
		err = s.WriteToFile()
		if err != nil {
			return microerror.Mask(err)
		}
	default:
		return fmt.Errorf("action %s is not known", s.action)
	}
	return nil
}

func (s *Setup) GetConfigCredentialsForProviders() error {
	config := []OidcOwnerProvider{}
	for _, p := range s.providers {
		credentials, err := p.GetCredentialsForAuthenticatedApp(s.appConfig)
		if err != nil {
			return microerror.Mask(err)
		}
		config = append(config, OidcOwnerProvider{Name: p.GetProviderName(), Credentials: credentials})
	}
	if s.action == UpdateAction {
		s.updateConfig(config)
	}
	if s.action == CreateAction {
		s.createConfig(config)
	}
	return nil
}
func (s *Setup) CleanConfigCredentialsForProviders() error {
	for _, p := range s.providers {
		err := p.CleanCredentialsForAuthenticatedApp(s.appConfig)
		if err != nil {
			return microerror.Mask(err)
		}
	}
	return nil
}

func (s *Setup) WriteToFile() error {
	data, err := yaml.Marshal(s.config)
	if err != nil {
		return microerror.Mask(err)
	}
	return os.WriteFile(s.outputFile, data, 0600)
}

func getProvidersFromConfig(credentials Config, include string, log logr.Logger) ([]provider.Provider, error) {

	providers := []provider.Provider{}
	// We are only returning the giantswarm providers. Either all or a specific one.
	for _, p := range credentials.Oidc.Giantswarm.Providers {
		if include == IncludeAll || include == p.Name {
			c := map[string]string{}
			if err := yaml.Unmarshal([]byte(p.Credentials), &c); err != nil {
				return nil, microerror.Mask(err)
			}
			provider, err := controllers.NewProvider(provider.ProviderCredential{Name: p.Name, Owner: "giantswarm", Credentials: c}, &log)
			if err != nil {
				return nil, microerror.Mask(err)
			}
			if providerAlreadyPresent(providers, provider) {
				return nil, microerror.Mask(fmt.Errorf("more than one provider with name %s", provider.GetName()))
			}
			providers = append(providers, provider)
		}
	}
	return providers, nil
}

func (s *Setup) updateConfig(newCredentials []OidcOwnerProvider) {
	for i, p := range s.config.Oidc.Giantswarm.Providers {
		for _, c := range newCredentials {
			if p.Name == c.Name {
				s.config.Oidc.Giantswarm.Providers[i].Credentials = c.Credentials
			}
		}
	}
}
func (s *Setup) createConfig(newCredentials []OidcOwnerProvider) {
	s.config = Config{}
	s.config.Oidc.Giantswarm.Providers = newCredentials
}

func providerAlreadyPresent(providers []provider.Provider, provider provider.Provider) bool {
	for _, p := range providers {
		if p.GetName() == provider.GetName() {
			return true
		}
	}
	return false
}

func getAppConfigForInstallation(installation string) provider.AppConfig {
	return provider.AppConfig{
		Name:                 fmt.Sprintf("dex-operator-%s", installation),
		SecretValidityMonths: 6,
	}
}