package controllers

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/giantswarm/dex-operator/pkg/key"
)

// An App CR and a HelmRelease with the same namespaced name share one set of identity
// provider apps (named after the namespaced name) and one dex config secret. While one
// of them is deleted and the other lives on, e.g. during an App CR to HelmRelease
// migration or its rollback, the deleted one must not clean up the shared state.

// handOverToSibling finishes the deletion of a dex target whose sibling lives on: it leaves
// the identity provider apps and the dex config secret untouched, hands the controller
// reference of the secret to the sibling, so that garbage collection keeps the secret and
// the sibling's controller reconciles at once, and removes the operator finalizer from the
// deleted target.
func handOverToSibling(ctx context.Context, c client.Client, scheme *runtime.Scheme, log logr.Logger, deleted, sibling client.Object, siblingKind string) error {
	secret := &corev1.Secret{}
	nn := types.NamespacedName{Name: key.GetDexConfigName(deleted.GetName()), Namespace: deleted.GetNamespace()}
	if err := c.Get(ctx, nn, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return microerror.Mask(err)
		}
	} else if metav1.IsControlledBy(secret, deleted) {
		if err := controllerutil.RemoveControllerReference(deleted, secret, scheme); err != nil {
			return microerror.Mask(err)
		}
		if err := controllerutil.SetControllerReference(sibling, secret, scheme); err != nil {
			return microerror.Mask(err)
		}
		if err := c.Update(ctx, secret); err != nil {
			return microerror.Mask(err)
		}
		log.Info(fmt.Sprintf("Handed dex config secret %s over to the %s with the same name.", nn.Name, siblingKind))
	}

	if controllerutil.RemoveFinalizer(deleted, key.DexOperatorFinalizer) {
		if err := c.Update(ctx, deleted); err != nil {
			return microerror.Mask(err)
		}
	}
	log.Info(fmt.Sprintf("Removed finalizer without cleanup: the %s with the same name keeps the identity provider apps and the dex config secret.", siblingKind))
	return nil
}
