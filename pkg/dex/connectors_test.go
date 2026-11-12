package dex

import (
	"encoding/json"
	"testing"
)

func TestConnectorShapes(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		input     string
		migration bool
	}{
		{"list", `{"giantswarm":{"connectors":[{"id":"staff","connectorType":"oidc","connectorConfig":"secret: retained"}]} ,"customer":{"connectors":[{"id":"customer"}]}}`, true},
		{"map", `{"giantswarm":{"connectors":{"staff":{"id":"ignored","connectorType":"oidc","connectorConfig":"secret: retained"}}},"customer":{"connectors":[{"id":"customer"}]}}`, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var configuration DexOidc
			if err := json.Unmarshal([]byte(testCase.input), &configuration); err != nil {
				t.Fatal(err)
			}
			if configuration.NeedsMigration() != testCase.migration {
				t.Fatalf("migration = %v, want %v", configuration.NeedsMigration(), testCase.migration)
			}
			if len(configuration.Giantswarm.Connectors) != 1 || configuration.Giantswarm.Connectors[0].ID != "staff" {
				t.Fatal("the connector id must come from the map key or list entry")
			}
			encoded, err := json.Marshal(configuration)
			if err != nil {
				t.Fatal(err)
			}
			var wire struct {
				Giantswarm struct {
					Connectors map[string]Connector `json:"connectors"`
				} `json:"giantswarm"`
				Customer DexOidcOwner `json:"customer"`
			}
			if err := json.Unmarshal(encoded, &wire); err != nil {
				t.Fatal(err)
			}
			if wire.Giantswarm.Connectors["staff"].Config != "secret: retained" {
				t.Fatal("connector configuration was not preserved")
			}
			if len(wire.Customer.Connectors) != 1 || wire.Customer.Connectors[0].ID != "customer" {
				t.Fatal("customer connectors must remain a list")
			}
			var roundTrip DexOidc
			if err := json.Unmarshal(encoded, &roundTrip); err != nil {
				t.Fatal(err)
			}
			if roundTrip.NeedsMigration() {
				t.Fatal("a written map must not require migration")
			}
		})
	}
}

func TestInvalidConnectorShape(t *testing.T) {
	var configuration DexOidc
	if err := json.Unmarshal([]byte(`{"giantswarm":{"connectors":"invalid"}}`), &configuration); err == nil {
		t.Fatal("invalid connector shapes must be refused")
	}
}
