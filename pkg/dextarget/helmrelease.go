package dextarget

import (
	"context"
	"fmt"
	"slices"
	"strings"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/giantswarm/k8smetadata/pkg/label"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/dex-operator/pkg/key"
)

// HelmReleaseTarget wraps a Flux HelmRelease to implement the DexTarget interface
type HelmReleaseTarget struct {
	*helmv2.HelmRelease
}

// NewHelmReleaseTarget creates a new HelmReleaseTarget wrapper
func NewHelmReleaseTarget(hr *helmv2.HelmRelease) *HelmReleaseTarget {
	return &HelmReleaseTarget{
		HelmRelease: hr,
	}
}

func (h *HelmReleaseTarget) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      h.Name,
		Namespace: h.Namespace,
	}
}

func (h *HelmReleaseTarget) GetClusterLabel() string {
	return h.GetLabels()[label.Cluster]
}

func (h *HelmReleaseTarget) GetOrganizationLabel() string {
	return h.GetLabels()[label.Organization]
}

func (h *HelmReleaseTarget) UserConnectorOwners(ctx context.Context, c client.Client) ([]string, error) {
	log := logr.FromContextOrDiscard(ctx)

	var owners []string
	add := func(found ...string) {
		for _, owner := range found {
			if !slices.Contains(owners, owner) {
				owners = append(owners, owner)
			}
		}
	}

	for _, vf := range h.Spec.ValuesFrom {
		// Skip our own managed secret
		if vf.Name == key.GetDexConfigName(h.Name) {
			continue
		}
		// A targetPath entry sets one value at that path, so only the path matters
		if vf.TargetPath != "" {
			if owner := targetPathOwner(vf.TargetPath); owner != "" {
				add(owner)
			}
			continue
		}

		valuesKey := vf.ValuesKey
		if valuesKey == "" {
			valuesKey = "values.yaml"
		}
		var values string
		switch vf.Kind {
		case "ConfigMap":
			cm := &corev1.ConfigMap{}
			if err := c.Get(ctx, types.NamespacedName{Name: vf.Name, Namespace: h.Namespace}, cm); err != nil {
				if !apierrors.IsNotFound(err) {
					return nil, microerror.Mask(err)
				}
				log.Error(err, "Failed to fetch ConfigMap referenced in valuesFrom, skipping connector check",
					"configmap", vf.Name, "namespace", h.Namespace)
				continue
			}
			data, ok := cm.Data[valuesKey]
			if !ok {
				continue
			}
			values = data

		case "Secret":
			secret := &corev1.Secret{}
			if err := c.Get(ctx, types.NamespacedName{Name: vf.Name, Namespace: h.Namespace}, secret); err != nil {
				if !apierrors.IsNotFound(err) {
					return nil, microerror.Mask(err)
				}
				log.Error(err, "Failed to fetch Secret referenced in valuesFrom, skipping connector check",
					"secret", vf.Name, "namespace", h.Namespace)
				continue
			}
			values = string(secret.Data[valuesKey])
		}

		if values != "" {
			found, err := connectorOwners(values)
			if err != nil {
				// Do not log the error, it can quote Secret content.
				log.Info("Failed to parse values from valuesFrom, skipping connector check",
					"kind", vf.Kind, "name", vf.Name, "namespace", h.Namespace, "valuesKey", valuesKey)
				continue
			}
			add(found...)
		}
	}
	return owners, nil
}

// targetPathOwner returns the owner whose oidc.<owner>.connectors list the
// given valuesFrom targetPath sets, or "" if there is none.
func targetPathOwner(targetPath string) string {
	for _, owner := range []string{key.OwnerGiantswarm, key.OwnerCustomer} {
		path := "oidc." + owner + ".connectors"
		if targetPath == path || strings.HasPrefix(targetPath, path+"[") || strings.HasPrefix(targetPath, path+".") {
			return owner
		}
	}
	return ""
}

func (h *HelmReleaseTarget) HasClusterValuesConfig() bool {
	for _, vf := range h.Spec.ValuesFrom {
		if vf.Kind == "ConfigMap" && strings.HasSuffix(vf.Name, key.ClusterValuesConfigmapSuffix) {
			return true
		}
	}
	return false
}

func (h *HelmReleaseTarget) GetClusterValuesConfigMapRef() (name, namespace string) {
	for _, vf := range h.Spec.ValuesFrom {
		if vf.Kind == "ConfigMap" && strings.HasSuffix(vf.Name, key.ClusterValuesConfigmapSuffix) {
			return vf.Name, h.Namespace
		}
	}
	return "", ""
}

// HasSecretConfig returns true if the dex config secret is referenced in valuesFrom.
func (h *HelmReleaseTarget) HasSecretConfig(secretName string) bool {
	for _, vf := range h.Spec.ValuesFrom {
		if vf.Kind == "Secret" && vf.Name == secretName {
			return true
		}
	}
	return false
}

func (h *HelmReleaseTarget) AddSecretConfig(secretName, secretNamespace string) error {
	if secretNamespace != h.Namespace {
		return fmt.Errorf("HelmRelease valuesFrom does not support cross-namespace references: secret %s/%s cannot be referenced from HelmRelease in namespace %s",
			secretNamespace, secretName, h.Namespace)
	}
	vf := helmv2.ValuesReference{
		Kind:      "Secret",
		Name:      secretName,
		ValuesKey: "default",
	}
	h.Spec.ValuesFrom = append(h.Spec.ValuesFrom, vf)
	return nil
}

// RemoveSecretConfig is a no-op for Flux-managed HelmRelease targets.
// For self-managed HelmReleases it removes the entry from valuesFrom in memory.
func (h *HelmReleaseTarget) RemoveSecretConfig(secretName, secretNamespace string) error {
	if h.Spec.ValuesFrom == nil {
		return nil
	}
	result := []helmv2.ValuesReference{}
	for _, vf := range h.Spec.ValuesFrom {
		if !(vf.Kind == "Secret" && vf.Name == secretName) {
			result = append(result, vf)
		}
	}
	h.Spec.ValuesFrom = result
	return nil
}

func (h *HelmReleaseTarget) IsBeingDeleted() bool {
	return !h.DeletionTimestamp.IsZero()
}

func (h *HelmReleaseTarget) GetTargetType() string {
	return "HelmRelease"
}

func (h *HelmReleaseTarget) GetObject() client.Object {
	return h.HelmRelease
}

// AttachSecretConfig persists valuesFrom changes for self-managed HelmReleases
// via a plain Update. For Flux-managed HelmReleases this is a no-op — the entry
// must be declared in the Git-managed manifest to avoid Flux ownership conflicts.
// Returns true if the target was actually modified.
func (h *HelmReleaseTarget) AttachSecretConfig(ctx context.Context, c client.Client) (bool, error) {
	if h.isFluxManaged() {
		return false, nil
	}
	if err := c.Update(ctx, h.HelmRelease); err != nil {
		return false, err
	}
	return true, nil
}

// ManagesSecretConfig returns true for self-managed HelmReleases (dex-operator
// can safely inject the valuesFrom entry) and false for Flux-managed ones
// (entry must be declared in the Git manifest to avoid SSA ownership conflicts).
func (h *HelmReleaseTarget) ManagesSecretConfig() bool {
	return !h.isFluxManaged()
}

// isFluxManaged returns true if this HelmRelease is reconciled by a Flux
// Kustomization, identified by the presence of Flux management labels.
func (h *HelmReleaseTarget) isFluxManaged() bool {
	labels := h.GetLabels()
	_, hasName := labels["kustomize.toolkit.fluxcd.io/name"]
	_, hasNamespace := labels["kustomize.toolkit.fluxcd.io/namespace"]
	return hasName && hasNamespace
}
