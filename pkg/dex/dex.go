package dex

import "github.com/dexidp/dex/server"

type DexConfig struct {
	Oidc DexOidc `json:"oidc"`
}

type DexOidc struct {
	Giantswarm DexOidcOwner `json:"giantswarm,omitempty"`
	Customer   DexOidcOwner `json:"customer,omitempty"`
}

type DexOidcOwner struct {
	Connectors []Connector `json:"connectors"`
}

type Connector struct {
	Type string `json:"connectorType"`
	Name string `json:"connectorName"`
	ID   string `json:"id"`

	Config server.ConnectorConfig `json:"connectorConfig"`
}
