package clusteroidc

import (
	"github.com/giantswarm/dex-operator/pkg/key"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
)

const (
	extraConfigKindConfigMap = "configMap"
)

func oidcExtraConfigPresent(app *v1alpha1.App) bool {
	if app.Spec.ExtraConfigs != nil {
		extraConfigName := key.GetClusterOIDCConfigName(app.Name)
		for _, ec := range app.Spec.ExtraConfigs {
			if ec.Name == extraConfigName && ec.Namespace == app.Namespace && ec.Kind == extraConfigKindConfigMap {
				return true
			}
		}
	}
	return false
}

func GetOIDCFlagsExtraConfig(app *v1alpha1.App) v1alpha1.AppExtraConfig {
	return v1alpha1.AppExtraConfig{
		Kind:      extraConfigKindConfigMap,
		Name:      key.GetClusterOIDCConfigName(app.Name),
		Namespace: app.Namespace,
		Priority:  150,
	}
}
