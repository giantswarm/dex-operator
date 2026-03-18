package idp

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/giantswarm/microerror"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/key"
)

func getBaseDomainFromClusterValues(clusterValuesConfigmap *corev1.ConfigMap) string {
	values := clusterValuesConfigmap.Data[key.ValuesConfigMapKey]
	rex := regexp.MustCompile(fmt.Sprintf(`(%v)(\s*:\s*)(\S+)`, key.BaseDomainKey))
	if matches := rex.FindStringSubmatch(values); len(matches) > 3 {
		return matches[3]
	}
	return ""
}

func getConnectorsFromSecret(secret *corev1.Secret) (map[string]dex.Connector, error) {
	connectors := map[string]dex.Connector{}
	configData, exists := secret.Data["default"]
	if !exists {
		return connectors, nil
	}
	config := dex.DexConfig{}
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, microerror.Mask(err)
	}
	return getConnectorsFromConfig(config), nil
}

func getDexConfigFromSecret(secret *corev1.Secret) (dex.DexConfig, error) {
	config := dex.DexConfig{}
	configData, exists := secret.Data["default"]
	if !exists {
		return config, nil
	}
	if err := json.Unmarshal(configData, &config); err != nil {
		return config, microerror.Mask(err)
	}
	return config, nil
}

func getConnectorsFromConfig(config dex.DexConfig) map[string]dex.Connector {
	connectors := map[string]dex.Connector{}
	if config.Oidc.Customer != nil {
		for _, connector := range config.Oidc.Customer.Connectors {
			connectors[connector.ID] = connector
		}
	}
	if config.Oidc.Giantswarm != nil {
		for _, connector := range config.Oidc.Giantswarm.Connectors {
			connectors[connector.ID] = connector
		}
	}
	return connectors
}

// GetDexSecretConfig returns the AppExtraConfig for the dex secret
// This is used by both the App CR and for test compatibility
func GetDexSecretConfig(n types.NamespacedName) v1alpha1.AppExtraConfig {
	return v1alpha1.AppExtraConfig{
		Kind:      "secret",
		Name:      key.GetDexConfigName(n.Name),
		Namespace: n.Namespace,
		Priority:  key.DexSecretConfigPriority,
	}
}

// GetVintageDexSecretConfig returns the AppExtraConfig for vintage dex secret format
func GetVintageDexSecretConfig(namespace string) v1alpha1.AppExtraConfig {
	return v1alpha1.AppExtraConfig{
		Kind:      "secret",
		Name:      key.DexConfigName,
		Namespace: namespace,
		Priority:  key.DexSecretConfigPriority,
	}
}

// removeExtraConfig removes the specified config from the extra configs list
func removeExtraConfig(extraConfigs []v1alpha1.AppExtraConfig, dexSecretConfig v1alpha1.AppExtraConfig) []v1alpha1.AppExtraConfig {
	if extraConfigs == nil {
		return extraConfigs
	}
	result := []v1alpha1.AppExtraConfig{}
	for _, config := range extraConfigs {
		if config.Kind != dexSecretConfig.Kind || config.Name != dexSecretConfig.Name || config.Namespace != dexSecretConfig.Namespace {
			result = append(result, config)
		}
	}
	return result
}
