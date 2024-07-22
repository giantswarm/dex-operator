package app

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/giantswarm/dex-operator/pkg/key"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func RemoveExtraConfig(extraConfigs []v1alpha1.AppExtraConfig, extraConfig v1alpha1.AppExtraConfig) []v1alpha1.AppExtraConfig {
	if extraConfigs == nil {
		return extraConfigs
	}
	result := []v1alpha1.AppExtraConfig{}
	for _, config := range extraConfigs {
		if !reflect.DeepEqual(config, extraConfig) {
			result = append(result, config)
		}
	}
	return result
}

func ClusterValuesIsPresent(app *v1alpha1.App) bool {
	return strings.HasSuffix(app.Spec.Config.ConfigMap.Name, key.ClusterValuesConfigmapSuffix)
}

func GetIssuerAddress(baseDomain string, managementClusterIssuerAddress string, managementClusterBaseDomain string) string {
	var issuerAddress string
	{
		// Derive issuer address from cluster basedomain if it exists
		if baseDomain != "" {
			issuerAddress = key.GetIssuerAddress(baseDomain)
		}

		// Otherwise fall back to management cluster issuer address if present
		if issuerAddress == "" {
			issuerAddress = managementClusterIssuerAddress
		}

		// If all else fails, fall back to the base domain (only works in vintage)
		if issuerAddress == "" {
			clusterDomain := key.GetVintageClusterDomain(managementClusterBaseDomain)
			issuerAddress = key.GetIssuerAddress(clusterDomain)
		}
	}
	return issuerAddress
}

func GetBaseDomainFromClusterValues(clusterValuesConfigmap *corev1.ConfigMap) string {
	values := clusterValuesConfigmap.Data[key.ValuesConfigMapKey]
	rex := regexp.MustCompile(fmt.Sprintf(`(%v)(\s*:\s*)(\S+)`, key.BaseDomainKey))
	if matches := rex.FindStringSubmatch(values); len(matches) > 3 {
		return matches[3]
	}
	return ""
}
