package giantswarmsso

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/idp/provider"
)

// getTestCredential returns a minimal valid credential for RFC 8693 token exchange.
// For token exchange, clientID and clientSecret are NOT required.
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
		{
			name: "case 6 - valid without clientID/clientSecret (RFC 8693 token exchange)",
			credentials: provider.ProviderCredential{
				Name:  ProviderName,
				Owner: "giantswarm",
				Credentials: map[string]string{
					IssuerKey:             "https://dex.central.example.com",
					CentralClusterNameKey: "central",
				},
			},
			log:         true,
			expectError: false,
		},
		{
			name: "case 7 - valid with clientID/clientSecret (standard OIDC flow)",
			credentials: provider.ProviderCredential{
				Name:  ProviderName,
				Owner: "giantswarm",
				Credentials: map[string]string{
					IssuerKey:             "https://dex.central.example.com",
					ClientIDKey:           "test-client-id",
					ClientSecretKey:       "test-client-secret",
					CentralClusterNameKey: "central",
				},
			},
			log:         true,
			expectError: false,
		},
		{
			name: "case 8 - issuer with HTTP scheme (not HTTPS)",
			credentials: provider.ProviderCredential{
				Name:  ProviderName,
				Owner: "giantswarm",
				Credentials: map[string]string{
					IssuerKey:             "http://dex.central.example.com",
					ClientIDKey:           "test-client-id",
					ClientSecretKey:       "test-client-secret",
					CentralClusterNameKey: "central",
				},
			},
			log:         true,
			expectError: true,
		},
		{
			name: "case 9 - issuer with invalid URL",
			credentials: provider.ProviderCredential{
				Name:  ProviderName,
				Owner: "giantswarm",
				Credentials: map[string]string{
					IssuerKey:             "not-a-valid-url",
					ClientIDKey:           "test-client-id",
					ClientSecretKey:       "test-client-secret",
					CentralClusterNameKey: "central",
				},
			},
			log:         true,
			expectError: true,
		},
		{
			name: "case 10 - issuer with empty host",
			credentials: provider.ProviderCredential{
				Name:  ProviderName,
				Owner: "giantswarm",
				Credentials: map[string]string{
					IssuerKey:             "https://",
					ClientIDKey:           "test-client-id",
					ClientSecretKey:       "test-client-secret",
					CentralClusterNameKey: "central",
				},
			},
			log:         true,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var log logr.Logger
			if tc.log {
				log = provider.GetTestLogger()
			}
			// When tc.log is false, log remains zero value (logr.Logger{})

			_, err := newConfig(tc.credentials, log)
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

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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
					IssuerKey:             "https://dex.mycentral.example.com",
					CentralClusterNameKey: "mycentral",
				},
			},
			managementClusterName: "mycentral", // matches the configured central cluster
			appConfig:             provider.GetTestConfig(),

			expectEmptyConnector: true,
		},
		{
			name: "case 3 - non-central cluster with central config",
			credentials: provider.ProviderCredential{
				Name:  ProviderName,
				Owner: "giantswarm",
				Credentials: map[string]string{
					IssuerKey:             "https://dex.mycentral.example.com",
					CentralClusterNameKey: "mycentral",
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

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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

func TestConnectorConfigWithoutClientCredentials(t *testing.T) {
	// Test RFC 8693 token exchange mode - no client credentials
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

	// Verify the connector config contains expected values for token exchange
	expectedStrings := []string{
		"issuer: https://dex.mycentral.example.com",
		"insecureEnableGroups: true",
		"redirectURI: https://dex.grizzly.example.com/callback",
		"scopes:",
		"openid",
		"groups",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(app.Connector.Config, expected) {
			t.Errorf("expected connector config to contain %q, got:\n%s", expected, app.Connector.Config)
		}
	}

	// Verify clientID and clientSecret are NOT present
	if strings.Contains(app.Connector.Config, "clientID") {
		t.Errorf("expected connector config to NOT contain clientID for token exchange mode, got:\n%s", app.Connector.Config)
	}
	if strings.Contains(app.Connector.Config, "clientSecret") {
		t.Errorf("expected connector config to NOT contain clientSecret for token exchange mode, got:\n%s", app.Connector.Config)
	}
}

func TestConnectorConfigWithClientCredentials(t *testing.T) {
	// Test standard OIDC flow mode - with client credentials
	credential := provider.ProviderCredential{
		Name:  ProviderName,
		Owner: "giantswarm",
		Credentials: map[string]string{
			IssuerKey:             "https://dex.mycentral.example.com",
			ClientIDKey:           "my-client-id",
			ClientSecretKey:       "my-client-secret",
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

	// Verify the connector config contains all expected values including client credentials
	expectedStrings := []string{
		"issuer: https://dex.mycentral.example.com",
		"clientID: my-client-id",
		"clientSecret: my-client-secret",
		"insecureEnableGroups: true",
		"redirectURI: https://dex.grizzly.example.com/callback",
		"scopes:",
		"openid",
		"groups",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(app.Connector.Config, expected) {
			t.Errorf("expected connector config to contain %q, got:\n%s", expected, app.Connector.Config)
		}
	}
}

func TestValidateIssuerURL(t *testing.T) {
	testCases := []struct {
		name        string
		issuer      string
		expectError bool
	}{
		{
			name:        "valid HTTPS URL",
			issuer:      "https://dex.example.com",
			expectError: false,
		},
		{
			name:        "valid HTTPS URL with path",
			issuer:      "https://dex.example.com/dex",
			expectError: false,
		},
		{
			name:        "valid HTTPS URL with port",
			issuer:      "https://dex.example.com:443",
			expectError: false,
		},
		{
			name:        "HTTP scheme rejected",
			issuer:      "http://dex.example.com",
			expectError: true,
		},
		{
			name:        "empty scheme rejected",
			issuer:      "dex.example.com",
			expectError: true,
		},
		{
			name:        "empty host rejected",
			issuer:      "https://",
			expectError: true,
		},
		{
			name:        "ftp scheme rejected",
			issuer:      "ftp://dex.example.com",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateIssuerURL(tc.issuer)
			if err != nil && !tc.expectError {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && tc.expectError {
				t.Fatalf("expected an error, got success")
			}
		})
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
