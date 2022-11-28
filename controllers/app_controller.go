/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"giantswarm/dex-operator/pkg/idp"
	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/idp/provider/azure"
	"giantswarm/dex-operator/pkg/idp/provider/mockprovider"
	"giantswarm/dex-operator/pkg/key"
	"time"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AppReconciler reconciles a App object
type AppReconciler struct {
	client.Client
	Log                 logr.Logger
	Scheme              *runtime.Scheme
	LabelSelector       metav1.LabelSelector
	BaseDomain          string
	ManagementCluster   string
	ProviderCredentials string
}

//+kubebuilder:rbac:groups=application.giantswarm.io.giantswarm,resources=apps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=application.giantswarm.io.giantswarm,resources=apps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=application.giantswarm.io.giantswarm,resources=apps/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the App object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("app", req.NamespacedName)

	// Fetch the App instance.
	app := &v1alpha1.App{}
	if err := r.Get(ctx, req.NamespacedName, app); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found. Return
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	var idpService *idp.Service
	{
		providers, err := r.GetProviders()
		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}

		c := idp.Config{
			Log:                         &log,
			Client:                      r.Client,
			App:                         app,
			Providers:                   providers,
			ManagementClusterBaseDomain: r.BaseDomain,
			ManagementClusterName:       r.ManagementCluster,
		}

		idpService, err = idp.New(c)
		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
	}

	// App is deleted.
	if !app.DeletionTimestamp.IsZero() {
		if err := idpService.ReconcileDelete(ctx); err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
		// remove finalizer
		if controllerutil.ContainsFinalizer(app, key.DexOperatorFinalizer) {
			controllerutil.RemoveFinalizer(app, key.DexOperatorFinalizer)
			if err := r.Update(ctx, app); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("Removed finalizer from dex app instance.")
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(app, key.DexOperatorFinalizer) {
		controllerutil.AddFinalizer(app, key.DexOperatorFinalizer)
		if err := r.Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to dex app instance.")
	}
	// App is not deleted
	if err := idpService.Reconcile(ctx); err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}
	return DefaultRequeue(), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	predicate, err := predicate.LabelSelectorPredicate(r.LabelSelector)
	if err != nil {
		return microerror.Mask(err)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.App{}).
		WithEventFilter(predicate).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func DefaultRequeue() reconcile.Result {
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: time.Minute * 5,
	}
}

func (r *AppReconciler) GetProviders() ([]provider.Provider, error) {
	providerCredentials, err := provider.ReadCredentials(r.ProviderCredentials)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	var providers []provider.Provider
	{
		for _, p := range providerCredentials {
			provider, err := r.NewProvider(p)
			if err != nil {
				return nil, microerror.Mask(err)
			}
			providers = append(providers, provider)
		}
	}
	return providers, nil
}

func (r *AppReconciler) NewProvider(p provider.ProviderCredential) (provider.Provider, error) {
	switch p.Name {
	case mockprovider.ProviderName:
		return mockprovider.New(p)
	case azure.ProviderName:
		return azure.New(p, &r.Log)
	}
	return nil, microerror.Maskf(invalidConfigError, "%s is not a valid provider name.", p.Name)
}
