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
			name  string
			input string
			isMap bool
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
				stored := ownerOf(configuration, owner)
				if stored.Map != testCase.isMap {
					t.Fatalf("Map = %v, want %v", stored.Map, testCase.isMap)
				}
				want := []Connector{{ID: "federation", Type: "oidc", Config: "secret: retained"}}
				if !reflect.DeepEqual(stored.Connectors, want) {
					t.Fatalf("connectors = %+v, want %+v", stored.Connectors, want)
				}

				encoded, err := json.Marshal(configuration)
				if err != nil {
					t.Fatal(err)
				}
				var roundTrip DexOidc
				if err := json.Unmarshal(encoded, &roundTrip); err != nil {
					t.Fatal(err)
				}
				written := ownerOf(roundTrip, owner)
				if written.Map != testCase.isMap || !reflect.DeepEqual(written.Connectors, want) {
					t.Fatalf("round trip = %+v, want the same shape and connectors", written)
				}
			})
		}
	}
}

func TestConnectorsAreWrittenInTheRequestedShape(t *testing.T) {
	connectors := []Connector{{ID: "b", Type: "oidc"}, {ID: "a", Type: "github"}}
	for _, testCase := range []struct {
		name  string
		isMap bool
		want  string
	}{
		{"list", false, `{"connectors":[{"connectorType":"oidc","connectorName":"","id":"b","connectorConfig":""},{"connectorType":"github","connectorName":"","id":"a","connectorConfig":""}]}`},
		{"map", true, `{"connectors":{"a":{"connectorType":"github","connectorName":"","id":"a","connectorConfig":""},"b":{"connectorType":"oidc","connectorName":"","id":"b","connectorConfig":""}}}`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			encoded, err := json.Marshal(DexOidcOwner{Connectors: connectors, Map: testCase.isMap})
			if err != nil {
				t.Fatal(err)
			}
			if string(encoded) != testCase.want {
				t.Fatalf("encoded = %s, want %s", encoded, testCase.want)
			}
		})
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
	var configuration DexOidc
	if err := json.Unmarshal([]byte(`{"giantswarm":{"connectors":[{"id":"staff"}]},"customer":{"connectors":{"federation":{}}}}`), &configuration); err != nil {
		t.Fatal(err)
	}
	if configuration.Giantswarm.Map || !configuration.Customer.Map {
		t.Fatal("each owner keeps its own shape")
	}
	if len(configuration.Giantswarm.Connectors) != 1 || len(configuration.Customer.Connectors) != 1 {
		t.Fatal("no owner's connectors may be lost")
	}
}

func TestEmptyConnectorShapes(t *testing.T) {
	for _, input := range []string{`{"giantswarm":{}}`, `{"giantswarm":{"connectors":null}}`, `{"giantswarm":{"connectors":[]}}`} {
		var configuration DexOidc
		if err := json.Unmarshal([]byte(input), &configuration); err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if configuration.Giantswarm.Map || len(configuration.Giantswarm.Connectors) != 0 {
			t.Fatalf("%s: want no connectors", input)
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

func TestSameShape(t *testing.T) {
	list := &DexOidcOwner{Connectors: []Connector{{ID: "a"}}}
	asMap := &DexOidcOwner{Connectors: []Connector{{ID: "a"}}, Map: true}
	empty := &DexOidcOwner{Map: true}
	for _, testCase := range []struct {
		name string
		a, b *DexOidcOwner
		want bool
	}{
		{"list and list", list, list, true},
		{"map and map", asMap, asMap, true},
		{"list and map", list, asMap, false},
		{"nil owner", nil, asMap, true},
		{"owner without connectors", empty, list, true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := SameShape(testCase.a, testCase.b); got != testCase.want {
				t.Fatalf("SameShape = %v, want %v", got, testCase.want)
			}
		})
	}
}

func ownerOf(configuration DexOidc, owner string) *DexOidcOwner {
	if owner == "giantswarm" {
		return configuration.Giantswarm
	}
	return configuration.Customer
}
