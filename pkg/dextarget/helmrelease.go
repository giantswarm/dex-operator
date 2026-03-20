package dextarget

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/giantswarm/k8smetadata/pkg/label"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
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

func (h *HelmReleaseTarget) HasUserConfigWithConnectors(ctx context.Context, c client.Client) (bool, error) {
	log := logr.FromContextOrDiscard(ctx)

	for _, vf := range h.Spec.ValuesFrom {
		// Skip our own managed secret
		if strings.HasSuffix(vf.Name, key.DexConfigName) {
			continue
		}

		var values string
		switch vf.Kind {
		case "ConfigMap":
			cm := &corev1.ConfigMap{}
			if err := c.Get(ctx, types.NamespacedName{Name: vf.Name, Namespace: h.Namespace}, cm); err != nil {
				log.Error(err, "Failed to fetch ConfigMap referenced in valuesFrom, skipping connector check",
					"configmap", vf.Name, "namespace", h.Namespace)
				continue
			}
			valuesKey := vf.ValuesKey
			if valuesKey == "" {
				valuesKey = "values.yaml"
			}
			data, ok := cm.Data[valuesKey]
			if !ok {
				continue
			}
			values = data

		case "Secret":
			secret := &corev1.Secret{}
			if err := c.Get(ctx, types.NamespacedName{Name: vf.Name, Namespace: h.Namespace}, secret); err != nil {
				log.Error(err, "Failed to fetch Secret referenced in valuesFrom, skipping connector check",
					"secret", vf.Name, "namespace", h.Namespace)
				continue
			}
			valuesKey := vf.ValuesKey
			if valuesKey == "" {
				valuesKey = "values.yaml"
			}
			values = string(secret.Data[valuesKey])
		}

		if values != "" {
			rex := regexp.MustCompile(fmt.Sprintf(`(%v)(\s*:\s*)(\S+)`, key.ConnectorsKey))
			if matches := rex.FindStringSubmatch(values); len(matches) > 3 {
				return true, nil
			}
		}
	}
	return false, nil
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
// For HelmRelease targets this entry is declared in the Git-managed manifest —
// dex-operator uses this only to detect misconfiguration.
func (h *HelmReleaseTarget) HasSecretConfig(secretName string) bool {
	for _, vf := range h.Spec.ValuesFrom {
		if vf.Kind == "Secret" && vf.Name == secretName {
			return true
		}
	}
	return false
}

// AddSecretConfig is a no-op for HelmRelease targets. The dex config secret
// reference must be declared in the Git-managed HelmRelease manifest upfront.
func (h *HelmReleaseTarget) AddSecretConfig(secretName, secretNamespace string) error {
	return nil
}

// RemoveSecretConfig is a no-op for HelmRelease targets.
func (h *HelmReleaseTarget) RemoveSecretConfig(secretName, secretNamespace string) error {
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

// AttachSecretConfig is a no-op for HelmRelease targets — the dex config
// secret reference is managed in the Git-managed manifest. Returns false
// to indicate no modification was made.
func (h *HelmReleaseTarget) AttachSecretConfig(ctx context.Context, c client.Client) (bool, error) {
	return false, nil
}

// ManagesSecretConfig returns false — for HelmRelease targets the dex config
// secret reference is declared in the Git-managed manifest upfront. dex-operator
// only manages the Secret contents and must not touch spec.valuesFrom.
func (h *HelmReleaseTarget) ManagesSecretConfig() bool {
	return false
}
