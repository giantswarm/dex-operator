package idp

import (
	"fmt"
	"giantswarm/dex-operator/pkg/key"
	"reflect"
	"regexp"
	"strings"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
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
