package dextarget

import (
	"context"
	"slices"

	"github.com/Masterminds/semver/v3"
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

	// UserConnectors returns what each of the target's other values sources
	// sets under oidc.<owner>.connectors.
	UserConnectors(ctx context.Context, client client.Client) ([]UserConnectors, error)

	// AcceptsConnectorMaps reports whether the dex-app chart the target runs,
	// and any version it is moving to, accepts connectors as a map.
	AcceptsConnectorMaps() bool

	// HasClusterValuesConfig returns true if the target has a cluster values configmap reference
	HasClusterValuesConfig() bool

	// GetClusterValuesConfigMapRef returns the name and namespace of the cluster values configmap
	GetClusterValuesConfigMapRef() (name, namespace string)

	// HasSecretConfig returns true if the dex secret config is already present
	HasSecretConfig(secretName string) bool

	// AddSecretConfig adds the dex secret config reference to the target
	// For App CR: adds to .spec.extraConfigs with priority
	// For HelmRelease: does nothing
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
	// to the target. For App CRs this performs a client Update; for HelmReleases it does nothing.
	// Returns true if the target was actually modified.
	AttachSecretConfig(ctx context.Context, c client.Client) (bool, error)

	// ManagesSecretConfig returns true if dex-operator should inject and manage
	// the dex config secret reference directly on this target.
	// True for App CRs. False for HelmReleases: Flux owns spec.valuesFrom, so
	// the manifest has to reference the secret.
	ManagesSecretConfig() bool
}

// UserConnectors is what one values source sets under oidc.<Owner>.connectors.
// Helm merges a map with the map the operator writes, entry by entry; any
// other value (a list, null, or a value replacing the whole connectors key)
// replaces the operator's connectors or is replaced by them.
type UserConnectors struct {
	Owner string
	// Map is true when the source sets connectors by id: a map, or a value at
	// or below oidc.<Owner>.connectors.<id>.
	Map bool
	// IDs are the connector ids the source sets when Map is true.
	IDs []string
}

// userConnectors returns what the given Helm values set under
// oidc.<owner>.connectors, per owner.
func userConnectors(values string) ([]UserConnectors, error) {
	// Decode into maps, not a struct: a value of an unexpected type elsewhere
	// under oidc must not fail the parse and hide a connectors key.
	var v map[string]interface{}
	if err := yaml.Unmarshal([]byte(values), &v); err != nil {
		return nil, err
	}
	oidc, _ := v["oidc"].(map[string]interface{})
	var found []UserConnectors
	for _, owner := range []string{key.OwnerGiantswarm, key.OwnerCustomer} {
		ownerValues, _ := oidc[owner].(map[string]interface{})
		connectors, ok := ownerValues["connectors"]
		if !ok {
			continue
		}
		entries, isMap := connectors.(map[string]interface{})
		uc := UserConnectors{Owner: owner, Map: isMap}
		for id := range entries {
			uc.IDs = append(uc.IDs, id)
		}
		slices.Sort(uc.IDs)
		found = append(found, uc)
	}
	return found, nil
}

// chartAcceptsConnectorMaps reports whether every given dex-app chart version
// accepts connector maps. An empty or unparsable version does not.
func chartAcceptsConnectorMaps(versions ...string) bool {
	minimum := semver.MustParse(key.ConnectorMapMinChartVersion)
	for _, v := range versions {
		parsed, err := semver.NewVersion(v)
		if err != nil || parsed.LessThan(minimum) {
			return false
		}
	}
	return len(versions) > 0
}
