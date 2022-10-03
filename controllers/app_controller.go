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

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/giantswarm/k8smetadata/pkg/label"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// AppReconciler reconciles a App object
type AppReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	LabelSelector metav1.LabelSelector
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

	// App is deleted.
	if !app.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(app, DexOperatorFinalizer) {
			return r.ReconcileDelete(ctx, app, log)
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(app, DexOperatorFinalizer) {
		controllerutil.AddFinalizer(app, DexOperatorFinalizer)
		if err := r.Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to dex app instance.")
	}
	// App is not deleted
	return r.ReconcileCreateOrUpdate(ctx, app, log)
}

func (r *AppReconciler) ReconcileCreateOrUpdate(ctx context.Context, app *v1alpha1.App, log logr.Logger) (ctrl.Result, error) {
	// Add secret config to app instance
	dexSecretConfig := getDexSecretConfig(app)
	if !dexSecretConfigIsPresent(app, dexSecretConfig) {
		app.Spec.ExtraConfigs = append(app.Spec.ExtraConfigs, dexSecretConfig)
		if err := r.Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Added secret config to dex app instance.")
	}

	// Fetch secret
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: dexSecretConfig.Name, Namespace: dexSecretConfig.Namespace}, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		// Create Secret if not found
		// TODO: idp logic
		secret = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      dexSecretConfig.Name,
				Namespace: dexSecretConfig.Namespace,
				Labels: map[string]string{
					label.ManagedBy: DexOperatorLabelValue,
				},
			},
		}
		if err := r.Create(ctx, secret); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Created default dex config secret for dex app instance.")
	}

	// Update secret if needed
	// TODO: idp logic
	return DefaultRequeue(), nil
}

func (r *AppReconciler) ReconcileDelete(ctx context.Context, app *v1alpha1.App, log logr.Logger) (ctrl.Result, error) {
	// Fetch secret if present
	dexSecretConfig := getDexSecretConfig(app)
	if dexSecretConfigIsPresent(app, dexSecretConfig) {
		secret := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{Name: dexSecretConfig.Name, Namespace: dexSecretConfig.Namespace}, secret); err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		} else {

			// TODO: idp logic
			//delete secret if it exists
			if err := r.Delete(ctx, secret); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
			}
			log.Info("Deleted default dex config secret for dex app instance.")
		}
	}

	// remove finalizer
	if controllerutil.ContainsFinalizer(app, DexOperatorFinalizer) {
		controllerutil.RemoveFinalizer(app, DexOperatorFinalizer)
		if err := r.Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Removed finalizer from dex app instance.")
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	predicate, err := predicate.LabelSelectorPredicate(r.LabelSelector)
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.App{}).
		WithEventFilter(predicate).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func getDexSecretConfig(app *v1alpha1.App) v1alpha1.AppExtraConfig {
	return v1alpha1.AppExtraConfig{
		Kind:      "secret",
		Name:      DexConfigSecretName,
		Namespace: app.Namespace}
}
