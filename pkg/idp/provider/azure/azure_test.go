package azure

import (
	"giantswarm/dex-operator/pkg/idp/provider"
	"strconv"
	"testing"

	"github.com/go-logr/logr"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

func TestGetRequestBody(t *testing.T) {
	testCases := []struct {
		name   string
		config provider.AppConfig
	}{
		{
			name:   "case 0",
			config: provider.GetTestConfig(),
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
			s := GetSecretCreateRequestBody(tc.config)
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
			app:          getAppCreateRequestBody(provider.GetTestConfig()),
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
			a := Azure{
				Log: provider.GetTestLogger(),
			}
			updateNeeded, _ := a.computeAppUpdatePatch(provider.GetTestConfig(), tc.app, models.NewApplication())
			if updateNeeded != tc.updateNeeded {
				t.Fatalf("Expected %v, got %v", updateNeeded, tc.updateNeeded)
			}
		})
	}
}

func TestNewConfig(t *testing.T) {
	testCases := []struct {
		name        string
		credentials provider.ProviderCredential
		log         *logr.Logger
		expectError bool
	}{
		{
			name:        "case 0",
			expectError: true,
		},
		{
			name:        "case 1",
			credentials: provider.GetTestCredential(),
			log:         provider.GetTestLogger(),
			expectError: true,
		},
		{
			name: "case 2",
			credentials: provider.ProviderCredential{
				Name:  "name",
				Owner: "test",
				Credentials: map[string]string{
					ClientIDKey:     "abc",
					ClientSecretKey: "xyz",
					TenantIDKey:     "123",
				},
			},
			log:         provider.GetTestLogger(),
			expectError: false,
		},
		{
			name: "case 3",
			credentials: provider.ProviderCredential{
				Name:  "name",
				Owner: "test",
				Credentials: map[string]string{
					ClientIDKey:     "abc",
					ClientSecretKey: "xyz",
					TenantIDKey:     "123",
				},
			},
			expectError: true,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := newAzureConfig(tc.credentials, tc.log)
			if err != nil && !tc.expectError {
				t.Fatal(err)
			}
			if err == nil && tc.expectError {
				t.Fatalf("Expected an error, got success.")
			}
		})
	}
}
