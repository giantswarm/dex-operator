package idp

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"giantswarm/dex-operator/pkg/dex"
	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/idp/provider/mockprovider"
	"giantswarm/dex-operator/pkg/key"

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
			appConfig: provider.AppConfig{
				RedirectURI: `hello.com`,
				Name:        "hello",
			},
		},
		{
			name: "case 1",
			providers: []provider.Provider{
				getExampleProvider(key.OwnerGiantswarm),
				getExampleProvider(key.OwnerCustomer)},
			appConfig: provider.AppConfig{
				RedirectURI: `hello.com`,
				Name:        "hello",
			},
		},
		{
			name: "case 2",
			providers: []provider.Provider{
				getExampleProvider(key.OwnerGiantswarm),
				getExampleProvider("somethingelse")},
			appConfig: provider.AppConfig{
				RedirectURI: `hello.com`,
				Name:        "hello",
			},
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
				key.ClusterValuesConfigMapKey: `
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
				key.ClusterValuesConfigMapKey: `
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
				key.ClusterValuesConfigMapKey: `
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
				key.ClusterValuesConfigMapKey: `
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
			secret := s.GetDefaultDexConfigSecret("example", "test")
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

func TestConnectorsNeedUpdate(t *testing.T) {
	testCases := []struct {
		name          string
		oldConnectors map[string]dex.Connector
		newConnectors map[string]dex.Connector
		updateNeeded  bool
	}{
		{
			// nothing changed
			name: "case 0",
			oldConnectors: map[string]dex.Connector{
				"first":  {},
				"second": {},
			},
			newConnectors: map[string]dex.Connector{
				"first":  {},
				"second": {},
			},
			updateNeeded: false,
		},
		{
			// new connector
			name: "case 1",
			oldConnectors: map[string]dex.Connector{
				"first":  {},
				"second": {},
			},
			newConnectors: map[string]dex.Connector{
				"first":  {},
				"second": {},
				"third":  {},
			},
			updateNeeded: true,
		},
		{
			// connector removed
			name: "case 2",
			oldConnectors: map[string]dex.Connector{
				"first":  {},
				"second": {},
				"third":  {},
			},
			newConnectors: map[string]dex.Connector{
				"first":  {},
				"second": {},
			},
			updateNeeded: true,
		},
		{
			// updated config
			name: "case 3",
			oldConnectors: map[string]dex.Connector{
				"first":  {},
				"second": {},
			},
			newConnectors: map[string]dex.Connector{
				"first":  {},
				"second": {Config: "something"},
			},
			updateNeeded: true,
		},
		{
			// updated various things
			name: "case 4",
			oldConnectors: map[string]dex.Connector{
				"first":  {},
				"second": {Config: "something"},
				"fourth": {Name: "somethingelse"},
			},
			newConnectors: map[string]dex.Connector{
				"first":  {Name: "something"},
				"third":  {},
				"fourth": {Config: "something"},
			},
			updateNeeded: true,
		},
		{
			// empty case
			name:          "case 4",
			oldConnectors: map[string]dex.Connector{},
			newConnectors: map[string]dex.Connector{},
			updateNeeded:  false,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			s := Service{
				log: ctrl.Log.WithName("test"),
			}
			updateNeeded := s.connectorsNeedUpdate(tc.oldConnectors, tc.newConnectors)
			if updateNeeded != tc.updateNeeded {
				t.Fatalf("Expected %v, got %v", updateNeeded, tc.updateNeeded)
			}
		})
	}
}

func getExampleProvider(owner string) provider.Provider {
	p, _ := mockprovider.New(provider.ProviderCredential{Owner: owner})
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
