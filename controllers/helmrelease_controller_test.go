package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/giantswarm/dex-operator/pkg/key"
)

const userConnectors = `
oidc:
  giantswarm:
    connectors:
    - id: user-github
`

// helmReleaseWriteGuard fails the test on any Update of a HelmRelease, and on
// any HelmRelease patch that touches more than metadata. Flux owns the spec.
func helmReleaseWriteGuard(t *testing.T) interceptor.Funcs {
	t.Helper()
	return interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*helmv2.HelmRelease); ok {
				t.Errorf("unexpected Update of HelmRelease %s", obj.GetName())
				return errors.New("update of HelmRelease not allowed")
			}
			return c.Update(ctx, obj, opts...)
		},
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if _, ok := obj.(*helmv2.HelmRelease); ok {
				data, err := patch.Data(obj)
				if err != nil {
					return err
				}
				var fields map[string]interface{}
				if err := json.Unmarshal(data, &fields); err != nil {
					return err
				}
				for field := range fields {
					if field != "metadata" {
						t.Errorf("HelmRelease patch touches %q: %s", field, data)
					}
				}
			}
			return c.Patch(ctx, obj, patch, opts...)
		},
	}
}

func getHelmRelease(t *testing.T, c client.Client) *helmv2.HelmRelease {
	t.Helper()
	hr := &helmv2.HelmRelease{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: siblingName, Namespace: siblingNamespace}, hr); err != nil {
		t.Fatalf("get HelmRelease: %v", err)
	}
	return hr
}

// A HelmRelease without the Flux labels used to get the secret added to
// spec.valuesFrom. Now only the finalizer is added, with a patch.
func TestHelmReleaseReconcile_OnlyAddsFinalizer(t *testing.T) {
	s := siblingScheme(t)
	hr := siblingHelmRelease(false)
	hr.Finalizers = nil
	hr.Spec.ValuesFrom = nil
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(hr).WithInterceptorFuncs(helmReleaseWriteGuard(t)).Build()

	reconcileSibling(t, c, s, hr)

	got := getHelmRelease(t, c)
	if !controllerutil.ContainsFinalizer(got, key.DexOperatorFinalizer) {
		t.Errorf("expected the finalizer, got %v", got.Finalizers)
	}
	if len(got.Spec.ValuesFrom) != 0 {
		t.Errorf("expected spec.valuesFrom untouched, got %v", got.Spec.ValuesFrom)
	}
	secret := &corev1.Secret{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: key.GetDexConfigName(siblingName), Namespace: siblingNamespace}, secret); err != nil {
		t.Errorf("expected the dex config secret to be created: %v", err)
	}
}

// User connectors under an owner dex-operator manages: an App CR backs off and
// deletes the secret, a HelmRelease keeps it, because its manifest references it.
func TestReconcile_UserConnectorClash(t *testing.T) {
	secretKey := types.NamespacedName{Name: key.GetDexConfigName(siblingName), Namespace: siblingNamespace}
	userValues := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "dex-app-user-values", Namespace: siblingNamespace},
		Data:       map[string]string{key.ValuesConfigMapKey: userConnectors},
	}

	t.Run("HelmRelease keeps the secret", func(t *testing.T) {
		s := siblingScheme(t)
		hr := siblingHelmRelease(false)
		hr.Spec.ValuesFrom = append(hr.Spec.ValuesFrom, helmv2.ValuesReference{Kind: "ConfigMap", Name: userValues.Name, ValuesKey: key.ValuesConfigMapKey})
		want := hr.Spec.ValuesFrom
		c := fake.NewClientBuilder().WithScheme(s).
			WithObjects(hr, userValues.DeepCopy(), siblingSecret(t, s, hr)).
			WithInterceptorFuncs(helmReleaseWriteGuard(t)).Build()

		reconcileSibling(t, c, s, hr)

		secret := &corev1.Secret{}
		if err := c.Get(context.Background(), secretKey, secret); err != nil {
			t.Fatalf("expected the dex config secret to be kept: %v", err)
		}
		if !controllerutil.ContainsFinalizer(secret, key.DexOperatorFinalizer) {
			t.Errorf("dex config secret lost its finalizer")
		}
		// Still reconciled, so provider credentials keep being renewed.
		if string(secret.Data["default"]) == `{}` {
			t.Errorf("expected the dex config secret to be updated with the managed connectors")
		}
		if got := getHelmRelease(t, c).Spec.ValuesFrom; !reflect.DeepEqual(got, want) {
			t.Errorf("expected spec.valuesFrom untouched, got %v", got)
		}
	})

	t.Run("App backs off and deletes the secret", func(t *testing.T) {
		s := siblingScheme(t)
		app := siblingApp(false)
		app.Spec.UserConfig.ConfigMap = v1alpha1.AppSpecUserConfigConfigMap{Name: userValues.Name, Namespace: siblingNamespace}
		c := fake.NewClientBuilder().WithScheme(s).WithObjects(app, userValues.DeepCopy(), siblingSecret(t, s, app)).Build()

		reconcileSibling(t, c, s, app)

		if err := c.Get(context.Background(), secretKey, &corev1.Secret{}); !apierrors.IsNotFound(err) {
			t.Fatalf("expected the dex config secret to be deleted, got %v", err)
		}
		got := &v1alpha1.App{}
		if err := c.Get(context.Background(), types.NamespacedName{Name: siblingName, Namespace: siblingNamespace}, got); err != nil {
			t.Fatal(err)
		}
		if len(got.Spec.ExtraConfigs) != 0 {
			t.Errorf("expected the secret reference to be removed, got %v", got.Spec.ExtraConfigs)
		}
	})
}

// dex-operator creates the secret whether or not the HelmRelease references it,
// so deleting the HelmRelease cleans it up either way.
func TestHelmReleaseDelete_WithoutSecretReference(t *testing.T) {
	s := siblingScheme(t)
	hr := siblingHelmRelease(true)
	hr.Spec.ValuesFrom = nil
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(hr, siblingSecret(t, s, hr)).WithInterceptorFuncs(helmReleaseWriteGuard(t)).Build()

	reconcileSibling(t, c, s, hr)

	if err := c.Get(context.Background(), types.NamespacedName{Name: siblingName, Namespace: siblingNamespace}, &helmv2.HelmRelease{}); !apierrors.IsNotFound(err) {
		t.Errorf("expected the HelmRelease to be gone, got %v", err)
	}
	if err := c.Get(context.Background(), types.NamespacedName{Name: key.GetDexConfigName(siblingName), Namespace: siblingNamespace}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Errorf("expected the dex config secret to be deleted, got %v", err)
	}
}
