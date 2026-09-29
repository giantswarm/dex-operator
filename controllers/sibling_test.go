package controllers

import (
	"context"
	"testing"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/go-logr/logr/testr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/giantswarm/dex-operator/pkg/idp"
	"github.com/giantswarm/dex-operator/pkg/key"
)

const (
	siblingName      = "dex-app"
	siblingNamespace = "org-example"
	appUID           = types.UID("app-uid")
	helmReleaseUID   = types.UID("hr-uid")
	providerFixture  = "test-data/credentials"
)

func siblingScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, v1alpha1.AddToScheme, helmv2.AddToScheme} {
		if err := add(s); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func siblingApp(deleted bool) *v1alpha1.App {
	nn := types.NamespacedName{Name: siblingName, Namespace: siblingNamespace}
	app := &v1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:       siblingName,
			Namespace:  siblingNamespace,
			UID:        appUID,
			Labels:     map[string]string{key.AppLabel: key.DexAppLabelValue},
			Finalizers: []string{key.DexOperatorFinalizer},
		},
		Spec: v1alpha1.AppSpec{ExtraConfigs: []v1alpha1.AppExtraConfig{idp.GetDexSecretConfig(nn)}},
	}
	if deleted {
		app.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	}
	return app
}

func siblingHelmRelease(deleted bool) *helmv2.HelmRelease {
	hr := &helmv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:       siblingName,
			Namespace:  siblingNamespace,
			UID:        helmReleaseUID,
			Labels:     map[string]string{key.AppLabel: key.DexAppLabelValue},
			Finalizers: []string{key.DexOperatorFinalizer},
		},
		Spec: helmv2.HelmReleaseSpec{ValuesFrom: []helmv2.ValuesReference{
			{Kind: "Secret", Name: key.GetDexConfigName(siblingName), ValuesKey: "default"},
		}},
	}
	if deleted {
		hr.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	}
	return hr
}

func siblingSecret(t *testing.T, s *runtime.Scheme, owner client.Object) *corev1.Secret {
	t.Helper()
	secret := idp.GetDefaultDexConfigSecret(key.GetDexConfigName(siblingName), siblingNamespace)
	secret.Finalizers = []string{key.DexOperatorFinalizer}
	secret.Data = map[string][]byte{"default": []byte(`{}`)}
	if err := controllerutil.SetControllerReference(owner, secret, s); err != nil {
		t.Fatal(err)
	}
	return secret
}

func reconcileSibling(t *testing.T, c client.Client, s *runtime.Scheme, target client.Object) {
	t.Helper()
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: siblingName, Namespace: siblingNamespace}}
	var err error
	switch target.(type) {
	case *helmv2.HelmRelease:
		r := &HelmReleaseReconciler{Client: c, Log: testr.New(t), Scheme: s, BaseDomain: "example.io", ManagementCluster: "mc", GiantswarmWriteAllGroups: []string{"admins"}, ProviderCredentials: providerFixture}
		_, err = r.Reconcile(context.Background(), req)
	case *v1alpha1.App:
		r := &AppReconciler{Client: c, Log: testr.New(t), Scheme: s, BaseDomain: "example.io", ManagementCluster: "mc", GiantswarmWriteAllGroups: []string{"admins"}, ProviderCredentials: providerFixture}
		_, err = r.Reconcile(context.Background(), req)
	}
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

func TestDeletionWithSibling(t *testing.T) {
	secretKey := types.NamespacedName{Name: key.GetDexConfigName(siblingName), Namespace: siblingNamespace}
	targetKey := types.NamespacedName{Name: siblingName, Namespace: siblingNamespace}

	testCases := []struct {
		name string
		// deleted is the target being deleted, sibling the same-named target or nil.
		deleted client.Object
		sibling client.Object
		// secretOwner is the controller of the dex config secret before the deletion.
		secretOwner func(deleted, sibling client.Object) client.Object
		// expectedController is the controller UID of the secret afterwards, empty if
		// the secret has to be deleted.
		expectedController types.UID
	}{
		{
			name:               "HelmRelease deleted, App lives on, secret created by the HelmRelease",
			deleted:            siblingHelmRelease(true),
			sibling:            siblingApp(false),
			secretOwner:        func(deleted, _ client.Object) client.Object { return deleted },
			expectedController: appUID,
		},
		{
			name:               "HelmRelease deleted, App lives on, secret created by the App",
			deleted:            siblingHelmRelease(true),
			sibling:            siblingApp(false),
			secretOwner:        func(_, sibling client.Object) client.Object { return sibling },
			expectedController: appUID,
		},
		{
			name:               "App deleted, HelmRelease lives on, secret created by the App",
			deleted:            siblingApp(true),
			sibling:            siblingHelmRelease(false),
			secretOwner:        func(deleted, _ client.Object) client.Object { return deleted },
			expectedController: helmReleaseUID,
		},
		{
			name:        "HelmRelease deleted without a sibling",
			deleted:     siblingHelmRelease(true),
			secretOwner: func(deleted, _ client.Object) client.Object { return deleted },
		},
		{
			name:        "App deleted without a sibling",
			deleted:     siblingApp(true),
			secretOwner: func(deleted, _ client.Object) client.Object { return deleted },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := siblingScheme(t)
			objects := []client.Object{tc.deleted, siblingSecret(t, s, tc.secretOwner(tc.deleted, tc.sibling))}
			if tc.sibling != nil {
				objects = append(objects, tc.sibling)
			}
			c := fake.NewClientBuilder().WithScheme(s).WithObjects(objects...).Build()

			reconcileSibling(t, c, s, tc.deleted)

			// The deleted target is gone: its finalizer was removed.
			if err := c.Get(context.Background(), targetKey, tc.deleted.DeepCopyObject().(client.Object)); !apierrors.IsNotFound(err) {
				t.Fatalf("deleted target still exists (err %v)", err)
			}

			secret := &corev1.Secret{}
			err := c.Get(context.Background(), secretKey, secret)
			if tc.expectedController == "" {
				// Without a sibling the shared state is cleaned up, the secret included.
				if !apierrors.IsNotFound(err) {
					t.Fatalf("expected the dex config secret to be deleted, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected the dex config secret to be kept: %v", err)
			}
			if string(secret.Data["default"]) != `{}` {
				t.Errorf("dex config secret data changed: %q", secret.Data["default"])
			}
			if !controllerutil.ContainsFinalizer(secret, key.DexOperatorFinalizer) {
				t.Errorf("dex config secret lost its finalizer")
			}
			controller := metav1.GetControllerOf(secret)
			if controller == nil || controller.UID != tc.expectedController {
				t.Errorf("expected the secret to be controlled by %s, got %v", tc.expectedController, controller)
			}
			if len(secret.OwnerReferences) != 1 {
				t.Errorf("expected one owner reference, got %v", secret.OwnerReferences)
			}

			// The sibling keeps its finalizer and its reference to the secret.
			sibling := tc.sibling.DeepCopyObject().(client.Object)
			if err := c.Get(context.Background(), targetKey, sibling); err != nil {
				t.Fatalf("sibling: %v", err)
			}
			if !controllerutil.ContainsFinalizer(sibling, key.DexOperatorFinalizer) {
				t.Errorf("sibling lost its finalizer")
			}
		})
	}
}
