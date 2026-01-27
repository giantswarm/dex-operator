package controllers

import (
	"context"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/giantswarm/dex-operator/pkg/auth"
	"github.com/giantswarm/dex-operator/pkg/dextarget"
	"github.com/giantswarm/dex-operator/pkg/idp"
	"github.com/giantswarm/dex-operator/pkg/idp/provider"
	"github.com/giantswarm/dex-operator/pkg/key"
)

// HelmReleaseReconciler reconciles a Flux HelmRelease object for dex-app
type HelmReleaseReconciler struct {
	client.Client
	Log                      logr.Logger
	Recorder                 record.EventRecorder
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

//+kubebuilder:rbac:groups=helm.toolkit.fluxcd.io,resources=helmreleases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=helm.toolkit.fluxcd.io,resources=helmreleases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=helm.toolkit.fluxcd.io,resources=helmreleases/finalizers,verbs=update

func (r *HelmReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("helmrelease", req.NamespacedName)

	// Fetch the HelmRelease instance
	hr := &helmv2.HelmRelease{}
	if err := r.Get(ctx, req.NamespacedName, hr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, microerror.Mask(err)
	}

	// HelmRelease takes priority over App CR
	// The App controller will skip reconciliation if a HelmRelease exists

	// Wrap in DexTarget
	target := dextarget.NewHelmReleaseTarget(hr)

	var authService *auth.Service
	{
		writeAllGroups, err := r.GetWriteAllGroups()
		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}

		c := auth.Config{
			Log:                             log,
			Client:                          r.Client,
			Target:                          target,
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
			Target:                         target,
			Providers:                      providers,
			ManagementClusterBaseDomain:    r.BaseDomain,
			ManagementClusterIssuerAddress: r.IssuerAddress,
			ManagementClusterName:          r.ManagementCluster,
			Owner:                          hr,
			Scheme:                         r.Scheme,
		}

		idpService, err = idp.New(c)
		if err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
	}

	// HelmRelease is deleted
	if target.IsBeingDeleted() {
		if err := idpService.ReconcileDelete(ctx); err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
		if err := authService.ReconcileDelete(ctx); err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
		// Remove finalizer
		if controllerutil.ContainsFinalizer(hr, key.DexOperatorFinalizer) {
			controllerutil.RemoveFinalizer(hr, key.DexOperatorFinalizer)
			if err := r.Update(ctx, hr); err != nil {
				return ctrl.Result{}, microerror.Mask(err)
			}
			log.Info("Removed finalizer from dex HelmRelease instance.")
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(hr, key.DexOperatorFinalizer) {
		controllerutil.AddFinalizer(hr, key.DexOperatorFinalizer)
		if err := r.Update(ctx, hr); err != nil {
			return ctrl.Result{}, microerror.Mask(err)
		}
		log.Info("Added finalizer to dex HelmRelease instance.")
	}

	// Reconcile auth configuration (for workload clusters)
	if err := authService.Reconcile(ctx); err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	// Reconcile IDP configuration
	if err := idpService.Reconcile(ctx); err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	// Self-renewal for management cluster dex HelmRelease
	if r.EnableSelfRenewal && key.IsManagementClusterDexHelmRelease(hr.Name, hr.Namespace) {
		if err := idpService.CheckAndRotateServiceCredentials(ctx); err != nil {
			log.Error(err, "Service credential rotation failed")
			// Emit a warning event so users can monitor rotation failures
			r.Recorder.Event(hr, corev1.EventTypeWarning, "CredentialRotationFailed",
				"Failed to rotate service credentials: "+err.Error())
			// Don't fail the reconciliation, just log the error
		}
	}

	return DefaultRequeue(), nil
}

func (r *HelmReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	labelPredicate, err := predicate.LabelSelectorPredicate(r.LabelSelector)
	if err != nil {
		return microerror.Mask(err)
	}
	namespacedNamePredicate, err := helmReleaseNamespacedNamePredicate(key.MCDexHelmReleaseDefaultNamespacedName())
	if err != nil {
		return microerror.Mask(err)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&helmv2.HelmRelease{}).
		WithEventFilter(predicate.Or(labelPredicate, namespacedNamePredicate)).
		Owns(&corev1.Secret{}).
		Complete(r)
}

// helmReleaseNamespacedNamePredicate constructs a Predicate from a namespaced name.
// Only objects matching the namespaced name will be admitted.
func helmReleaseNamespacedNamePredicate(s types.NamespacedName) (predicate.Predicate, error) {
	return predicate.NewPredicateFuncs(func(o client.Object) bool {
		return o.GetName() == s.Name && o.GetNamespace() == s.Namespace
	}), nil
}

func (r *HelmReleaseReconciler) GetProviders() ([]provider.Provider, error) {
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

func (r *HelmReleaseReconciler) GetWriteAllGroups() ([]string, error) {
	return append(r.GiantswarmWriteAllGroups, r.CustomerWriteAllGroups...), nil
}
