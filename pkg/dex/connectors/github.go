// Package connectors contains local copies of Dex connector configuration types.
//
// Source: https://github.com/dexidp/dex/blob/v2.42.0/connector/github/github.go
package connectors

// GitHubConfig holds configuration options for GitHub logins.
// This is a local copy of the Dex GitHub connector config struct.
type GitHubConfig struct {
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
	RedirectURI  string `json:"redirectURI"`

	// Org is deprecated, use Orgs instead.
	Org string `json:"org,omitempty"`

	// Orgs is the list of organizations and teams to allow.
	Orgs []GitHubOrg `json:"orgs,omitempty"`

	// HostName is the GitHub Enterprise hostname (optional).
	HostName string `json:"hostName,omitempty"`

	// RootCA is the path to a CA certificate file (optional).
	RootCA string `json:"rootCA,omitempty"`

	// TeamNameField specifies which field to use for team names.
	// Can be "name", "slug", or "both". Defaults to "name".
	TeamNameField string `json:"teamNameField,omitempty"`

	// LoadAllGroups loads all groups the user is a member of.
	LoadAllGroups bool `json:"loadAllGroups,omitempty"`

	// UseLoginAsID uses the user's login as their ID instead of numeric ID.
	UseLoginAsID bool `json:"useLoginAsID,omitempty"`

	// PreferredEmailDomain specifies the preferred email domain.
	PreferredEmailDomain string `json:"preferredEmailDomain,omitempty"`
}

// GitHubOrg holds org-team filters, in which teams are optional.
type GitHubOrg struct {
	// Name is the organization name in GitHub (not slug, full name).
	// Only users in this GitHub organization can authenticate.
	Name string `json:"name"`

	// Teams is the list of team names in the organization.
	// A user will be able to authenticate if they are members of at least
	// one of these teams. Users in the organization can authenticate if
	// this field is omitted.
	Teams []string `json:"teams,omitempty"`
}
