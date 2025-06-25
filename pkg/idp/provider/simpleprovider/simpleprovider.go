package simpleprovider

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dexidp/dex/server"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	"gopkg.in/yaml.v2"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/idp/provider"
	"github.com/giantswarm/dex-operator/pkg/key"
)

const (
	ProviderName        = "simple"
	ProviderDisplayName = "Simple Provider"
	ProviderType        = "simple"
	connectorTypeKey    = "connectorType"
	connectorConfigKey  = "connectorConfig"
)

// The simple provider is a provider that can be used if no idp access is configured for the operator.
// It can also be used in case the idp in question is not supported by the operator.
// It will create a connector with the given type and config.
type SimpleProvider struct {
	Log             logr.Logger
	Name            string
	Description     string
	Type            string
	Owner           string
	ConnectorType   string
	ConnectorConfig string
}

type Config struct {
	connectorType   string
	connectorConfig string
}

func New(p provider.ProviderCredential, log logr.Logger) (*SimpleProvider, error) {
	// get configuration from credentials
	c, err := newSimpleConfig(p, log)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return &SimpleProvider{
		Name:            key.GetProviderName(p.Owner, fmt.Sprintf("%s-%s", ProviderName, c.connectorType)),
		Description:     p.GetConnectorDescription(ProviderDisplayName),
		Type:            ProviderType,
		Owner:           p.Owner,
		ConnectorType:   c.connectorType,
		ConnectorConfig: c.connectorConfig,
	}, nil
}

func newSimpleConfig(p provider.ProviderCredential, log logr.Logger) (Config, error) {
	if (logr.Logger{}) == log {
		return Config{}, microerror.Maskf(invalidConfigError, "Logger must not be empty.")
	}
	if p.Name == "" {
		return Config{}, microerror.Maskf(invalidConfigError, "Credential name must not be empty.")
	}
	if p.Owner == "" {
		return Config{}, microerror.Maskf(invalidConfigError, "Credential owner must not be empty.")
	}

	var connectorType, connectorConfig string
	{
		if connectorType = p.Credentials[connectorTypeKey]; connectorType == "" {
			return Config{}, microerror.Maskf(invalidConfigError, fmt.Sprintf("%s must not be empty.", connectorTypeKey))
		}

		if connectorConfig = p.Credentials[connectorConfigKey]; connectorConfig == "" {
			return Config{}, microerror.Maskf(invalidConfigError, fmt.Sprintf("%s must not be empty.", connectorConfigKey))
		}
	}

	return Config{
		connectorType:   connectorType,
		connectorConfig: connectorConfig,
	}, nil
}

func (s *SimpleProvider) GetName() string {
	return s.Name
}

func (s *SimpleProvider) GetProviderName() string {
	return ProviderName
}

func (s *SimpleProvider) GetType() string {
	return s.Type
}

func (s *SimpleProvider) GetOwner() string {
	return s.Owner
}

func (s *SimpleProvider) CreateOrUpdateApp(config provider.AppConfig, ctx context.Context, oldConnector dex.Connector) (provider.ProviderApp, error) {
	// Inject the redirect URI into the connector config
	connectorConfig := s.injectRedirectURI(config.RedirectURI)

	// validate that the connector type is an existing type and that connector config is valid for the given connector type
	err := s.validateConnectorConfig(connectorConfig)
	if err != nil {
		return provider.ProviderApp{}, microerror.Mask(err)
	}

	return provider.ProviderApp{
		Connector: dex.Connector{
			Type:   s.ConnectorType,
			ID:     s.Name,
			Name:   s.Description,
			Config: connectorConfig,
		},
		SecretEndDateTime: time.Now().AddDate(0, 6, 0),
	}, nil
}

func (s *SimpleProvider) DeleteApp(name string, ctx context.Context) error {
	return nil
}

func (s *SimpleProvider) GetCredentialsForAuthenticatedApp(config provider.AppConfig) (map[string]string, error) {
	s.Log.Info(fmt.Sprintf("No new credentials will be created for the %s provider because it does not allow dex-operator access.", ProviderName))
	return map[string]string{}, nil
}
func (s *SimpleProvider) CleanCredentialsForAuthenticatedApp(config provider.AppConfig) error {
	return nil
}

func (s *SimpleProvider) DeleteAuthenticatedApp(config provider.AppConfig) error {
	return nil
}

func (s *SimpleProvider) injectRedirectURI(redirectURI string) string {
	config := s.ConnectorConfig

	if !usesRedirectURI(s.ConnectorType) {
		return config
	}

	// if a redirect URI is already defined in the config, we replace it with the new one
	if strings.Contains(config, "redirectURI") {
		regexp := regexp.MustCompile(`redirectURI:.*`)
		config = regexp.ReplaceAllString(config, fmt.Sprintf("redirectURI: %s", redirectURI))
	} else {
		config = fmt.Sprintf("%s\nredirectURI: %s", config, redirectURI)
	}
	return config
}

func (s *SimpleProvider) validateConnectorConfig(connectorConfig string) error {
	// validate that the connector type is an existing type
	f, ok := server.ConnectorsConfig[s.ConnectorType]
	if !ok {
		return microerror.Maskf(invalidConfigError, "Unknown connector type %q", s.ConnectorType)
	}

	// validate that connector config is valid for the given connector type
	connConfig := f()
	if len(connectorConfig) != 0 {
		data := []byte(connectorConfig)
		if err := yaml.Unmarshal(data, connConfig); err != nil {
			return microerror.Maskf(invalidConfigError, "Parse connector config: %v", err)
		}
	}

	return nil
}

func usesRedirectURI(connectorType string) bool {
	switch connectorType {
	case "ldap", "authproxy", "atlassian-crowd", "keystone":
		return false
	default:
		return true
	}
}
