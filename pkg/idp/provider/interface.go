package provider

import "github.com/dexidp/dex/server"

type Provider interface {
	CreateApp() (Connector, error)
	DeleteApp() error
}

type Connector struct {
	Type string `json:"type"`
	Name string `json:"name"`
	ID   string `json:"id"`

	Config server.ConnectorConfig `json:"config"`
}
