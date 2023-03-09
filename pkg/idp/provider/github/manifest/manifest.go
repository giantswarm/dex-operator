package manifest

import (
	"giantswarm/dex-operator/pkg/idp/provider"
)

type Manifest struct {
	Name        string            `json:"name"`
	Permissions map[string]string `json:"default_permissions,omitempty"`
	RedirectURL string            `json:"redirect_url"`
	URL         string            `json:"url"`
}

func NewManifest(config provider.AppConfig) Manifest {
	return Manifest{
		Name:        config.Name,
		Permissions: getPermissions(),
		URL:         config.IdentifierURI,
	}
}

func getPermissions() map[string]string {
	return map[string]string{}
}
