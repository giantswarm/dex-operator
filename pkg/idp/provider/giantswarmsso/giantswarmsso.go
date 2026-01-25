package giantswarmsso

import (
	"context"
	"net/url"
	"time"

	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/dex/connectors"
	"github.com/giantswarm/dex-operator/pkg/idp/provider"
	"github.com/giantswarm/dex-operator/pkg/key"
	"github.com/giantswarm/dex-operator/pkg/yaml"
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

	// staticConfigExpiryYears is the number of years until static OIDC config expires.
	// Since this provider uses static configuration (no dynamic secrets), we use a
	// far-future expiry to avoid unnecessary rotation.
	staticConfigExpiryYears = 10
)

// Config holds the configuration for the Giant Swarm SSO provider.
type Config struct {
	// Issuer is the OIDC issuer URL of the central Dex instance (e.g., "https://dex.central.example.com")
	Issuer string
	// ClientID is the OAuth2 client ID registered with the central Dex instance
	ClientID string
	// ClientSecret is the OAuth2 client secret for authentication with the central Dex instance
	ClientSecret string
	// CentralClusterName is the name of the central cluster to skip (e.g., "central")
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
		// clientID and clientSecret are optional for RFC 8693 token exchange flow.
		// For token exchange, the OIDC connector only needs the issuer to validate
		// subject tokens. Client credentials are configured separately in staticClients.
		// However, if provided, they will be included in the connector config for
		// standard OIDC authorization code flow support.
		clientID = p.Credentials[ClientIDKey]
		clientSecret = p.Credentials[ClientSecretKey]
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

	// Build OIDC connector config using the Dex OIDC connector struct.
	// For RFC 8693 token exchange, clientID and clientSecret are NOT required
	// in the connector config - the issuer alone is sufficient to validate
	// subject tokens. Client credentials are configured separately in staticClients.
	// However, we include them if provided for standard OIDC flow support.
	connectorConfig := g.buildConnectorConfig(config.RedirectURI)

	// Use MarshalWithJsonAnnotations to preserve field names like "clientID", "redirectURI"
	// as defined in the Dex OIDC connector struct's JSON tags.
	data, err := yaml.MarshalWithJsonAnnotations(connectorConfig)
	if err != nil {
		return provider.ProviderApp{}, microerror.Mask(err)
	}

	return provider.ProviderApp{
		Connector: dex.Connector{
			Type:   ProviderType,
			ID:     g.Name,
			Name:   g.Description,
			Config: string(data),
		},
		SecretEndDateTime: time.Now().AddDate(staticConfigExpiryYears, 0, 0),
	}, nil
}

// buildConnectorConfig creates the OIDC connector configuration.
// For RFC 8693 token exchange, clientID and clientSecret are optional.
func (g *GiantSwarmSSO) buildConnectorConfig(redirectURI string) *connectors.OIDCConfig {
	// InsecureEnableGroups: Despite the "insecure" naming, this setting is safe and
	// required for our use case. The name stems from Dex issue #1065 which identified
	// that blindly trusting group claims from arbitrary upstream providers could be
	// risky. However, in Giant Swarm's cross-cluster SSO setup:
	// - The upstream provider is our own trusted central Dex instance
	// - Group claims are essential for Kubernetes RBAC (e.g., oidc-admins, team-based access)
	// - All management clusters are under Giant Swarm's control
	// Without this setting, group claims would be stripped from tokens, breaking authorization.
	// See: https://github.com/dexidp/dex/issues/1065
	return &connectors.OIDCConfig{
		Issuer:       g.config.Issuer,
		ClientID:     g.config.ClientID,
		ClientSecret: g.config.ClientSecret,
		RedirectURI:  redirectURI,
		Scopes:       []string{"openid", "profile", "email", "groups"},
		// InsecureEnableGroups enables passing through group claims from the upstream
		// OIDC provider. This is required for Kubernetes RBAC based on group membership.
		InsecureEnableGroups: true,
	}
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
