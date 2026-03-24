package dextarget

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	// HasUserConfigWithConnectors returns true if the target has user-defined connector config
	// that should prevent dex-operator from managing connectors
	HasUserConfigWithConnectors(ctx context.Context, client client.Client) (bool, error)

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
	// to the target. For App CR targets this performs a client Update; for HelmRelease
	// targets this is a no-op because the reference is managed in the Git manifest.
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
