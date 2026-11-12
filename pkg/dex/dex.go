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
	Giantswarm                 *DexOidcOwner `json:"giantswarm,omitempty"`
	Customer                   *DexOidcOwner `json:"customer,omitempty"`
	legacyGiantswarmConnectors bool
}

func (d DexOidc) MarshalJSON() ([]byte, error) {
	type mapOwner struct {
		Connectors map[string]Connector `json:"connectors,omitempty"`
	}
	var giantswarm *mapOwner
	if d.Giantswarm != nil {
		giantswarm = &mapOwner{Connectors: make(map[string]Connector, len(d.Giantswarm.Connectors))}
		for _, connector := range d.Giantswarm.Connectors {
			giantswarm.Connectors[connector.ID] = connector
		}
	}
	return json.Marshal(struct {
		Giantswarm *mapOwner     `json:"giantswarm,omitempty"`
		Customer   *DexOidcOwner `json:"customer,omitempty"`
	}{Giantswarm: giantswarm, Customer: d.Customer})
}

func (d *DexOidc) UnmarshalJSON(data []byte) error {
	var owners struct {
		Giantswarm *struct {
			Connectors json.RawMessage `json:"connectors"`
		} `json:"giantswarm"`
		Customer *DexOidcOwner `json:"customer"`
	}
	if err := json.Unmarshal(data, &owners); err != nil {
		return err
	}
	decoded := DexOidc{Customer: owners.Customer}
	if owners.Giantswarm != nil {
		decoded.Giantswarm = &DexOidcOwner{}
		connectors := bytes.TrimSpace(owners.Giantswarm.Connectors)
		if len(connectors) > 0 && connectors[0] == '[' {
			if err := json.Unmarshal(connectors, &decoded.Giantswarm.Connectors); err != nil {
				return err
			}
			decoded.legacyGiantswarmConnectors = true
		} else if len(connectors) > 0 {
			var entries map[string]Connector
			if err := json.Unmarshal(connectors, &entries); err != nil {
				return err
			}
			for id, connector := range entries {
				connector.ID = id
				decoded.Giantswarm.Connectors = append(decoded.Giantswarm.Connectors, connector)
			}
			slices.SortFunc(decoded.Giantswarm.Connectors, func(a, b Connector) int {
				return strings.Compare(a.ID, b.ID)
			})
		}
	}
	*d = decoded
	return nil
}

func (d DexOidc) NeedsMigration() bool {
	return d.legacyGiantswarmConnectors
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
