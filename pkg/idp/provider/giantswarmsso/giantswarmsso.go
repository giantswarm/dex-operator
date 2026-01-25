package giantswarmsso

import (
	"context"
	"fmt"
	"net/url"
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
	ProviderName        = "giantswarmsso"
	ProviderDisplayName = "Giant Swarm SSO"
	ProviderType        = "oidc"

	// Configuration keys for credentials
	IssuerKey             = "issuer"
	ClientIDKey           = "clientID"
	ClientSecretKey       = "clientSecret"
	CentralClusterNameKey = "centralClusterName"
)

// Config holds the configuration for the Giant Swarm SSO provider.
type Config struct {
	// Issuer is the OIDC issuer URL of the central Dex instance (e.g., "https://dex.gazelle.awsprod.gigantic.io")
	Issuer string
	// ClientID is the OAuth2 client ID registered with the central Dex instance
	ClientID string
	// ClientSecret is the OAuth2 client secret for authentication with the central Dex instance
	ClientSecret string
	// CentralClusterName is the name of the central cluster to skip (e.g., "gazelle")
	CentralClusterName string
}

// GiantSwarmSSO is a provider that creates an OIDC connector pointing to a central
// Giant Swarm Dex instance. This enables RFC 8693 token exchange for cross-cluster SSO.
// The connector allows users authenticated on the central cluster to access other
// management clusters without re-authenticating.
type GiantSwarmSSO struct {
	Log                   logr.Logger
	Name                  string
	Description           string
	Owner                 string
	config                Config
	managementClusterName string
}

var _ provider.Provider = (*GiantSwarmSSO)(nil)

func New(providerConfig provider.ProviderConfig) (*GiantSwarmSSO, error) {
	config, err := newConfig(providerConfig.Credential, providerConfig.Log)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return &GiantSwarmSSO{
		Log:                   providerConfig.Log,
		Name:                  key.GetProviderName(providerConfig.Credential.Owner, ProviderName),
		Description:           providerConfig.Credential.GetConnectorDescription(ProviderDisplayName),
		Owner:                 providerConfig.Credential.Owner,
		config:                config,
		managementClusterName: providerConfig.ManagementClusterName,
	}, nil
}

func newConfig(p provider.ProviderCredential, log logr.Logger) (Config, error) {
	if (logr.Logger{}) == log {
		return Config{}, microerror.Maskf(invalidConfigError, "Logger must not be empty.")
	}
	if p.Name == "" {
		return Config{}, microerror.Maskf(invalidConfigError, "Credential name must not be empty.")
	}
	if p.Owner == "" {
		return Config{}, microerror.Maskf(invalidConfigError, "Credential owner must not be empty.")
	}

	var issuer, clientID, clientSecret, centralClusterName string
	{
		if issuer = p.Credentials[IssuerKey]; issuer == "" {
			return Config{}, microerror.Maskf(invalidConfigError, "%s must not be empty.", IssuerKey)
		}
		// Validate issuer is a valid HTTPS URL
		if err := validateIssuerURL(issuer); err != nil {
			return Config{}, microerror.Mask(err)
		}
		if clientID = p.Credentials[ClientIDKey]; clientID == "" {
			return Config{}, microerror.Maskf(invalidConfigError, "%s must not be empty.", ClientIDKey)
		}
		if clientSecret = p.Credentials[ClientSecretKey]; clientSecret == "" {
			return Config{}, microerror.Maskf(invalidConfigError, "%s must not be empty.", ClientSecretKey)
		}
		if centralClusterName = p.Credentials[CentralClusterNameKey]; centralClusterName == "" {
			return Config{}, microerror.Maskf(invalidConfigError, "%s must not be empty.", CentralClusterNameKey)
		}
	}

	return Config{
		Issuer:             issuer,
		ClientID:           clientID,
		ClientSecret:       clientSecret,
		CentralClusterName: centralClusterName,
	}, nil
}

// validateIssuerURL validates that the issuer is a valid HTTPS URL.
func validateIssuerURL(issuer string) error {
	parsed, err := url.Parse(issuer)
	if err != nil {
		return microerror.Maskf(invalidConfigError, "issuer is not a valid URL: %v", err)
	}
	if parsed.Scheme != "https" {
		return microerror.Maskf(invalidConfigError, "issuer must use HTTPS scheme, got %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return microerror.Maskf(invalidConfigError, "issuer must have a valid host")
	}
	return nil
}

func (g *GiantSwarmSSO) GetName() string {
	return g.Name
}

func (g *GiantSwarmSSO) GetProviderName() string {
	return ProviderName
}

func (g *GiantSwarmSSO) GetType() string {
	return ProviderType
}

func (g *GiantSwarmSSO) GetOwner() string {
	return g.Owner
}

func (g *GiantSwarmSSO) CreateOrUpdateApp(config provider.AppConfig, ctx context.Context, oldConnector dex.Connector) (provider.ProviderApp, error) {
	// Skip the central cluster itself - no token exchange needed for same cluster
	if g.managementClusterName == g.config.CentralClusterName {
		g.Log.Info("Skipping Giant Swarm SSO connector on central cluster itself",
			"centralCluster", g.config.CentralClusterName)
		return provider.ProviderApp{}, nil
	}

	// Build OIDC connector config as YAML
	// Using raw YAML instead of struct because the dex library version doesn't
	// include all fields we need (like insecureEnableGroups for group claims)
	connectorConfig := fmt.Sprintf(`issuer: %s
clientID: %s
clientSecret: %s
redirectURI: %s
insecureEnableGroups: true
scopes:
  - openid
  - profile
  - email
  - groups`, g.config.Issuer, g.config.ClientID, g.config.ClientSecret, config.RedirectURI)

	// Validate the connector config against Dex's OIDC connector schema
	if err := validateConnectorConfig(connectorConfig); err != nil {
		return provider.ProviderApp{}, microerror.Mask(err)
	}

	return provider.ProviderApp{
		Connector: dex.Connector{
			Type:   ProviderType,
			ID:     g.Name,
			Name:   g.Description,
			Config: connectorConfig,
		},
		// Static config, use a far future expiry
		SecretEndDateTime: time.Now().AddDate(10, 0, 0),
	}, nil
}

// validateConnectorConfig validates the OIDC connector configuration against Dex's schema.
func validateConnectorConfig(connectorConfig string) error {
	f, ok := server.ConnectorsConfig[ProviderType]
	if !ok {
		return microerror.Maskf(invalidConfigError, "unknown connector type %q", ProviderType)
	}

	connConfig := f()
	if len(connectorConfig) != 0 {
		data := []byte(connectorConfig)
		if err := yaml.Unmarshal(data, connConfig); err != nil {
			return microerror.Maskf(invalidConfigError, "invalid connector config: %v", err)
		}
	}

	return nil
}

func (g *GiantSwarmSSO) DeleteApp(name string, ctx context.Context) error {
	// No external resources to clean up for static OIDC config
	return nil
}

func (g *GiantSwarmSSO) GetCredentialsForAuthenticatedApp(config provider.AppConfig) (map[string]string, error) {
	// No credentials needed for Giant Swarm SSO provider - it uses static OIDC config
	return map[string]string{}, nil
}

func (g *GiantSwarmSSO) CleanCredentialsForAuthenticatedApp(config provider.AppConfig) error {
	// No credentials to clean up
	return nil
}

func (g *GiantSwarmSSO) DeleteAuthenticatedApp(config provider.AppConfig) error {
	// No authenticated app to delete
	return nil
}

// Self-renewal methods - Giant Swarm SSO provider uses static configuration and doesn't need renewal
func (g *GiantSwarmSSO) SupportsServiceCredentialRenewal() bool {
	return false
}

func (g *GiantSwarmSSO) ShouldRotateServiceCredentials(ctx context.Context, config provider.AppConfig) (bool, error) {
	return false, nil
}

func (g *GiantSwarmSSO) RotateServiceCredentials(ctx context.Context, config provider.AppConfig) (map[string]string, error) {
	return nil, nil
}
