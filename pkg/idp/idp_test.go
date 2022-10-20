package idp

import (
	"context"
	"strconv"
	"testing"

	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/idp/provider/mockprovider"

	ctrl "sigs.k8s.io/controller-runtime"
)

func TestCreateProviderApps(t *testing.T) {
	testCases := []struct {
		name      string
		providers []provider.Provider
		appConfig provider.AppConfig
	}{
		{
			name:      "case 0",
			providers: []provider.Provider{getExampleProvider()},
			appConfig: provider.AppConfig{
				RedirectURI: `hello.com`,
				Name:        "hello",
			},
		},
		{
			name: "case 1",
			providers: []provider.Provider{
				getExampleProvider(),
				getExampleProvider()},
			appConfig: provider.AppConfig{
				RedirectURI: `hello.com`,
				Name:        "hello",
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			s := Service{
				providers: tc.providers,
				log:       ctrl.Log.WithName("test"),
			}
			_, err := s.CreateProviderApps(tc.appConfig, context.Background())
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func getExampleProvider() provider.Provider {
	p, _ := mockprovider.New(provider.ProviderCredential{})
	return p
}
