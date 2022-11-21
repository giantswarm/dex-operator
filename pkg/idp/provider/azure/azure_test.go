package azure

import (
	"giantswarm/dex-operator/pkg/idp/provider"
	"strconv"
	"testing"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

func TestGetRequestBody(t *testing.T) {
	testCases := []struct {
		name   string
		config provider.AppConfig
	}{
		{
			name:   "case 0",
			config: getTestConfig(),
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			m := getAppCreateRequestBody(tc.config)
			name := m.GetDisplayName()
			if *name != tc.config.Name {
				t.Fatalf("Expected %s, got %v", tc.config.Name, *name)
			}
			uri := m.GetWeb().GetRedirectUris()
			if len(uri) != 1 {
				t.Fatalf("Unexpected number of redirect URIs")
			}
			if uri[0] != tc.config.RedirectURI {
				t.Fatalf("Expected %s, got %v", tc.config.RedirectURI, uri[0])
			}
			s := getSecretCreateRequestBody(tc.config)
			secretName := s.GetPasswordCredential().GetDisplayName()
			if *secretName != tc.config.Name {
				t.Fatalf("Expected %s, got %v", tc.config.Name, *secretName)
			}
		})
	}
}

func TestComputeAppUpdatePatch(t *testing.T) {
	testCases := []struct {
		name         string
		app          models.Applicationable
		updateNeeded bool
	}{
		{
			name:         "case 0",
			app:          getAppCreateRequestBody(getTestConfig()),
			updateNeeded: false,
		},
		{
			name:         "case 1",
			app:          models.NewApplication(),
			updateNeeded: true,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			updateNeeded, _ := computeAppUpdatePatch(getTestConfig(), tc.app, models.NewApplication())
			if updateNeeded != tc.updateNeeded {
				t.Fatalf("Expected %v, got %v", updateNeeded, tc.updateNeeded)
			}
		})
	}
}

func getTestConfig() provider.AppConfig {
	return provider.AppConfig{RedirectURI: "hello.io", Name: "test"}
}
