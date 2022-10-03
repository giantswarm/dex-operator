package controllers

import (
	"reflect"
	"time"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	DexOperatorLabelValue = "dex-operator"
	DexConfigSecretName   = "default-dex-config-secret"
	DexOperatorFinalizer  = "dex-operator.finalizers.giantswarm.io"
)

func DefaultRequeue() reconcile.Result {
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: time.Minute * 5,
	}
}

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
