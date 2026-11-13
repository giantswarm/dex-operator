package dex

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestConnectorShapes(t *testing.T) {
	for _, owner := range []string{"giantswarm", "customer"} {
		for _, testCase := range []struct {
			name      string
			input     string
			migration bool
		}{
			{"list", `[{"id":"federation","connectorType":"oidc","connectorConfig":"secret: retained"}]`, true},
			{"map", `{"federation":{"id":"ignored","connectorType":"oidc","connectorConfig":"secret: retained"}}`, false},
		} {
			t.Run(owner+"/"+testCase.name, func(t *testing.T) {
				var configuration DexOidc
				input := fmt.Sprintf(`{%q:{"connectors":%s}}`, owner, testCase.input)
				if err := json.Unmarshal([]byte(input), &configuration); err != nil {
					t.Fatal(err)
				}
				if configuration.NeedsMigration() != testCase.migration {
					t.Fatalf("migration = %v, want %v", configuration.NeedsMigration(), testCase.migration)
				}
				encoded, err := json.Marshal(configuration)
				if err != nil {
					t.Fatal(err)
				}
				var wire map[string]struct {
					Connectors map[string]Connector `json:"connectors"`
				}
				if err := json.Unmarshal(encoded, &wire); err != nil {
					t.Fatal(err)
				}
				connector := wire[owner].Connectors["federation"]
				if connector.ID != "federation" || connector.Config != "secret: retained" {
					t.Fatal("map ids and connector configuration must be preserved")
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
}

func TestMixedOwnerShapes(t *testing.T) {
	for _, input := range []string{
		`{"giantswarm":{"connectors":[{"id":"staff"}]},"customer":{"connectors":{"federation":{}}}}`,
		`{"giantswarm":{"connectors":{"staff":{}}},"customer":{"connectors":[{"id":"federation"}]}}`,
	} {
		var configuration DexOidc
		if err := json.Unmarshal([]byte(input), &configuration); err != nil {
			t.Fatal(err)
		}
		if !configuration.NeedsMigration() || len(configuration.Giantswarm.Connectors) != 1 || len(configuration.Customer.Connectors) != 1 {
			t.Fatal("a list under either owner must migrate without losing the other owner's map")
		}
	}
}

func TestInvalidConnectorShape(t *testing.T) {
	for _, owner := range []string{"giantswarm", "customer"} {
		var configuration DexOidc
		if err := json.Unmarshal([]byte(fmt.Sprintf(`{%q:{"connectors":"invalid"}}`, owner)), &configuration); err == nil {
			t.Fatal("invalid connector shapes must be refused")
		}
	}
}
