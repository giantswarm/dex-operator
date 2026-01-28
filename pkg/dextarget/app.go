package dextarget

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/giantswarm/k8smetadata/pkg/label"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/dex-operator/pkg/key"
)

// AppTarget wraps a Giant Swarm App CR to implement the DexTarget interface
type AppTarget struct {
	*v1alpha1.App
}

// NewAppTarget creates a new AppTarget wrapper
func NewAppTarget(app *v1alpha1.App) *AppTarget {
	return &AppTarget{
		App: app,
	}
}

func (a *AppTarget) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      a.Name,
		Namespace: a.Namespace,
	}
}

func (a *AppTarget) GetClusterLabel() string {
	return a.GetLabels()[label.Cluster]
}

func (a *AppTarget) GetOrganizationLabel() string {
	return a.GetLabels()[label.Organization]
}

func (a *AppTarget) HasUserConfigWithConnectors(ctx context.Context, c client.Client) (bool, error) {
	// Check if user configmap is present
	if a.Spec.UserConfig.ConfigMap.Name == "" && a.Spec.UserConfig.ConfigMap.Namespace == "" {
		return false, nil
	}

	userConfigMap := &corev1.ConfigMap{}
	if err := c.Get(ctx, types.NamespacedName{
		Name:      a.Spec.UserConfig.ConfigMap.Name,
		Namespace: a.Spec.UserConfig.ConfigMap.Namespace},
		userConfigMap); err != nil {
		return false, err
	}

	// Check if connectors are defined in user configmap
	values, ok := userConfigMap.Data[key.ValuesConfigMapKey]
	if !ok {
		return false, nil
	}
	rex := regexp.MustCompile(fmt.Sprintf(`(%v)(\s*:\s*)(\S+)`, key.ConnectorsKey))
	if matches := rex.FindStringSubmatch(values); len(matches) > 3 {
		return true, nil
	}
	return false, nil
}

func (a *AppTarget) HasClusterValuesConfig() bool {
	return strings.HasSuffix(a.Spec.Config.ConfigMap.Name, key.ClusterValuesConfigmapSuffix)
}

func (a *AppTarget) GetClusterValuesConfigMapRef() (name, namespace string) {
	return a.Spec.Config.ConfigMap.Name, a.Spec.Config.ConfigMap.Namespace
}

func (a *AppTarget) HasSecretConfig(secretName string) bool {
	if a.Spec.ExtraConfigs == nil {
		return false
	}
	for _, config := range a.Spec.ExtraConfigs {
		if config.Kind == "secret" && config.Name == secretName && config.Namespace == a.Namespace {
			return true
		}
	}
	return false
}

func (a *AppTarget) AddSecretConfig(secretName, secretNamespace string) error {
	config := v1alpha1.AppExtraConfig{
		Kind:      "secret",
		Name:      secretName,
		Namespace: secretNamespace,
		Priority:  key.DexSecretConfigPriority,
	}
	a.Spec.ExtraConfigs = append(a.Spec.ExtraConfigs, config)
	return nil
}

func (a *AppTarget) RemoveSecretConfig(secretName, secretNamespace string) error {
	if a.Spec.ExtraConfigs == nil {
		return nil
	}
	result := []v1alpha1.AppExtraConfig{}
	for _, config := range a.Spec.ExtraConfigs {
		if !(config.Kind == "secret" && config.Name == secretName && config.Namespace == secretNamespace) {
			result = append(result, config)
		}
	}
	a.Spec.ExtraConfigs = result
	return nil
}

func (a *AppTarget) IsBeingDeleted() bool {
	return !a.DeletionTimestamp.IsZero()
}

func (a *AppTarget) GetTargetType() string {
	return "App"
}

func (a *AppTarget) GetObject() client.Object {
	return a.App
}
