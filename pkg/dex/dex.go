package dex

type DexConfig struct {
	Oidc DexOidc `json:"oidc"`
}

type DexOidc struct {
	Giantswarm *DexOidcOwner `json:"giantswarm,omitempty"`
	Customer   *DexOidcOwner `json:"customer,omitempty"`
}

type DexOidcOwner struct {
	Connectors []Connector `json:"connectors,omitempty"`
}

type Connector struct {
	Type string `json:"connectorType"`
	Name string `json:"connectorName"`
	ID   string `json:"id"`

	Config string `json:"connectorConfig"`
}
