package dextarget

import (
	"context"
	"encoding/json"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/giantswarm/dex-operator/pkg/key"
)

// DexTarget is an interface that abstracts the common functionality between
// Giant Swarm App CRs and Flux HelmReleases for dex-operator configuration injection.
type DexTarget interface {
	client.Object

	// GetNamespacedName returns the namespaced name of the target
	GetNamespacedName() types.NamespacedName

	// GetClusterLabel returns the cluster label value if present
	GetClusterLabel() string

	// GetOrganizationLabel returns the organization label value if present
	GetOrganizationLabel() string

	// UserConnectorOwners returns the owners that have connectors set under
	// oidc.<owner>.connectors in the user config of the target
	UserConnectorOwners(ctx context.Context, client client.Client) ([]string, error)

	// HasClusterValuesConfig returns true if the target has a cluster values configmap reference
	HasClusterValuesConfig() bool

	// GetClusterValuesConfigMapRef returns the name and namespace of the cluster values configmap
	GetClusterValuesConfigMapRef() (name, namespace string)

	// HasSecretConfig returns true if the dex secret config is already present
	HasSecretConfig(secretName string) bool

	// AddSecretConfig adds the dex secret config reference to the target
	// For App CR: adds to .spec.extraConfigs with priority
	// For HelmRelease: adds to .spec.valuesFrom
	AddSecretConfig(secretName, secretNamespace string) error

	// RemoveSecretConfig removes the dex secret config reference from the target
	RemoveSecretConfig(secretName, secretNamespace string) error

	// IsBeingDeleted returns true if the target is being deleted
	IsBeingDeleted() bool

	// GetTargetType returns the type of the target ("App" or "HelmRelease")
	GetTargetType() string

	// GetObject returns the underlying Kubernetes object for use with client.Update
	// This is needed because the wrapper types don't have GVK registered in the scheme
	GetObject() client.Object

	// AttachSecretConfig persists the secret config reference added by AddSecretConfig
	// to the target. For App CRs and HelmReleases not managed by Flux, this performs a client Update;
	// for HelmRelease targets managed by Flux, this is a no-op.
	// Returns true if the target was actually modified.
	AttachSecretConfig(ctx context.Context, c client.Client) (bool, error)

	// ManagesSecretConfig returns true if dex-operator should inject and manage
	// the dex config secret reference directly on this target.
	// For App CR targets this is always true.
	// For HelmRelease targets it is true only if the HelmRelease is self-managed
	// (no Flux Kustomization labels) — Flux-managed HelmReleases must declare the
	// entry in their Git manifest to avoid SSA ownership conflicts.
	ManagesSecretConfig() bool
}

// connectorOwners returns the owners that set oidc.<owner>.connectors in the
// given Helm values. A key set to [] or null counts too, because Helm still
// replaces the list dex-operator writes.
func connectorOwners(values string) ([]string, error) {
	type ownerValues struct {
		Connectors json.RawMessage `json:"connectors"`
	}
	var v struct {
		Oidc struct {
			Giantswarm ownerValues `json:"giantswarm"`
			Customer   ownerValues `json:"customer"`
		} `json:"oidc"`
	}
	if err := yaml.Unmarshal([]byte(values), &v); err != nil {
		return nil, err
	}
	var owners []string
	if v.Oidc.Giantswarm.Connectors != nil {
		owners = append(owners, key.OwnerGiantswarm)
	}
	if v.Oidc.Customer.Connectors != nil {
		owners = append(owners, key.OwnerCustomer)
	}
	return owners, nil
}
