package idp

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/idp/provider"
	"github.com/giantswarm/dex-operator/pkg/idp/provider/mockprovider"
	"github.com/giantswarm/dex-operator/pkg/key"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestCreateProviderApps(t *testing.T) {
	testCases := []struct {
		name        string
		providers   []provider.Provider
		appConfig   provider.AppConfig
		expectError bool
	}{
		{
			name:      "case 0",
			providers: []provider.Provider{getExampleProvider(key.OwnerGiantswarm)},
			appConfig: provider.GetTestConfig(),
		},
		{
			name: "case 1",
			providers: []provider.Provider{
				getExampleProvider(key.OwnerGiantswarm),
				getExampleProvider(key.OwnerCustomer)},
			appConfig: provider.GetTestConfig(),
		},
		{
			name: "case 2",
			providers: []provider.Provider{
				getExampleProvider(key.OwnerGiantswarm),
				getExampleProvider("somethingelse")},
			appConfig:   provider.GetTestConfig(),
			expectError: true,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			s := Service{
				providers: tc.providers,
				log:       ctrl.Log.WithName("test"),
				app:       getExampleApp(),
			}
			_, err := s.CreateOrUpdateProviderApps(tc.appConfig, context.Background(), map[string]dex.Connector{})
			if err != nil && !tc.expectError {
				t.Fatal(err)
			}
			if err == nil && tc.expectError {
				t.Fatalf("Expected an error, got success.")
			}
		})
	}
}

func TestGetBaseDomain(t *testing.T) {
	testCases := []struct {
		name           string
		data           map[string]string
		expectedDomain string
	}{
		{
			name: "case 0",
			data: map[string]string{
				key.ValuesConfigMapKey: `
					something: "12"
					baseDomain: hello.io
					somethingelse: "false"
					object:
					  yes: no
					`,
			},
			expectedDomain: "hello.io",
		},
		{
			name: "case 1",
			data: map[string]string{
				key.ValuesConfigMapKey: `
					something: "12"
					somethingelse: "false"
					object:
					  yes: no
					baseDomain: hi.goodday.hello.io
					`,
			},
			expectedDomain: "hi.goodday.hello.io",
		},
		{
			name: "case 2",
			data: map[string]string{
				key.ValuesConfigMapKey: `
					something: "12"
					somethingelse: "false"
					object:
					  yes: no
					`,
			},
			expectedDomain: "",
		},
		{
			name: "case 3",
			data: map[string]string{
				key.ValuesConfigMapKey: `
				baseDomain: hi.goodday.hello.io
					something: "12"
					somethingelse: "false"
					object:
					  yes: no
					  baseDomain: no.goodday.hello.io
					`,
			},
			expectedDomain: "hi.goodday.hello.io",
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			cm := &corev1.ConfigMap{
				Data: tc.data,
			}
			baseDomain := getBaseDomainFromClusterValues(cm)
			if baseDomain != tc.expectedDomain {
				t.Fatalf("Expected %v to be equal to %v", baseDomain, tc.expectedDomain)
			}
		})
	}
}

func TestUserConfigMap(t *testing.T) {
	testCases := []struct {
		name           string
		app            *v1alpha1.App
		expectedResult bool
	}{
		{
			name:           "case 0",
			app:            getExampleApp(),
			expectedResult: false,
		},
		{
			name: "case 0",
			app: &v1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "example",
				},
				Spec: v1alpha1.AppSpec{
					UserConfig: v1alpha1.AppSpecUserConfig{},
				},
			},
			expectedResult: false,
		},
		{
			name: "case 1",
			app: &v1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "example",
				},
				Spec: v1alpha1.AppSpec{
					UserConfig: v1alpha1.AppSpecUserConfig{
						ConfigMap: v1alpha1.AppSpecUserConfigConfigMap{},
					},
				},
			},
			expectedResult: false,
		},
		{
			name: "case 2",
			app: &v1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "example",
				},
				Spec: v1alpha1.AppSpec{
					UserConfig: v1alpha1.AppSpecUserConfig{
						ConfigMap: v1alpha1.AppSpecUserConfigConfigMap{
							Name:      "test",
							Namespace: "test",
						},
					},
				},
			},
			expectedResult: true,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			if userConfigMapPresent(tc.app) != tc.expectedResult {
				t.Fatalf("expected result to be %v", tc.expectedResult)
			}
		})
	}
}

func TestUserConnectors(t *testing.T) {
	testCases := []struct {
		name           string
		data           map[string]string
		expectedResult bool
	}{
		{
			name: "case 0",
			data: map[string]string{
				key.ValuesConfigMapKey: `
					something: "12"
					baseDomain: hello.io
					somethingelse: "false"
					the:
					  big:
					    connectors:
						- id: a
						  name: b
					object:
					  yes: no
					`,
			},
			expectedResult: true,
		},
		{
			name: "case 1",
			data: map[string]string{
				key.ValuesConfigMapKey: `
					something: "12"
					baseDomain: hello.io
					somethingelse: "false"
					object:
					  yes: no
					`,
			},
			expectedResult: false,
		},
		{
			name: "case 2",
			data: map[string]string{
				key.ValuesConfigMapKey: `
					something: "12"
					baseDomain: hello.io
					somethingelse: "false"
					the:
					  small:
					    connectors: []
					object:
					  yes: no
					`,
			},
			expectedResult: true,
		},
		{
			name: "case 3",
			data: map[string]string{
				key.ValuesConfigMapKey: `
					something: "12"
					baseDomain: hello.io
					somethingelse: "false"
					the:
					  small:
					    connectors: []
					  big:
					    connectors: []
					object:
					  yes: no
					`,
			},
			expectedResult: true,
		},
		{
			name: "case 4",
			data: map[string]string{
				key.ValuesConfigMapKey: `
					something: "12"
					baseDomain: hello.io
					somethingelse: "false"
					the:
					  big:
					    connectors: null
					object:
					  yes: no
					`,
			},
			expectedResult: true,
		},
		{
			name: "case 5",
			data: map[string]string{
				key.ValuesConfigMapKey: `
					something: "12"
					baseDomain: hello.io
					somethingelse: "false"
					the:
					  big:
					    connectors:
					object:
					  yes: no
					`,
			},
			expectedResult: true,
		},
		{
			name: "case 6",
			data: map[string]string{
				key.ValuesConfigMapKey: `
					something: "12"
					baseDomain: hello.io
					somethingelse: "false"
					the:
					  small:
					    connectors: []
					  big:
					    connectors:
					    - id: a
						  name: b
					object:
					  yes: no
					`,
			},
			expectedResult: true,
		},
		{
			name: "case 7",
			data: map[string]string{
				key.ValuesConfigMapKey: `
					something: "12"
					baseDomain: hello.connectors.io
					somethingelse: "false"
					object:
					  yes: no
					`,
			},
			expectedResult: false,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			cm := &corev1.ConfigMap{
				Data: tc.data,
			}
			if connectorsDefinedInUserConfigMap(cm) != tc.expectedResult {
				t.Fatalf("Expected %v to be equal to %v", connectorsDefinedInUserConfigMap(cm), tc.expectedResult)
			}
		})
	}
}

func TestRemoveExtraConfig(t *testing.T) {
	testCases := []struct {
		name           string
		configBefore   []v1alpha1.AppExtraConfig
		configAfter    []v1alpha1.AppExtraConfig
		configToRemove v1alpha1.AppExtraConfig
	}{
		{
			name:           "case 0",
			configBefore:   nil,
			configAfter:    nil,
			configToRemove: GetVintageDexSecretConfig("test"),
		},
		{
			name: "case 2",
			configBefore: []v1alpha1.AppExtraConfig{
				GetVintageDexSecretConfig("test2"),
			},
			configAfter: []v1alpha1.AppExtraConfig{
				GetVintageDexSecretConfig("test2"),
			},
			configToRemove: GetVintageDexSecretConfig("test"),
		},
		{
			name: "case 2",
			configBefore: []v1alpha1.AppExtraConfig{
				GetVintageDexSecretConfig("test"),
			},
			configAfter:    []v1alpha1.AppExtraConfig{},
			configToRemove: GetVintageDexSecretConfig("test"),
		},
		{
			name: "case 3",
			configBefore: []v1alpha1.AppExtraConfig{
				GetVintageDexSecretConfig("test"),
				GetVintageDexSecretConfig("test2"),
			},
			configAfter: []v1alpha1.AppExtraConfig{
				GetVintageDexSecretConfig("test2"),
			},
			configToRemove: GetVintageDexSecretConfig("test"),
		},
		{
			name:           "case 3",
			configBefore:   []v1alpha1.AppExtraConfig{},
			configAfter:    []v1alpha1.AppExtraConfig{},
			configToRemove: GetVintageDexSecretConfig("test"),
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			if !reflect.DeepEqual(removeExtraConfig(tc.configBefore, tc.configToRemove), tc.configAfter) {
				t.Fatalf("expected result to be %v", tc.configAfter)
			}
		})
	}
}

func TestGetOldConnectorsFromSecret(t *testing.T) {
	testCases := []struct {
		name               string
		providers          []provider.Provider
		expectedConnectors []string
	}{
		{
			// Nothing changed
			name: "case 0",
			providers: []provider.Provider{
				getExampleProvider(key.OwnerGiantswarm),
				getExampleProvider(key.OwnerCustomer)},
			expectedConnectors: []string{"giantswarm-mock", "customer-mock"},
		},
		{
			// Add connector
			name: "case 1",
			providers: []provider.Provider{
				getExampleProvider(key.OwnerGiantswarm),
				getExampleProvider(key.OwnerCustomer)},
			expectedConnectors: []string{"giantswarm-mock", "customer-mock"},
		},
		{
			// Remove connector
			name: "case 2",
			providers: []provider.Provider{
				getExampleProvider(key.OwnerCustomer)},
			expectedConnectors: []string{"customer-mock"},
		},
		{
			// Empty first
			name: "case 3",
			providers: []provider.Provider{
				getExampleProvider(key.OwnerCustomer)},
			expectedConnectors: []string{"customer-mock"},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			appConfig := provider.AppConfig{
				RedirectURI: `hello.com`,
				Name:        "hello",
			}
			ctx := context.Background()

			//Initial reconcile, creating apps
			s := Service{
				providers: tc.providers,
				log:       ctrl.Log.WithName("test"),
				app:       getExampleApp(),
			}
			config, err := s.CreateOrUpdateProviderApps(appConfig, ctx, map[string]dex.Connector{})
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			secret := GetDefaultDexConfigSecret("example", "test")
			secret.Data["default"] = data
			connectors, err := getConnectorsFromSecret(secret)
			if err != nil {
				t.Fatal(err)
			}

			//Check configuration
			if len(connectors) != len(tc.expectedConnectors) {
				t.Fatalf("Expected %v connectors, got %v", len(tc.expectedConnectors), len(connectors))
			}
			for _, c := range tc.expectedConnectors {
				if _, exists := connectors[c]; !exists {
					t.Fatalf("Expected %v connector to exist.", c)
				}
			}
		})
	}
}

func TestSecretDataNeedsUpdate(t *testing.T) {
	testCases := []struct {
		name         string
		oldConfig    dex.DexConfig
		newConfig    dex.DexConfig
		updateNeeded bool
	}{
		{
			name: "case 0: No changes",
			oldConfig: dex.DexConfig{
				Oidc: dex.DexOidc{
					Customer: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "first"},
						},
					},
					Giantswarm: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "second"},
						},
					},
				},
			},
			newConfig: dex.DexConfig{
				Oidc: dex.DexOidc{
					Customer: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "first"},
						},
					},
					Giantswarm: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "second"},
						},
					},
				},
			},
			updateNeeded: false,
		},
		{
			name: "case 1: New connector",
			oldConfig: dex.DexConfig{
				Oidc: dex.DexOidc{
					Customer: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "first"},
						},
					},
					Giantswarm: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "second"},
						},
					},
				},
			},
			newConfig: dex.DexConfig{
				Oidc: dex.DexOidc{
					Customer: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "first"},
						},
					},
					Giantswarm: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "second"},
							{ID: "third"},
						},
					},
				},
			},
			updateNeeded: true,
		},
		{
			name: "case 2: Connector removed",
			oldConfig: dex.DexConfig{
				Oidc: dex.DexOidc{
					Customer: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "first"},
						},
					},
					Giantswarm: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "second"},
							{ID: "third"},
						},
					},
				},
			},
			newConfig: dex.DexConfig{
				Oidc: dex.DexOidc{
					Customer: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "first"},
						},
					},
					Giantswarm: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "second"},
						},
					},
				},
			},
			updateNeeded: true,
		},
		{
			name: "case 3: Updated config",
			oldConfig: dex.DexConfig{
				Oidc: dex.DexOidc{
					Customer: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "first"},
						},
					},
					Giantswarm: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "second"},
						},
					},
				},
			},
			newConfig: dex.DexConfig{
				Oidc: dex.DexOidc{
					Customer: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "first"},
						},
					},
					Giantswarm: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "second", Config: "something"},
						},
					},
				},
			},
			updateNeeded: true,
		},
		{
			name: "case 4: Updated various things",
			oldConfig: dex.DexConfig{
				Oidc: dex.DexOidc{
					Customer: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "first"},
						},
					},
					Giantswarm: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "second", Config: "something"},
							{ID: "fourth", Config: "somethingelse"},
						},
					},
				},
			},
			newConfig: dex.DexConfig{
				Oidc: dex.DexOidc{
					Customer: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "first", Config: "something"},
							{ID: "third"},
						},
					},
					Giantswarm: &dex.DexOidcOwner{
						Connectors: []dex.Connector{
							{ID: "fourth", Config: "something"},
						},
					},
				},
			},
			updateNeeded: true,
		},
		{
			name:         "case 5: Empty case",
			oldConfig:    dex.DexConfig{},
			newConfig:    dex.DexConfig{},
			updateNeeded: false,
		},
		{
			name: "case 6: Update triggering empty case 1",
			oldConfig: dex.DexConfig{
				Oidc: dex.DexOidc{
					Customer: &dex.DexOidcOwner{},
				},
			},
			newConfig:    dex.DexConfig{},
			updateNeeded: true,
		},
		{
			name:      "case 7: Update triggering empty case 2",
			oldConfig: dex.DexConfig{},
			newConfig: dex.DexConfig{
				Oidc: dex.DexOidc{
					Giantswarm: &dex.DexOidcOwner{},
				},
			},
			updateNeeded: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := Service{
				log: ctrl.Log.WithName("test"),
			}
			updateNeeded := s.secretDataNeedsUpdate(tc.oldConfig, tc.newConfig)
			if updateNeeded != tc.updateNeeded {
				t.Fatalf("Expected %v, got %v", updateNeeded, tc.updateNeeded)
			}
		})
	}
}

func TestGetAppConfig(t *testing.T) {
	testCases := []struct {
		name                           string
		managementClusterName          string
		managementClusterBaseDomain    string
		managementClusterIssuerAddress string
		app                            *v1alpha1.App
		clusterValuesConfigMap         *corev1.ConfigMap
		expectedAppConfig              provider.AppConfig
	}{
		{
			name:                           "case 0: Get issuer URL from cluster config values",
			managementClusterName:          "testcluster",
			managementClusterBaseDomain:    "base.domain.io",
			managementClusterIssuerAddress: "issuer.cluster.base.domain.io",
			app:                            getExampleApp(),
			clusterValuesConfigMap:         getClusterValuesConfigMap("baseDomain: wc.cluster.domain.io"),
			expectedAppConfig: provider.AppConfig{
				Name:                 "testcluster-example-test",
				RedirectURI:          "https://dex.wc.cluster.domain.io/callback",
				IdentifierURI:        "https://dex.giantswarm.io/testcluster-example-test",
				SecretValidityMonths: key.SecretValidityMonths,
			},
		},
		{
			name:                           "case 1: Get issuer URL from management cluster issuer URL property",
			managementClusterName:          "testcluster",
			managementClusterBaseDomain:    "base.domain.io",
			managementClusterIssuerAddress: "issuer.cluster.domain.io",
			app:                            getExampleApp(),
			expectedAppConfig: provider.AppConfig{
				Name:                 "testcluster-example-test",
				RedirectURI:          "https://issuer.cluster.domain.io/callback",
				IdentifierURI:        "https://dex.giantswarm.io/testcluster-example-test",
				SecretValidityMonths: key.SecretValidityMonths,
			},
		},
		{
			name:                        "case 2: Get issuer URL from management cluster base domain",
			managementClusterName:       "testcluster",
			managementClusterBaseDomain: "base.domain.io",
			app:                         getExampleApp(),
			expectedAppConfig: provider.AppConfig{
				Name:                 "testcluster-example-test",
				RedirectURI:          "https://dex.g8s.base.domain.io/callback",
				IdentifierURI:        "https://dex.giantswarm.io/testcluster-example-test",
				SecretValidityMonths: key.SecretValidityMonths,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			ctx := context.Background()

			fakeClientBuilder := fake.NewClientBuilder()
			if tc.clusterValuesConfigMap != nil {
				tc.app.Spec = v1alpha1.AppSpec{
					Config: v1alpha1.AppSpecConfig{
						ConfigMap: v1alpha1.AppSpecConfigConfigMap{
							Name:      tc.clusterValuesConfigMap.Name,
							Namespace: tc.clusterValuesConfigMap.Namespace,
						},
					},
				}
				fakeClientBuilder.WithObjects(tc.clusterValuesConfigMap)
			}

			service := Service{
				Client:                         fakeClientBuilder.Build(),
				log:                            ctrl.Log.WithName("test"),
				app:                            tc.app,
				managementClusterName:          tc.managementClusterName,
				managementClusterBaseDomain:    tc.managementClusterBaseDomain,
				managementClusterIssuerAddress: tc.managementClusterIssuerAddress,
			}

			appConfig, err := service.GetAppConfig(ctx)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(appConfig, tc.expectedAppConfig) {
				t.Fatalf("Expacted %v, got %v", tc.expectedAppConfig, appConfig)
			}
		})
	}
}

func getExampleProvider(owner string) provider.Provider {
	p, _ := mockprovider.New(provider.ProviderConfig{
		Credential: provider.ProviderCredential{Owner: owner},
	})
	return p
}

func getExampleApp() *v1alpha1.App {
	return &v1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "example",
		},
	}
}

func getClusterValuesConfigMap(clusterValues string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("test-%s", key.ClusterValuesConfigmapSuffix),
			Namespace: "example",
		},
		Data: map[string]string{
			key.ValuesConfigMapKey: clusterValues,
		},
	}
}
