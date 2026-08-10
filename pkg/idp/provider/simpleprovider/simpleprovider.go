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
	"gopkg.in/yaml.v3"

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

	// CredentialKeyConnectorID is an optional credentials key. When set and
	// non-empty, it overrides the auto-derived connector ID
	// (`<owner>-simple-<connectorType>`) with the literal value provided.
	// Used by the Giant Swarm SSO federation connector (PLAN §6 TB-5) which
	// must land as `id: giantswarm` to match production MCPServer CRs.
	// Validated against the same identifier pattern Muster's
	// MCPServerAuth.TokenExchange.ConnectorID enforces.
	CredentialKeyConnectorID = "connectorId"

	// CredentialKeyCentralCluster is an optional credentials key naming the
	// management cluster whose Dex this connector federates to. When the
	// operator's own --management-cluster matches this value, the provider is
	// skipped to avoid writing a self-referencing connector (PLAN §6 TB-5).
	CredentialKeyCentralCluster = "centralCluster"
)

// connectorIDPattern matches the identifier pattern enforced by Muster's
// MCPServerAuth.TokenExchange.ConnectorID (PLAN §5 Gap D).
var connectorIDPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// The simple provider is a provider that can be used if no idp access is configured for the operator.
// It can also be used in case the idp in question is not supported by the operator.
// It will create a connector with the given type and config.
//
// Recognised credentials keys:
//   - connectorType   (required) - dex connector type (e.g. "oidc", "ldap")
//   - connectorConfig (required) - YAML body of the dex connector config
//   - connectorId     (optional) - literal connector ID override; defaults to
//     "<owner>-simple-<connectorType>" when unset
//   - centralCluster  (optional) - skip this provider when the operator runs
//     on this management cluster (avoids self-referencing federation
//     connectors; see PLAN §6 TB-5)
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
	connectorID     string
}

var _ provider.Provider = (*SimpleProvider)(nil)

func New(config provider.ProviderConfig) (*SimpleProvider, error) {
	// Central-cluster skip (PLAN §6 TB-5): if this credential targets the
	// management cluster the operator itself runs on, do not create a
	// connector — that would be a self-referencing federation connector.
	// Returning (nil, nil) tells the caller to skip this credential.
	if central := config.Credential.Credentials[CredentialKeyCentralCluster]; central != "" &&
		config.ManagementClusterName != "" &&
		central == config.ManagementClusterName {
		if (logr.Logger{}) != config.Log {
			config.Log.Info(fmt.Sprintf(
				"Skipping simpleprovider credential %q: centralCluster %q matches operator's management cluster (avoids self-referencing connector)",
				config.Credential.Name, central))
		}
		return nil, nil
	}

	// get configuration from credentials
	c, err := newSimpleConfig(config.Credential, config.Log)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	// Connector ID: prefer the explicit override when provided, otherwise
	// fall back to today's auto-derived `<owner>-simple-<connectorType>`.
	name := c.connectorID
	if name == "" {
		name = key.GetProviderName(config.Credential.Owner, fmt.Sprintf("%s-%s", ProviderName, c.connectorType))
	}

	return &SimpleProvider{
		Name:            name,
		Description:     config.Credential.GetConnectorDescription(ProviderDisplayName),
		Type:            ProviderType,
		Owner:           config.Credential.Owner,
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
			return Config{}, microerror.Maskf(invalidConfigError, "%s must not be empty", connectorTypeKey)
		}

		if connectorConfig = p.Credentials[connectorConfigKey]; connectorConfig == "" {
			return Config{}, microerror.Maskf(invalidConfigError, "%s must not be empty", connectorConfigKey)
		}
	}

	// Optional connector ID override. When unset, the caller derives the ID
	// from owner + connector type (today's behaviour). When set, we validate
	// the format eagerly so misconfiguration surfaces at startup rather than
	// causing dex to reject the secret on reload.
	connectorID := p.Credentials[CredentialKeyConnectorID]
	if connectorID != "" && !connectorIDPattern.MatchString(connectorID) {
		return Config{}, microerror.Maskf(invalidConfigError,
			"%s %q is invalid: must match %s", CredentialKeyConnectorID, connectorID, connectorIDPattern.String())
	}

	return Config{
		connectorType:   connectorType,
		connectorConfig: connectorConfig,
		connectorID:     connectorID,
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

// Self-renewal methods implementation - SimpleProvider doesn't support renewal
func (s *SimpleProvider) SupportsServiceCredentialRenewal() bool {
	return false
}

func (s *SimpleProvider) ShouldRotateServiceCredentials(ctx context.Context, config provider.AppConfig) (bool, error) {
	return false, nil
}

func (s *SimpleProvider) RotateServiceCredentials(ctx context.Context, config provider.AppConfig) (map[string]string, error) {
	return nil, microerror.Maskf(invalidConfigError, "Simple provider does not support service credential rotation")
}
