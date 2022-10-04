package idp

import (
	"reflect"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
)

const (
	DexConfigName = "default-dex-config"
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

func getDexSecretConfig(app *v1alpha1.App) v1alpha1.AppExtraConfig {
	return v1alpha1.AppExtraConfig{
		Kind:      "secret",
		Name:      DexConfigName,
		Namespace: app.Namespace}
}
