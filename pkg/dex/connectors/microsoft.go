// Package connectors contains local copies of Dex connector configuration types.
//
// Source: https://github.com/dexidp/dex/blob/v2.42.0/connector/microsoft/microsoft.go
package connectors

// MicrosoftConfig holds configuration options for Microsoft/Azure AD logins.
// This is a local copy of the Dex Microsoft connector config struct.
type MicrosoftConfig struct {
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
	RedirectURI  string `json:"redirectURI"`
	Tenant       string `json:"tenant"`

	// OnlySecurityGroups only returns security groups if true.
	OnlySecurityGroups bool `json:"onlySecurityGroups,omitempty"`

	// Groups are the list of groups to filter.
	Groups []string `json:"groups,omitempty"`

	// GroupNameFormat specifies the format of the group name.
	GroupNameFormat string `json:"groupNameFormat,omitempty"`

	// UseGroupsAsWhitelist uses groups as a whitelist.
	UseGroupsAsWhitelist bool `json:"useGroupsAsWhitelist,omitempty"`

	// EmailToLowercase converts email to lowercase.
	EmailToLowercase bool `json:"emailToLowercase,omitempty"`

	// APIURL is the Microsoft API URL (optional).
	APIURL string `json:"apiURL,omitempty"`

	// GraphURL is the Microsoft Graph API URL (optional).
	GraphURL string `json:"graphURL,omitempty"`

	// PromptType is used for the prompt query parameter.
	PromptType string `json:"promptType,omitempty"`

	// DomainHint is used for the domain_hint query parameter.
	DomainHint string `json:"domainHint,omitempty"`

	// Scopes defaults to user.read.
	Scopes []string `json:"scopes,omitempty"`
}
