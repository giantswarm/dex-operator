package controllers

import (
	"context"
	"time"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/dex-operator/pkg/auth"
	"github.com/giantswarm/dex-operator/pkg/idp"
	"github.com/giantswarm/dex-operator/pkg/idp/provider"
	"github.com/giantswarm/dex-operator/pkg/idp/provider/azure"
	"github.com/giantswarm/dex-operator/pkg/idp/provider/github"
	"github.com/giantswarm/dex-operator/pkg/idp/provider/mockprovider"
	"github.com/giantswarm/dex-operator/pkg/idp/provider/simpleprovider"
	"github.com/giantswarm/dex-operator/pkg/key"
)

// AppReconciler reconciles a App object
type AppReconciler struct {
	client.Client
	Log                      logr.Logger
	Scheme                   *runtime.Scheme
	LabelSelector            metav1.LabelSelector
	BaseDomain               string
	IssuerAddress            string
	ManagementCluster        string
	ProviderCredentials      string
	GiantswarmWriteAllGroups []string
	CustomerWriteAllGroups   []string
	EnableSelfRenewal        bool
}

//+kubebuilder:rbac:groups=application.giantswarm.io.giantswarm,resources=apps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=application.giantswarm.io.giantswarm,resources=apps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=application.giantswarm.io.giantswarm,resources=apps/finalizers,verbs=update

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

	var authService *auth.Service
	{
		writeAllGroups, err := r.GetWriteAllGroups()
		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}

		c := auth.Config{
			Log:                             log,
			Client:                          r.Client,
			App:                             app,
			ManagementClusterName:           r.ManagementCluster,
			ManagementClusterWriteAllGroups: writeAllGroups,
		}

		authService, err = auth.New(c)
		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
	}

	var idpService *idp.Service
	{
		providers, err := r.GetProviders()
		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}

		c := idp.Config{
			Log:                            log,
			Client:                         r.Client,
			App:                            app,
			Providers:                      providers,
			ManagementClusterBaseDomain:    r.BaseDomain,
			ManagementClusterIssuerAddress: r.IssuerAddress,
			ManagementClusterName:          r.ManagementCluster,
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
		if err := authService.ReconcileDelete(ctx); err != nil {
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
	if err := authService.Reconcile(ctx); err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}
	if err := idpService.Reconcile(ctx); err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	if r.EnableSelfRenewal && key.IsManagementClusterDexApp(app) {
		if err := idpService.CheckAndRotateServiceCredentials(ctx); err != nil {
			log.Error(err, "Service credential rotation failed")
			// Don't fail the reconciliation, just log the error
			// This prevents self-renewal issues from blocking normal dex operations
		}
	}

	return DefaultRequeue(), nil
}

func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	labelPredicate, err := predicate.LabelSelectorPredicate(r.LabelSelector)
	if err != nil {
		return microerror.Mask(err)
	}
	namespacedNamePredicate, err := namespacedNamePredicate(key.MCDexDefaultNamespacedName())
	if err != nil {
		return microerror.Mask(err)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.App{}).
		WithEventFilter(predicate.Or(labelPredicate, namespacedNamePredicate)).
		Owns(&corev1.Secret{}).
		Complete(r)
}

// namespacedNamePredicate constructs a Predicate from a namespaced name.
// Only objects matching the namespaced name will be admitted.
func namespacedNamePredicate(s types.NamespacedName) (predicate.Predicate, error) {
	return predicate.NewPredicateFuncs(func(o client.Object) bool {
		return o.GetName() == s.Name && o.GetNamespace() == s.Namespace
	}), nil
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

	providers := []provider.Provider{}
	for _, p := range providerCredentials {
		config := provider.ProviderConfig{
			Credential:            p,
			Log:                   r.Log,
			ManagementClusterName: r.ManagementCluster,
		}

		provider, err := NewProvider(config)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		providers = append(providers, provider)
	}
	return providers, nil
}

func (r *AppReconciler) GetWriteAllGroups() ([]string, error) {
	return append(r.GiantswarmWriteAllGroups, r.CustomerWriteAllGroups...), nil
}

func NewProvider(config provider.ProviderConfig) (provider.Provider, error) {
	switch config.Credential.Name {
	case mockprovider.ProviderName:
		return mockprovider.New(config)
	case azure.ProviderName:
		return azure.New(config)
	case github.ProviderName:
		return github.New(config)
	case simpleprovider.ProviderName:
		return simpleprovider.New(config)
	}
	return nil, microerror.Maskf(invalidConfigError, "%s is not a valid provider name.", config.Credential.Name)
}

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(idp.AppInfo)
}
