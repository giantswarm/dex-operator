package idp

import (
	"encoding/json"
	"fmt"
	"testing"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/key"
)

func TestConnectorListMigratesWithoutCredentialChange(t *testing.T) {
	for _, owner := range []string{key.OwnerGiantswarm, key.OwnerCustomer} {
		t.Run(owner, func(t *testing.T) {
			var oldConfiguration dex.DexConfig
			input := fmt.Sprintf(`{"oidc":{%q:{"connectors":[{"id":"federation","connectorType":"oidc","connectorConfig":"clientSecret: retained"}]}}}`, owner)
			if err := json.Unmarshal([]byte(input), &oldConfiguration); err != nil {
				t.Fatal(err)
			}
			service := Service{log: ctrl.Log.WithName("test")}
			encoded, err := json.Marshal(oldConfiguration)
			if err != nil {
				t.Fatal(err)
			}
			var newConfiguration dex.DexConfig
			if err := json.Unmarshal(encoded, &newConfiguration); err != nil {
				t.Fatal(err)
			}
			if !service.secretDataNeedsUpdate(oldConfiguration, newConfiguration) {
				t.Fatal("unchanged credentials must still migrate from list to map")
			}
			if service.secretDataNeedsUpdate(newConfiguration, newConfiguration) {
				t.Fatal("an unchanged map must not trigger another update")
			}
		})
	}
}
