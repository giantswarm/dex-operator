// Package connectors contains local copies of Dex connector configuration types.
// These types are copied from github.com/dexidp/dex to avoid dependency issues
// with Dex's broken Go module versioning (v2.x tags without /v2 module path).
//
// The types are used purely for YAML serialization of connector configs.
// The actual Dex instances validate these configs at runtime.
//
// Source: https://github.com/dexidp/dex/blob/v2.42.0/connector/oidc/oidc.go
package connectors

// OIDCConfig holds configuration options for OpenID Connect logins.
// This is a local copy of the Dex OIDC connector config struct.
type OIDCConfig struct {
	Issuer      string `json:"issuer"`
	IssuerAlias string `json:"issuerAlias,omitempty"`
	ClientID    string `json:"clientID"`
	// ClientSecret is optional for RFC 8693 token exchange flow.
	ClientSecret string `json:"clientSecret,omitempty"`
	RedirectURI  string `json:"redirectURI"`

	// ProviderDiscoveryOverrides allows overriding discovered OIDC endpoints.
	ProviderDiscoveryOverrides *OIDCProviderDiscoveryOverrides `json:"providerDiscoveryOverrides,omitempty"`

	// BasicAuthUnsupported causes client_secret to be passed as POST parameters
	// instead of basic auth. This is specifically "NOT RECOMMENDED" by the OAuth2
	// RFC, but some providers require it.
	BasicAuthUnsupported *bool `json:"basicAuthUnsupported,omitempty"`

	// Scopes defaults to "profile" and "email".
	Scopes []string `json:"scopes,omitempty"`

	// HostedDomains is deprecated and will be removed in future releases.
	HostedDomains []string `json:"hostedDomains,omitempty"`

	// RootCAs are certificates for SSL validation.
	RootCAs []string `json:"rootCAs,omitempty"`

	// InsecureSkipEmailVerified overrides the value of email_verified to true.
	InsecureSkipEmailVerified bool `json:"insecureSkipEmailVerified,omitempty"`

	// InsecureEnableGroups enables groups claims.
	// Despite the "insecure" naming, this setting is safe for trusted upstream
	// providers like Giant Swarm's central Dex. The name stems from Dex issue #1065
	// which identified that blindly trusting group claims from arbitrary upstream
	// providers could be risky. For cross-cluster SSO with a trusted central Dex,
	// this is required for Kubernetes RBAC based on group membership.
	// See: https://github.com/dexidp/dex/issues/1065
	InsecureEnableGroups bool `json:"insecureEnableGroups,omitempty"`

	// AllowedGroups filters which groups are passed through.
	AllowedGroups []string `json:"allowedGroups,omitempty"`

	// AcrValues specifies Authentication Context Class Reference Values.
	AcrValues []string `json:"acrValues,omitempty"`

	// InsecureSkipVerify disables certificate verification.
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`

	// GetUserInfo uses the userinfo endpoint to get additional claims.
	GetUserInfo bool `json:"getUserInfo,omitempty"`

	UserIDKey   string `json:"userIDKey,omitempty"`
	UserNameKey string `json:"userNameKey,omitempty"`

	// PromptType will be used for the prompt parameter.
	PromptType *string `json:"promptType,omitempty"`

	// OverrideClaimMapping allows overriding default claim mappings.
	OverrideClaimMapping bool `json:"overrideClaimMapping,omitempty"`

	// ClaimMapping allows customizing claim field names.
	ClaimMapping *OIDCClaimMapping `json:"claimMapping,omitempty"`
}

// OIDCProviderDiscoveryOverrides allows overriding discovered OIDC endpoints.
type OIDCProviderDiscoveryOverrides struct {
	TokenURL string `json:"tokenURL,omitempty"`
	AuthURL  string `json:"authURL,omitempty"`
	JwksURL  string `json:"jwksURL,omitempty"`
}

// OIDCClaimMapping allows customizing claim field names.
type OIDCClaimMapping struct {
	PreferredUsernameKey string `json:"preferred_username,omitempty"`
	EmailKey             string `json:"email,omitempty"`
	GroupsKey            string `json:"groups,omitempty"`
}
