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

// StoredAsMap reports whether either owner's connectors were read from the
// map form. The operator writes lists, so such a Secret is rewritten.
func (d DexOidc) StoredAsMap() bool {
	return (d.Giantswarm != nil && d.Giantswarm.mapConnectors) ||
		(d.Customer != nil && d.Customer.mapConnectors)
}

// DexOidcOwner holds one owner's connectors. It reads connectors as a list or
// as a map keyed by id, and writes a list: dex-app releases before 3.5.0 only
// accept a list.
type DexOidcOwner struct {
	Connectors    []Connector `json:"connectors,omitempty"`
	mapConnectors bool
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
	} else if len(connectors) > 0 && !bytes.Equal(connectors, []byte("null")) {
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
		decoded.mapConnectors = true
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
