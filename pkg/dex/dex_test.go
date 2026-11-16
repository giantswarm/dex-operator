package dex

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestConnectorShapes(t *testing.T) {
	for _, owner := range []string{"giantswarm", "customer"} {
		for _, testCase := range []struct {
			name      string
			input     string
			storedMap bool
		}{
			{"list", `[{"id":"federation","connectorType":"oidc","connectorConfig":"secret: retained"}]`, false},
			{"map", `{"federation":{"id":"ignored","connectorType":"oidc","connectorConfig":"secret: retained"}}`, true},
		} {
			t.Run(owner+"/"+testCase.name, func(t *testing.T) {
				var configuration DexOidc
				input := fmt.Sprintf(`{%q:{"connectors":%s}}`, owner, testCase.input)
				if err := json.Unmarshal([]byte(input), &configuration); err != nil {
					t.Fatal(err)
				}
				if configuration.StoredAsMap() != testCase.storedMap {
					t.Fatalf("StoredAsMap() = %v, want %v", configuration.StoredAsMap(), testCase.storedMap)
				}
				encoded, err := json.Marshal(configuration)
				if err != nil {
					t.Fatal(err)
				}
				var wire map[string]struct {
					Connectors []Connector `json:"connectors"`
				}
				if err := json.Unmarshal(encoded, &wire); err != nil {
					t.Fatalf("connectors must be written as a list: %v", err)
				}
				want := []Connector{{ID: "federation", Type: "oidc", Config: "secret: retained"}}
				if !reflect.DeepEqual(wire[owner].Connectors, want) {
					t.Fatalf("written connectors = %+v, want %+v", wire[owner].Connectors, want)
				}
				var roundTrip DexOidc
				if err := json.Unmarshal(encoded, &roundTrip); err != nil {
					t.Fatal(err)
				}
				if roundTrip.StoredAsMap() {
					t.Fatal("a written list must not be rewritten again")
				}
			})
		}
	}
}

func TestMapConnectorsAreSortedByID(t *testing.T) {
	var configuration DexOidc
	if err := json.Unmarshal([]byte(`{"giantswarm":{"connectors":{"b":{},"a":{}}}}`), &configuration); err != nil {
		t.Fatal(err)
	}
	want := []Connector{{ID: "a"}, {ID: "b"}}
	if !reflect.DeepEqual(configuration.Giantswarm.Connectors, want) {
		t.Fatalf("connectors = %+v, want %+v", configuration.Giantswarm.Connectors, want)
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
		if !configuration.StoredAsMap() || len(configuration.Giantswarm.Connectors) != 1 || len(configuration.Customer.Connectors) != 1 {
			t.Fatal("a map under either owner must be rewritten without losing the other owner's list")
		}
	}
}

func TestEmptyConnectorShapes(t *testing.T) {
	for _, input := range []string{`{"giantswarm":{}}`, `{"giantswarm":{"connectors":null}}`, `{"giantswarm":{"connectors":[]}}`} {
		var configuration DexOidc
		if err := json.Unmarshal([]byte(input), &configuration); err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if configuration.StoredAsMap() || len(configuration.Giantswarm.Connectors) != 0 {
			t.Fatalf("%s: want no connectors and no rewrite", input)
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
