package dex

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
)

type DexConfig struct {
	Oidc DexOidc `json:"oidc"`
}

type DexOidc struct {
	Giantswarm *DexOidcOwner `json:"giantswarm,omitempty"`
	Customer   *DexOidcOwner `json:"customer,omitempty"`
}

func (d DexOidc) NeedsMigration() bool {
	return (d.Giantswarm != nil && d.Giantswarm.legacyConnectors) ||
		(d.Customer != nil && d.Customer.legacyConnectors)
}

type DexOidcOwner struct {
	Connectors       []Connector `json:"connectors,omitempty"`
	legacyConnectors bool
}

func (d DexOidcOwner) MarshalJSON() ([]byte, error) {
	connectors := make(map[string]Connector, len(d.Connectors))
	for _, connector := range d.Connectors {
		connectors[connector.ID] = connector
	}
	return json.Marshal(struct {
		Connectors map[string]Connector `json:"connectors,omitempty"`
	}{Connectors: connectors})
}

func (d *DexOidcOwner) UnmarshalJSON(data []byte) error {
	var owner struct {
		Connectors json.RawMessage `json:"connectors"`
	}
	if err := json.Unmarshal(data, &owner); err != nil {
		return err
	}
	decoded := DexOidcOwner{}
	connectors := bytes.TrimSpace(owner.Connectors)
	if len(connectors) > 0 && connectors[0] == '[' {
		if err := json.Unmarshal(connectors, &decoded.Connectors); err != nil {
			return err
		}
		decoded.legacyConnectors = true
	} else if len(connectors) > 0 {
		var entries map[string]Connector
		if err := json.Unmarshal(connectors, &entries); err != nil {
			return err
		}
		for id, connector := range entries {
			connector.ID = id
			decoded.Connectors = append(decoded.Connectors, connector)
		}
		slices.SortFunc(decoded.Connectors, func(a, b Connector) int {
			return strings.Compare(a.ID, b.ID)
		})
	}
	*d = decoded
	return nil
}

type Connector struct {
	Type string `json:"connectorType"`
	Name string `json:"connectorName"`
	ID   string `json:"id"`

	Config string `json:"connectorConfig"`
}
