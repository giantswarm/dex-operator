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

	corev1 "k8s.io/api/core/v1"
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
		originalProviders  []provider.Provider
		newProviders       []provider.Provider
		expectedConnectors []string
	}{
		{
			// Nothing changed
			name: "case 0",
			originalProviders: []provider.Provider{
				getExampleProvider(key.OwnerGiantswarm),
				getExampleProvider(key.OwnerCustomer)},
			newProviders: []provider.Provider{
				getExampleProvider(key.OwnerGiantswarm),
				getExampleProvider(key.OwnerCustomer)},
			expectedConnectors: []string{"giantswarm-mock", "customer-mock"},
		},
		{
			// Add connector
			name: "case 1",
			originalProviders: []provider.Provider{
				getExampleProvider(key.OwnerCustomer)},
			newProviders: []provider.Provider{
				getExampleProvider(key.OwnerGiantswarm),
				getExampleProvider(key.OwnerCustomer)},
			expectedConnectors: []string{"giantswarm-mock", "customer-mock"},
		},
		{
			// Remove connector
			name: "case 2",
			originalProviders: []provider.Provider{
				getExampleProvider(key.OwnerGiantswarm),
				getExampleProvider(key.OwnerCustomer)},
			newProviders: []provider.Provider{
				getExampleProvider(key.OwnerCustomer)},
			expectedConnectors: []string{"customer-mock"},
		},
		{
			// Empty first
			name:              "case 3",
			originalProviders: []provider.Provider{},
			newProviders: []provider.Provider{
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
				providers: tc.originalProviders,
				log:       ctrl.Log.WithName("test"),
			}
			config, err := s.CreateOrUpdateProviderApps(appConfig, ctx, map[string]dex.Connector{})
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			secret := s.GetDefaultDexConfigSecret("example", "test", data)
			connectors, err := getOldConnectorsFromSecret(secret)
			if err != nil {
				t.Fatal(err)
			}

			//Subsequent reconcile, updating apps
			s = Service{
				providers: tc.newProviders,
				log:       ctrl.Log.WithName("test"),
			}
			config, err = s.CreateOrUpdateProviderApps(appConfig, ctx, connectors)
			if err != nil {
				t.Fatal(err)
			}
			data, err = json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			secret = s.GetDefaultDexConfigSecret("example", "test", data)
			connectors, err = getOldConnectorsFromSecret(secret)
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

func getExampleProvider(owner string) provider.Provider {
	p, _ := mockprovider.New(provider.ProviderCredential{Owner: owner})
	return p
}
