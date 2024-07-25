package clusteroidc

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/giantswarm/dex-operator/pkg/key"
	"github.com/giantswarm/dex-operator/pkg/tests"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	bootstrapv1beta1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	appName                        = "test-app"
	clusterName                    = "test-cluster"
	namespace                      = "default"
	clusterConfigName              = "test-cluster-cluster-values"
	managementClusterName          = "management-cluster"
	managementClusterBaseDomain    = "base.domain.io"
	managementClusterIssuerAddress = "issuer.cluster.base.domain.io"
)

func TestOIDCConfigCreate(t *testing.T) {
	testCases := []struct {
		name                string
		app                 *v1alpha1.App
		clusterApp          *v1alpha1.App
		clusterConfig       *corev1.ConfigMap
		kubeadmControlPlane *v1beta1.KubeadmControlPlane
		oidcConfig          *corev1.ConfigMap
		expectedOidcConfig  *corev1.ConfigMap
		expectedError       error
	}{
		{
			name:                "case 0: App without the OIDC flags annotation",
			app:                 getTestDexApp(appName, namespace, clusterName, clusterConfigName, false),
			clusterApp:          getTestClusterApp(clusterName, namespace, clusterConfigName, ""),
			clusterConfig:       tests.GetClusterValuesConfigMap(clusterConfigName, namespace, "baseDomain: wc.cluster.domain.io"),
			kubeadmControlPlane: getTestKubeadmControlPlane(""),
		},
		{
			name:                "case 1: App with the OIDC flags annotation",
			app:                 getTestDexApp(appName, namespace, clusterName, clusterConfigName, true),
			clusterApp:          getTestClusterApp(clusterName, namespace, "", ""),
			clusterConfig:       tests.GetClusterValuesConfigMap(clusterConfigName, namespace, "baseDomain: wc.cluster.domain.io"),
			kubeadmControlPlane: getTestKubeadmControlPlane(""),
			expectedOidcConfig:  getOIDCConfig("https://dex.wc.cluster.domain.io"),
		},
		{
			name:                "case 2: App with the annotation and existing OIDC flags",
			app:                 getTestDexApp(appName, namespace, clusterName, clusterConfigName, true),
			clusterApp:          getTestClusterApp(clusterName, namespace, clusterConfigName, ""),
			clusterConfig:       tests.GetClusterValuesConfigMap(clusterConfigName, namespace, "baseDomain: wc.cluster.domain.io"),
			kubeadmControlPlane: getTestKubeadmControlPlane("https://issuer.externaldomain.io"),
		},
		{
			name:                "case 3: App with the annotation and existing OIDC config with the same values",
			app:                 getTestDexApp(appName, namespace, clusterName, clusterConfigName, true),
			clusterApp:          getTestClusterApp(clusterName, namespace, clusterConfigName, ""),
			clusterConfig:       tests.GetClusterValuesConfigMap(clusterConfigName, namespace, "baseDomain: wc.cluster.domain.io"),
			oidcConfig:          getOIDCConfig("https://dex.wc.cluster.domain.io"),
			kubeadmControlPlane: getTestKubeadmControlPlane(""),
			expectedOidcConfig:  getOIDCConfig("https://dex.wc.cluster.domain.io"),
		},
		{
			name:                "case 4: App with the annotation and existing OIDC config with different values",
			app:                 getTestDexApp(appName, namespace, clusterName, clusterConfigName, true),
			clusterApp:          getTestClusterApp(clusterName, namespace, clusterConfigName, key.GetClusterOIDCConfigName(appName)),
			clusterConfig:       tests.GetClusterValuesConfigMap(clusterConfigName, namespace, "baseDomain: wc.cluster.domain.io"),
			oidcConfig:          getOIDCConfig("https://dex.wc.cluster.olddomain.io"),
			kubeadmControlPlane: getTestKubeadmControlPlane(""),
			expectedOidcConfig:  getOIDCConfig("https://dex.wc.cluster.domain.io"),
		},
		{
			name:                "case 5: App with annotation without cluster label",
			app:                 getTestDexApp(appName, namespace, "", clusterConfigName, true),
			clusterApp:          getTestClusterApp(clusterName, namespace, clusterConfigName, ""),
			clusterConfig:       tests.GetClusterValuesConfigMap(clusterConfigName, namespace, "baseDomain: wc.cluster.domain.io"),
			kubeadmControlPlane: getTestKubeadmControlPlane(""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			ctx := context.Background()

			err := v1alpha1.AddToScheme(scheme.Scheme)
			if err != nil {
				t.Fatal(err)
			}

			err = v1beta1.AddToScheme(scheme.Scheme)
			if err != nil {
				t.Fatal(err)
			}

			var initObjects []client.Object
			if tc.app != nil {
				initObjects = append(initObjects, tc.app)
			}
			if tc.clusterApp != nil {
				initObjects = append(initObjects, tc.clusterApp)
			}
			if tc.clusterConfig != nil {
				initObjects = append(initObjects, tc.clusterConfig)
			}
			if tc.kubeadmControlPlane != nil {
				initObjects = append(initObjects, tc.kubeadmControlPlane)
			}
			if tc.oidcConfig != nil {
				initObjects = append(initObjects, tc.oidcConfig)
			}

			fakeClient := fake.NewClientBuilder().WithObjects(initObjects...).Build()

			service, err := New(Config{
				Client:                         fakeClient,
				Log:                            ctrl.Log.WithName("test"),
				App:                            tc.app,
				ManagementClusterBaseDomain:    managementClusterBaseDomain,
				ManagementClusterIssuerAddress: managementClusterIssuerAddress,
				ManagementClusterName:          managementClusterName,
			})

			if err != nil {
				t.Fatal(err)
			}

			err = service.Reconcile(ctx)
			if tc.expectedError != nil {
				if err == nil {
					t.Fatalf("did not receive expected error %v", tc.expectedError)
				}
				if !errors.Is(err, tc.expectedError) {
					t.Fatalf("received unexpected error - expected %v, actual %v", tc.expectedError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("received unexpected error %v", err)
			}

			actualApp := &v1alpha1.App{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: tc.app.Name, Namespace: tc.app.Namespace}, actualApp)
			if err != nil {
				t.Fatalf("received unexpected error %v", err)
			}

			if _, ok := actualApp.ObjectMeta.Annotations[key.UpdateOIDCFlagsAnnotationName]; ok {
				t.Errorf("found unexpected %s annotation in the app", key.UpdateOIDCFlagsAnnotationName)
			}

			actualClusterApp := &v1alpha1.App{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: tc.clusterApp.Name, Namespace: tc.clusterApp.Namespace}, actualClusterApp)
			if err != nil {
				t.Fatalf("received unexpected error %v", err)
			}

			actualOidcConfig := &corev1.ConfigMap{}
			actualOidcConfigExists := true
			err = fakeClient.Get(ctx, types.NamespacedName{Name: key.GetClusterOIDCConfigName(actualClusterApp.Name), Namespace: actualApp.Namespace}, actualOidcConfig)
			if err != nil {
				if apierrors.IsNotFound(err) {
					actualOidcConfigExists = false
				} else {
					t.Errorf("received unexpected error %v", err)
				}
			}
			if tc.expectedOidcConfig != nil {
				if !actualOidcConfigExists {
					t.Errorf("did not receive expected config %v", tc.expectedOidcConfig)
				}

				if tc.expectedOidcConfig.Name != actualOidcConfig.Name || tc.expectedOidcConfig.Namespace != actualOidcConfig.Namespace {
					t.Errorf("unexpected OIDC config identifier - expected %s/%s, actual %s/%s",
						tc.expectedOidcConfig.Namespace, tc.expectedOidcConfig.Name,
						actualOidcConfig.Namespace, actualOidcConfig.Name)
				}

				if !reflect.DeepEqual(tc.expectedOidcConfig.Data, actualOidcConfig.Data) {
					t.Errorf("unexpected OIDC configmap content - expected %v, actual %v",
						string(tc.expectedOidcConfig.BinaryData[key.ValuesConfigMapKey]),
						string(actualOidcConfig.BinaryData[key.ValuesConfigMapKey]))
				}

				if !oidcExtraConfigPresent(actualClusterApp) {
					t.Errorf("expected extra config %s is not present in app %v", tc.expectedOidcConfig.Name, actualClusterApp)
				}

			} else if actualOidcConfigExists {
				t.Errorf("received unexpected OIDC config %v", actualOidcConfig)
			}
		})
	}
}

func TestOIDCConfigDelete(t *testing.T) {

	// Delete app with OIDC extra config present
	// Delete app without OIDC extra config present

	testCases := []struct {
		name                string
		app                 *v1alpha1.App
		clusterApp          *v1alpha1.App
		clusterConfig       *corev1.ConfigMap
		kubeadmControlPlane *v1beta1.KubeadmControlPlane
		oidcConfig          *corev1.ConfigMap
	}{
		{
			name:                "case 0: Delete app with OIDC annotation and extra config present",
			app:                 getTestDexApp(appName, namespace, clusterName, clusterConfigName, true),
			clusterApp:          getTestClusterApp(clusterName, namespace, clusterConfigName, key.GetClusterOIDCConfigName(appName)),
			clusterConfig:       tests.GetClusterValuesConfigMap(clusterConfigName, namespace, "baseDomain: wc.cluster.domain.io"),
			oidcConfig:          getOIDCConfig("https://dex.wc.cluster.domain.io"),
			kubeadmControlPlane: getTestKubeadmControlPlane("https://dex.wc.cluster.domain.io"),
		},
		{
			name:                "case 1: Delete app without OIDC annotation and with extra config present",
			app:                 getTestDexApp(appName, namespace, clusterName, clusterConfigName, false),
			clusterApp:          getTestClusterApp(clusterName, namespace, clusterConfigName, key.GetClusterOIDCConfigName(appName)),
			clusterConfig:       tests.GetClusterValuesConfigMap(clusterConfigName, namespace, "baseDomain: wc.cluster.domain.io"),
			oidcConfig:          getOIDCConfig("https://dex.wc.cluster.domain.io"),
			kubeadmControlPlane: getTestKubeadmControlPlane("https://dex.wc.cluster.domain.io"),
		},
		{
			name:                "case 2: Delete app without OIDC annotation and extra config",
			app:                 getTestDexApp(appName, namespace, clusterName, clusterConfigName, false),
			clusterApp:          getTestClusterApp(clusterName, namespace, clusterConfigName, ""),
			clusterConfig:       tests.GetClusterValuesConfigMap(clusterConfigName, namespace, "baseDomain: wc.cluster.domain.io"),
			kubeadmControlPlane: getTestKubeadmControlPlane("https://dex.wc.cluster.domain.io"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			err := v1alpha1.AddToScheme(scheme.Scheme)
			if err != nil {
				t.Fatal(err)
			}

			err = v1beta1.AddToScheme(scheme.Scheme)
			if err != nil {
				t.Fatal(err)
			}

			var initObjects []client.Object
			if tc.app != nil {
				initObjects = append(initObjects, tc.app)
			}
			if tc.clusterApp != nil {
				initObjects = append(initObjects, tc.clusterApp)
			}
			if tc.clusterConfig != nil {
				initObjects = append(initObjects, tc.clusterConfig)
			}
			if tc.kubeadmControlPlane != nil {
				initObjects = append(initObjects, tc.kubeadmControlPlane)
			}
			if tc.oidcConfig != nil {
				initObjects = append(initObjects, tc.oidcConfig)
			}

			fakeClient := fake.NewClientBuilder().WithObjects(initObjects...).Build()

			service, err := New(Config{
				Client:                         fakeClient,
				Log:                            ctrl.Log.WithName("test"),
				App:                            tc.app,
				ManagementClusterBaseDomain:    managementClusterBaseDomain,
				ManagementClusterIssuerAddress: managementClusterIssuerAddress,
				ManagementClusterName:          managementClusterName,
			})

			if err != nil {
				t.Fatal(err)
			}

			err = service.ReconcileDelete(ctx)
			if err != nil {
				t.Fatal(err)
			}

			actualClusterApp := &v1alpha1.App{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: tc.clusterApp.Name, Namespace: tc.clusterApp.Namespace}, actualClusterApp)
			if err != nil {
				t.Fatal(err)
			}

			if oidcExtraConfigPresent(actualClusterApp) {
				t.Errorf("found unexpected extra config in %s/%s app: %v", actualClusterApp.Namespace, actualClusterApp.Name, actualClusterApp)
			}

			actualOidcConfig := &corev1.ConfigMap{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: key.GetClusterOIDCConfigName(tc.clusterApp.Name), Namespace: tc.clusterApp.Namespace}, actualOidcConfig)
			if err == nil {
				t.Errorf("found unexpected config map %s/%s", actualOidcConfig.Namespace, actualOidcConfig.Name)
			} else if !apierrors.IsNotFound(err) {
				t.Errorf("received unexpected error %v", err)
			}

		})
	}
}

func getApp(name, namespace, clusterConfigName string) *v1alpha1.App {
	return &v1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.AppSpec{
			Name:      name,
			Namespace: clusterName,
			Config: v1alpha1.AppSpecConfig{
				ConfigMap: v1alpha1.AppSpecConfigConfigMap{
					Name:      clusterConfigName,
					Namespace: namespace,
				},
			},
		},
	}
}

func getTestDexApp(name, namespace, clusterName, clusterConfigName string, oidcAnnotation bool) *v1alpha1.App {
	app := getApp(name, namespace, clusterConfigName)

	if clusterName != "" {
		app.ObjectMeta.Labels = map[string]string{
			key.AppClusterLabel: clusterName,
		}
	}

	if oidcAnnotation {
		app.ObjectMeta.Annotations = map[string]string{
			key.UpdateOIDCFlagsAnnotationName: key.UpdateOIDCFlagsAnnotationValue,
		}
	}

	return app
}

func getTestClusterApp(name, namespace, clusterConfigName, extraConfigName string) *v1alpha1.App {
	app := getApp(name, namespace, clusterConfigName)

	if extraConfigName != "" {
		app.Spec.ExtraConfigs = append(app.Spec.ExtraConfigs, v1alpha1.AppExtraConfig{
			Kind:      extraConfigKindConfigMap,
			Name:      extraConfigName,
			Namespace: namespace,
			Priority:  150,
		})
	}

	return app
}

func getTestKubeadmControlPlane(clusterIssuer string) *v1beta1.KubeadmControlPlane {
	spec := v1beta1.KubeadmControlPlaneSpec{
		KubeadmConfigSpec: bootstrapv1beta1.KubeadmConfigSpec{
			ClusterConfiguration: &bootstrapv1beta1.ClusterConfiguration{
				APIServer: bootstrapv1beta1.APIServer{},
			},
		},
	}
	if clusterIssuer != "" {
		spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs = map[string]string{
			key.OIDCIssuerAPIServerExtraArg: clusterIssuer,
		}
	}
	return &v1beta1.KubeadmControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Spec: spec,
	}
}

func getOIDCConfig(clusterIssuer string) *corev1.ConfigMap {
	data, _ := CreateOIDCFlagsConfigMapValues(clusterIssuer)
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.GetClusterOIDCConfigName(clusterName),
			Namespace: namespace,
			Finalizers: []string{
				key.DexOperatorFinalizer,
			},
		},
		Data: data,
	}
}
