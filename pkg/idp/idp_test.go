package idp

import (
	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/idp/provider/mockprovider"
	"strconv"
	"testing"
)

func TestCreateProviderApps(t *testing.T) {
	testCases := []struct {
		name      string
		providers []provider.Provider
		appConfig provider.AppConfig
	}{
		{
			name:      "case 0",
			providers: []provider.Provider{&mockprovider.MockProvider{Name: mockprovider.ProviderName}},
			appConfig: provider.AppConfig{
				RedirectURI: `hello.com`,
				Name:        "hello",
			},
		},
		{
			name: "case 1",
			providers: []provider.Provider{
				&mockprovider.MockProvider{Name: mockprovider.ProviderName},
				&mockprovider.MockProvider{Name: mockprovider.ProviderName}},
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
			}
			_, err := s.CreateProviderApps(tc.appConfig)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
