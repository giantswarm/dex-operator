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

// DexOidcOwner holds one owner's connectors. It reads connectors as a list or
// as a map keyed by id. It writes a map when Map is set and a list otherwise:
// dex-app releases before 3.5.0 only accept a list, and only a map merges with
// connectors other values sources set under the same owner.
type DexOidcOwner struct {
	Connectors []Connector `json:"connectors,omitempty"`
	// Map is the shape the connectors were read in, and are written in.
	Map bool `json:"-"`
}

// SameShape reports whether a and b store their connectors in the same shape.
// An owner without connectors has no shape.
func SameShape(a, b *DexOidcOwner) bool {
	if a == nil || b == nil || len(a.Connectors) == 0 || len(b.Connectors) == 0 {
		return true
	}
	return a.Map == b.Map
}

func (d DexOidcOwner) MarshalJSON() ([]byte, error) {
	if !d.Map {
		return json.Marshal(struct {
			Connectors []Connector `json:"connectors,omitempty"`
		}{Connectors: d.Connectors})
	}
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
		decoded.Map = true
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
