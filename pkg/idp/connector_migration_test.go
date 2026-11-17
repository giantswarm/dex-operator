package idp

import (
	"encoding/json"
	"fmt"
	"testing"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/key"
)

func TestConnectorShapeChangeRewritesWithoutCredentialChange(t *testing.T) {
	stored := map[bool]string{
		false: `[{"id":"federation","connectorType":"oidc","connectorConfig":"clientSecret: retained"}]`,
		true:  `{"federation":{"connectorType":"oidc","connectorConfig":"clientSecret: retained"}}`,
	}
	for _, owner := range []string{key.OwnerGiantswarm, key.OwnerCustomer} {
		for _, writesMaps := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/writesMaps=%v", owner, writesMaps), func(t *testing.T) {
				service := Service{log: ctrl.Log.WithName("test")}
				var old dex.DexConfig
				if err := json.Unmarshal([]byte(fmt.Sprintf(`{"oidc":{%q:{"connectors":%s}}}`, owner, stored[!writesMaps])), &old); err != nil {
					t.Fatal(err)
				}

				desired := old
				desiredOwner := *ownerOf(old, owner)
				desiredOwner.Map = writesMaps
				setOwner(&desired, owner, &desiredOwner)

				if !service.secretDataNeedsUpdate(old, desired) {
					t.Fatal("a Secret in the other shape must be rewritten")
				}
				encoded, err := json.Marshal(desired)
				if err != nil {
					t.Fatal(err)
				}
				var written dex.DexConfig
				if err := json.Unmarshal(encoded, &written); err != nil {
					t.Fatal(err)
				}
				if got := ownerOf(written, owner); got.Map != writesMaps || got.Connectors[0] != ownerOf(old, owner).Connectors[0] {
					t.Fatalf("written = %+v, want the same connector in the requested shape", got)
				}
				if service.secretDataNeedsUpdate(written, desired) {
					t.Fatal("a Secret already in the requested shape must not be rewritten again")
				}
			})
		}
	}
}

func ownerOf(config dex.DexConfig, owner string) *dex.DexOidcOwner {
	if owner == key.OwnerGiantswarm {
		return config.Oidc.Giantswarm
	}
	return config.Oidc.Customer
}

func setOwner(config *dex.DexConfig, owner string, value *dex.DexOidcOwner) {
	if owner == key.OwnerGiantswarm {
		config.Oidc.Giantswarm = value
		return
	}
	config.Oidc.Customer = value
}
