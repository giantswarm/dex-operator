package idp

import (
	"encoding/json"
	"fmt"
	"giantswarm/dex-operator/pkg/dex"
	"giantswarm/dex-operator/pkg/key"
	"reflect"
	"regexp"
	"strings"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/giantswarm/microerror"
	corev1 "k8s.io/api/core/v1"
)

func dexSecretConfigIsPresent(app *v1alpha1.App, dexSecretConfig v1alpha1.AppExtraConfig) bool {
	if app.Spec.ExtraConfigs == nil {
		return false
	}
	for _, config := range app.Spec.ExtraConfigs {
		if reflect.DeepEqual(config, dexSecretConfig) {
			return true
		}
	}
	return false
}

func clusterValuesIsPresent(app *v1alpha1.App) bool {
	return strings.HasSuffix(app.Spec.Config.ConfigMap.Name, key.ClusterValuesConfigmapSuffix)
}

func getBaseDomainFromClusterValues(clusterValuesConfigmap *corev1.ConfigMap) string {
	values := clusterValuesConfigmap.Data[key.ClusterValuesConfigMapKey]
	rex := regexp.MustCompile(fmt.Sprintf(`(%v)(\s*:\s*)(\S+)`, key.BaseDomainKey))
	if matches := rex.FindStringSubmatch(values); len(matches) > 3 {
		return matches[3]
	}
	return ""
}

func GetDexSecretConfig(namespace string) v1alpha1.AppExtraConfig {
	return v1alpha1.AppExtraConfig{
		Kind:      "secret",
		Name:      key.DexConfigName,
		Namespace: namespace,
		Priority:  25}
}

func getOldConnectorsFromSecret(secret *corev1.Secret) (map[string]dex.Connector, error) {
	connectors := map[string]dex.Connector{}
	configData, exists := secret.Data["default"]
	if !exists {
		return connectors, nil
	}
	config := &dex.DexConfig{}
	if err := json.Unmarshal(configData, config); err != nil {
		return nil, microerror.Mask(err)
	}

	for _, connector := range config.Oidc.Customer.Connectors {
		connectors[connector.ID] = connector
	}
	for _, connector := range config.Oidc.Giantswarm.Connectors {
		connectors[connector.ID] = connector
	}
	return connectors, nil
}
