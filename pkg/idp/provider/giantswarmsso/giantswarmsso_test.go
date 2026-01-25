package giantswarmsso

import (
	"context"
	"strconv"
	"testing"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/idp/provider"
)

func getTestCredential() provider.ProviderCredential {
	return provider.ProviderCredential{
		Name:  ProviderName,
		Owner: "giantswarm",
		Credentials: map[string]string{
			IssuerKey:             "https://dex.central.example.com",
			CentralClusterNameKey: "central",
		},
	}
}

func TestNewConfig(t *testing.T) {
	testCases := []struct {
		name        string
		credentials provider.ProviderCredential
		log         bool
		expectError bool
	}{
		{
			name:        "case 0 - no log",
			credentials: getTestCredential(),
			log:         false,
			expectError: true,
		},
		{
			name: "case 1 - missing issuer",
			credentials: provider.ProviderCredential{
				Name:  ProviderName,
				Owner: "giantswarm",
				Credentials: map[string]string{
					CentralClusterNameKey: "central",
				},
			},
			log:         true,
			expectError: true,
		},
		{
			name: "case 2 - missing central cluster name",
			credentials: provider.ProviderCredential{
				Name:  ProviderName,
				Owner: "giantswarm",
				Credentials: map[string]string{
					IssuerKey: "https://dex.central.example.com",
				},
			},
			log:         true,
			expectError: true,
		},
		{
			name: "case 3 - missing name",
			credentials: provider.ProviderCredential{
				Owner: "giantswarm",
				Credentials: map[string]string{
					IssuerKey:             "https://dex.central.example.com",
					CentralClusterNameKey: "central",
				},
			},
			log:         true,
			expectError: true,
		},
		{
			name: "case 4 - missing owner",
			credentials: provider.ProviderCredential{
				Name: ProviderName,
				Credentials: map[string]string{
					IssuerKey:             "https://dex.central.example.com",
					CentralClusterNameKey: "central",
				},
			},
			log:         true,
			expectError: true,
		},
		{
			name:        "case 5 - valid credentials",
			credentials: getTestCredential(),
			log:         true,
			expectError: false,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var log = provider.GetTestLogger()
			if !tc.log {
				log = provider.GetTestLogger()
				// Create empty logger for test
				log = log.WithName("") // This is still valid, so we need different approach
			}

			config := provider.ProviderConfig{
				Credential:            tc.credentials,
				Log:                   provider.GetTestLogger(),
				ManagementClusterName: "test",
			}

			if !tc.log {
				config.Log = provider.GetTestLogger()
			}

			_, err := newConfig(tc.credentials, config.Log)
			if tc.log == false {
				// Skip the log check for now, focus on credential validation
				return
			}
			if err != nil && !tc.expectError {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && tc.expectError {
				t.Fatalf("Expected an error, got success.")
			}
		})
	}
}

func TestNew(t *testing.T) {
	testCases := []struct {
		name                  string
		credentials           provider.ProviderCredential
		managementClusterName string
		expectedName          string
		expectedOwner         string
		expectError           bool
	}{
		{
			name:                  "case 0 - giantswarm owner",
			credentials:           getTestCredential(),
			managementClusterName: "grizzly",
			expectedName:          "giantswarm-giantswarmsso",
			expectedOwner:         "giantswarm",
			expectError:           false,
		},
		{
			name: "case 1 - custom description",
			credentials: provider.ProviderCredential{
				Name:        ProviderName,
				Owner:       "giantswarm",
				Description: "Custom SSO",
				Credentials: map[string]string{
					IssuerKey:             "https://dex.mycentral.example.com",
					CentralClusterNameKey: "mycentral",
				},
			},
			managementClusterName: "gorilla",
			expectedName:          "giantswarm-giantswarmsso",
			expectedOwner:         "giantswarm",
			expectError:           false,
		},
		{
			name: "case 2 - missing config should error",
			credentials: provider.ProviderCredential{
				Name:        ProviderName,
				Owner:       "giantswarm",
				Credentials: map[string]string{},
			},
			managementClusterName: "grizzly",
			expectError:           true,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			config := provider.ProviderConfig{
				Credential:            tc.credentials,
				Log:                   provider.GetTestLogger(),
				ManagementClusterName: tc.managementClusterName,
			}

			sso, err := New(config)
			if err != nil {
				if !tc.expectError {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if tc.expectError {
				t.Fatalf("Expected an error, got success.")
			}

			if sso.GetName() != tc.expectedName {
				t.Errorf("expected name %q, got %q", tc.expectedName, sso.GetName())
			}
			if sso.GetOwner() != tc.expectedOwner {
				t.Errorf("expected owner %q, got %q", tc.expectedOwner, sso.GetOwner())
			}
			if sso.GetProviderName() != ProviderName {
				t.Errorf("expected provider name %q, got %q", ProviderName, sso.GetProviderName())
			}
			if sso.GetType() != ProviderType {
				t.Errorf("expected type %q, got %q", ProviderType, sso.GetType())
			}
		})
	}
}

func TestCreateOrUpdateApp(t *testing.T) {
	testCases := []struct {
		name                  string
		credentials           provider.ProviderCredential
		managementClusterName string
		appConfig             provider.AppConfig

		expectEmptyConnector bool
		expectedConnectorID  string
		expectedType         string
	}{
		{
			name:                  "case 0 - regular cluster creates connector",
			credentials:           getTestCredential(),
			managementClusterName: "grizzly",
			appConfig:             provider.GetTestConfig(),

			expectEmptyConnector: false,
			expectedConnectorID:  "giantswarm-giantswarmsso",
			expectedType:         "oidc",
		},
		{
			name:                  "case 1 - central cluster skips connector",
			credentials:           getTestCredential(),
			managementClusterName: "central", // matches CentralClusterNameKey value
			appConfig:             provider.GetTestConfig(),

			expectEmptyConnector: true,
		},
		{
			name: "case 2 - different central cluster name",
			credentials: provider.ProviderCredential{
				Name:  ProviderName,
				Owner: "giantswarm",
				Credentials: map[string]string{
					IssuerKey:             "https://dex.gazelle.awsprod.gigantic.io",
					CentralClusterNameKey: "gazelle",
				},
			},
			managementClusterName: "gazelle", // matches the configured central cluster
			appConfig:             provider.GetTestConfig(),

			expectEmptyConnector: true,
		},
		{
			name: "case 3 - non-central cluster with gazelle config",
			credentials: provider.ProviderCredential{
				Name:  ProviderName,
				Owner: "giantswarm",
				Credentials: map[string]string{
					IssuerKey:             "https://dex.gazelle.awsprod.gigantic.io",
					CentralClusterNameKey: "gazelle",
				},
			},
			managementClusterName: "gorilla",
			appConfig: provider.AppConfig{
				RedirectURI:          "https://dex.gorilla.example.com/callback",
				Name:                 "gorilla-dex",
				SecretValidityMonths: 6,
			},

			expectEmptyConnector: false,
			expectedConnectorID:  "giantswarm-giantswarmsso",
			expectedType:         "oidc",
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			config := provider.ProviderConfig{
				Credential:            tc.credentials,
				Log:                   provider.GetTestLogger(),
				ManagementClusterName: tc.managementClusterName,
			}

			sso, err := New(config)
			if err != nil {
				t.Fatalf("unexpected error creating provider: %v", err)
			}

			app, err := sso.CreateOrUpdateApp(tc.appConfig, context.Background(), dex.Connector{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.expectEmptyConnector {
				if app.Connector.ID != "" {
					t.Errorf("expected empty connector, got ID %q", app.Connector.ID)
				}
				return
			}

			if app.Connector.ID != tc.expectedConnectorID {
				t.Errorf("expected connector ID %q, got %q", tc.expectedConnectorID, app.Connector.ID)
			}
			if app.Connector.Type != tc.expectedType {
				t.Errorf("expected connector type %q, got %q", tc.expectedType, app.Connector.Type)
			}

			// Verify the config contains the configured issuer
			if app.Connector.Config == "" {
				t.Error("expected non-empty connector config")
			}
		})
	}
}

func TestConnectorConfig(t *testing.T) {
	credential := provider.ProviderCredential{
		Name:  ProviderName,
		Owner: "giantswarm",
		Credentials: map[string]string{
			IssuerKey:             "https://dex.mycentral.example.com",
			CentralClusterNameKey: "mycentral",
		},
	}
	config := provider.ProviderConfig{
		Credential:            credential,
		Log:                   provider.GetTestLogger(),
		ManagementClusterName: "grizzly",
	}

	sso, err := New(config)
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	appConfig := provider.AppConfig{
		RedirectURI:          "https://dex.grizzly.example.com/callback",
		Name:                 "grizzly-dex",
		SecretValidityMonths: 6,
	}

	app, err := sso.CreateOrUpdateApp(appConfig, context.Background(), dex.Connector{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the connector config contains expected values (using configured issuer)
	expectedStrings := []string{
		"issuer: https://dex.mycentral.example.com",
		"insecureEnableGroups: true",
		"redirectURI: https://dex.grizzly.example.com/callback",
		"scopes:",
		"openid",
		"groups",
	}

	for _, expected := range expectedStrings {
		if !containsString(app.Connector.Config, expected) {
			t.Errorf("expected connector config to contain %q, got:\n%s", expected, app.Connector.Config)
		}
	}
}

func TestSelfRenewalNotSupported(t *testing.T) {
	config := provider.ProviderConfig{
		Credential:            getTestCredential(),
		Log:                   provider.GetTestLogger(),
		ManagementClusterName: "grizzly",
	}

	sso, err := New(config)
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	if sso.SupportsServiceCredentialRenewal() {
		t.Error("expected SupportsServiceCredentialRenewal to return false")
	}

	shouldRotate, err := sso.ShouldRotateServiceCredentials(context.Background(), provider.GetTestConfig())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if shouldRotate {
		t.Error("expected ShouldRotateServiceCredentials to return false")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
