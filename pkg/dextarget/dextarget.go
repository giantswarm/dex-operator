package dextarget

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FieldManager is the field manager name used for server-side apply patches.
const FieldManager = "dex-operator"

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

	// PatchTarget persists a change to the target's config reference (e.g. after
	// AddSecretConfig or RemoveSecretConfig). AppTarget uses a plain Update;
	// HelmReleaseTarget uses server-side apply so that Flux does not overwrite
	// the valuesFrom entry dex-operator adds.
	PatchTarget(ctx context.Context, c client.Client) error

	// ManagesSecretConfig returns true if dex-operator should inject and manage
	// the dex config secret reference directly on this target (App CR).
	// For HelmRelease targets the entry is declared in the Git-managed manifest
	// upfront, so dex-operator must not touch spec.valuesFrom.
	ManagesSecretConfig() bool
}
