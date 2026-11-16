package idp

import (
	"encoding/json"
	"fmt"
	"testing"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/key"
)

func TestConnectorMapIsRewrittenAsListWithoutCredentialChange(t *testing.T) {
	for _, owner := range []string{key.OwnerGiantswarm, key.OwnerCustomer} {
		t.Run(owner, func(t *testing.T) {
			var stored dex.DexConfig
			input := fmt.Sprintf(`{"oidc":{%q:{"connectors":{"federation":{"connectorType":"oidc","connectorConfig":"clientSecret: retained"}}}}}`, owner)
			if err := json.Unmarshal([]byte(input), &stored); err != nil {
				t.Fatal(err)
			}
			service := Service{log: ctrl.Log.WithName("test")}
			encoded, err := json.Marshal(stored)
			if err != nil {
				t.Fatal(err)
			}
			var written dex.DexConfig
			if err := json.Unmarshal(encoded, &written); err != nil {
				t.Fatal(err)
			}
			if !service.secretDataNeedsUpdate(stored, written) {
				t.Fatal("a stored map must be rewritten as a list")
			}
			if service.secretDataNeedsUpdate(written, written) {
				t.Fatal("an unchanged list must not trigger another update")
			}
		})
	}
}
