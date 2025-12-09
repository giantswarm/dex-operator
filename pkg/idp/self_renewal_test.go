package idp

import (
	"context"
	"errors"
	"testing"
	"time"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/dextarget"
	"github.com/giantswarm/dex-operator/pkg/idp/provider"
)

// Test provider implementation
type testSelfRenewalProvider struct {
	name                  string
	providerName          string
	owner                 string
	supportsRenewal       bool
	shouldRotate          bool
	shouldRotateError     error
	rotateCredentials     map[string]string
	rotateError           error
	rotateCallCount       int
	shouldRotateCallCount int
}

var _ provider.Provider = (*testSelfRenewalProvider)(nil)

func (t *testSelfRenewalProvider) CreateOrUpdateApp(config provider.AppConfig, ctx context.Context, connector dex.Connector) (provider.ProviderApp, error) {
	return provider.ProviderApp{}, nil
}

func (t *testSelfRenewalProvider) DeleteApp(name string, ctx context.Context) error {
	return nil
}

func (t *testSelfRenewalProvider) GetCredentialsForAuthenticatedApp(config provider.AppConfig) (map[string]string, error) {
	return map[string]string{}, nil
}

func (t *testSelfRenewalProvider) CleanCredentialsForAuthenticatedApp(config provider.AppConfig) error {
	return nil
}

func (t *testSelfRenewalProvider) DeleteAuthenticatedApp(config provider.AppConfig) error {
	return nil
}

func (t *testSelfRenewalProvider) GetName() string {
	return t.name
}

func (t *testSelfRenewalProvider) GetProviderName() string {
	return t.providerName
}

func (t *testSelfRenewalProvider) GetOwner() string {
	return t.owner
}

func (t *testSelfRenewalProvider) GetType() string {
	return "test"
}

func (t *testSelfRenewalProvider) SupportsServiceCredentialRenewal() bool {
	return t.supportsRenewal
}

func (t *testSelfRenewalProvider) ShouldRotateServiceCredentials(ctx context.Context, config provider.AppConfig) (bool, error) {
	t.shouldRotateCallCount++
	return t.shouldRotate, t.shouldRotateError
}

func (t *testSelfRenewalProvider) RotateServiceCredentials(ctx context.Context, config provider.AppConfig) (map[string]string, error) {
	t.rotateCallCount++
	if t.rotateError != nil {
		return nil, t.rotateError
	}
	return t.rotateCredentials, nil
}

func TestCheckAndRotateServiceCredentials(t *testing.T) {
	testCases := []struct {
		name                   string
		providers              []provider.Provider
		existingSecret         *corev1.Secret
		expectedRotationCalled bool
		expectedSecretUpdated  bool
		expectedError          bool
		expectedAnnotation     bool
		validateCredentials    func(t *testing.T, secret *corev1.Secret)
	}{
		{
			name: "No providers support self-renewal",
			providers: []provider.Provider{
				&testSelfRenewalProvider{
					name:            "test-provider",
					providerName:    "test",
					owner:           "giantswarm",
					supportsRenewal: false,
				},
			},
			existingSecret:         getTestCredentialsSecret(),
			expectedRotationCalled: false,
			expectedSecretUpdated:  false,
		},
		{
			name: "Provider supports renewal but doesn't need it",
			providers: []provider.Provider{
				&testSelfRenewalProvider{
					name:              "test-provider",
					providerName:      "test",
					owner:             "giantswarm",
					supportsRenewal:   true,
					shouldRotate:      false,
					rotateCredentials: map[string]string{"new-key": "new-value"},
				},
			},
			existingSecret:         getTestCredentialsSecret(),
			expectedRotationCalled: false,
			expectedSecretUpdated:  false,
		},
		{
			name: "Provider needs renewal and succeeds",
			providers: []provider.Provider{
				&testSelfRenewalProvider{
					name:              "test-provider",
					providerName:      "test",
					owner:             "giantswarm",
					supportsRenewal:   true,
					shouldRotate:      true,
					rotateCredentials: map[string]string{"client-id": "new-client", "client-secret": "new-secret"},
				},
			},
			existingSecret:         getTestCredentialsSecret(),
			expectedRotationCalled: true,
			expectedSecretUpdated:  true,
			expectedAnnotation:     true,
			validateCredentials: func(t *testing.T, secret *corev1.Secret) {
				var providers []map[string]interface{}
				err := yaml.Unmarshal(secret.Data["credentials"], &providers)
				if err != nil {
					t.Errorf("Failed to unmarshal credentials: %v", err)
					return
				}

				if len(providers) != 1 {
					t.Errorf("Expected exactly 1 provider, got %d", len(providers))
					return
				}

				provider := providers[0]
				if provider["name"] != "test" {
					t.Errorf("Expected provider name 'test', got %v", provider["name"])
					return
				}

				creds, ok := provider["credentials"].(map[interface{}]interface{})
				if !ok {
					t.Errorf("Expected credentials to be a map, got %T", provider["credentials"])
					return
				}

				if creds["client-id"] != "new-client" {
					t.Errorf("Expected client-id 'new-client', got %v", creds["client-id"])
					return
				}

				if creds["client-secret"] != "new-secret" {
					t.Errorf("Expected client-secret 'new-secret', got %v", creds["client-secret"])
					return
				}
			},
		},
		{
			name: "Provider rotation fails",
			providers: []provider.Provider{
				&testSelfRenewalProvider{
					name:            "failing-provider",
					providerName:    "test",
					owner:           "giantswarm",
					supportsRenewal: true,
					shouldRotate:    true,
					rotateError:     errors.New("rotation failed"),
				},
			},
			existingSecret:         getTestCredentialsSecret(),
			expectedRotationCalled: true,  // Rotation IS called, but it fails
			expectedSecretUpdated:  false, // Secret is NOT updated due to failure
		},
		{
			name: "Missing credentials secret",
			providers: []provider.Provider{
				&testSelfRenewalProvider{
					name:              "test-provider",
					providerName:      "test",
					owner:             "giantswarm",
					supportsRenewal:   true,
					shouldRotate:      true,
					rotateCredentials: map[string]string{"client-id": "new-client"},
				},
			},
			existingSecret: nil, // No secret exists
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			scheme := runtime.NewScheme()
			corev1.AddToScheme(scheme)

			fakeClientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			if tc.existingSecret != nil {
				fakeClientBuilder = fakeClientBuilder.WithObjects(tc.existingSecret)
			}
			fakeClient := fakeClientBuilder.Build()
			app := getTestApp()
			target := dextarget.NewAppTarget(ctx, fakeClient, app)

			service := Service{
				Client:                fakeClient,
				log:                   ctrl.Log.WithName("test"),
				target:                target,
				providers:             tc.providers,
				managementClusterName: "test-cluster",
			}

			err := service.CheckAndRotateServiceCredentials(ctx)

			// Check error expectation
			if tc.expectedError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// If we expected an error, skip further validation
			if tc.expectedError {
				return
			}

			// Validate rotation was called exactly the expected number of times
			for _, prov := range tc.providers {
				if testProv, ok := prov.(*testSelfRenewalProvider); ok {
					if testProv.supportsRenewal && testProv.shouldRotate {
						expectedCalls := 1
						if testProv.rotateError != nil {
							// Even if rotation fails, it should still be attempted once
							expectedCalls = 1
						}
						if testProv.rotateCallCount != expectedCalls {
							t.Errorf("Expected rotation to be called exactly %d time(s) for provider %s, got %d",
								expectedCalls, testProv.name, testProv.rotateCallCount)
						}
					} else {
						// If provider doesn't support renewal or shouldn't rotate, no calls expected
						if testProv.rotateCallCount != 0 {
							t.Errorf("Expected rotation NOT to be called for provider %s, got %d calls",
								testProv.name, testProv.rotateCallCount)
						}
					}
				}
			}

			// Check if secret was updated
			if tc.expectedSecretUpdated || tc.expectedAnnotation {
				updatedSecret := &corev1.Secret{}
				err := fakeClient.Get(ctx, types.NamespacedName{
					Name:      CredentialsSecretName,
					Namespace: "example",
				}, updatedSecret)
				if err != nil {
					t.Errorf("Failed to get updated secret: %v", err)
					return
				}

				// Check annotation
				if tc.expectedAnnotation {
					if updatedSecret.Annotations == nil || updatedSecret.Annotations[SelfRenewalAnnotation] == "" {
						t.Errorf("Expected self-renewal annotation to be set")
					}
				}

				// Run custom validation if provided
				if tc.validateCredentials != nil {
					tc.validateCredentials(t, updatedSecret)
				}
			}
		})
	}
}

func TestUpdateCredentialsSecret(t *testing.T) {
	testCases := []struct {
		name               string
		existingSecret     *corev1.Secret
		updates            []ProviderCredentialUpdate
		expectedError      bool
		expectedAnnotation bool
	}{
		{
			name:           "Single provider update",
			existingSecret: getTestCredentialsSecret(),
			updates: []ProviderCredentialUpdate{
				{
					ProviderName: "test",
					Credentials: map[string]string{
						"client-id":     "updated-client",
						"client-secret": "updated-secret",
					},
				},
			},
			expectedAnnotation: true,
		},
		{
			name:           "Provider not found in credentials",
			existingSecret: getTestCredentialsSecret(),
			updates: []ProviderCredentialUpdate{
				{
					ProviderName: "nonexistent",
					Credentials: map[string]string{
						"some-key": "some-value",
					},
				},
			},
			expectedError: true,
		},
		{
			name:           "No credentials data in secret",
			existingSecret: getSecretWithoutCredentialsData(),
			updates: []ProviderCredentialUpdate{
				{
					ProviderName: "test",
					Credentials: map[string]string{
						"client-id": "new-client",
					},
				},
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			scheme := runtime.NewScheme()
			corev1.AddToScheme(scheme)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.existingSecret).
				Build()
			app := getTestApp()
			target := dextarget.NewAppTarget(ctx, fakeClient, app)

			service := Service{
				Client: fakeClient,
				log:    ctrl.Log.WithName("test"),
				target: target,
			}

			err := service.updateCredentialsSecret(ctx, tc.updates)

			// Check error expectation
			if tc.expectedError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// If we expected an error, skip further validation
			if tc.expectedError {
				return
			}

			// Validate the secret was updated correctly
			updatedSecret := &corev1.Secret{}
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      CredentialsSecretName,
				Namespace: "example",
			}, updatedSecret)
			if err != nil {
				t.Errorf("Failed to get updated secret: %v", err)
				return
			}

			// Check annotation
			if tc.expectedAnnotation {
				if updatedSecret.Annotations == nil || updatedSecret.Annotations[SelfRenewalAnnotation] == "" {
					t.Errorf("Expected self-renewal annotation to be set")
				}
			}
		})
	}
}

func TestAddSelfRenewalAnnotation(t *testing.T) {
	service := &Service{
		log: ctrl.Log.WithName("test"),
	}

	testCases := []struct {
		name           string
		secret         *corev1.Secret
		expectedResult func(*corev1.Secret) bool
	}{
		{
			name: "Secret with no annotations",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-secret",
				},
			},
			expectedResult: func(s *corev1.Secret) bool {
				return s.Annotations != nil && s.Annotations[SelfRenewalAnnotation] != ""
			},
		},
		{
			name: "Secret with existing annotations",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-secret",
					Annotations: map[string]string{
						"existing-annotation": "existing-value",
					},
				},
			},
			expectedResult: func(s *corev1.Secret) bool {
				return s.Annotations != nil &&
					s.Annotations[SelfRenewalAnnotation] != "" &&
					s.Annotations["existing-annotation"] == "existing-value"
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service.addSelfRenewalAnnotation(tc.secret)

			if !tc.expectedResult(tc.secret) {
				t.Errorf("Annotation was not added correctly")
			}

			// Verify the annotation value is a valid RFC3339 timestamp
			timestamp := tc.secret.Annotations[SelfRenewalAnnotation]
			if _, err := time.Parse(time.RFC3339, timestamp); err != nil {
				t.Errorf("Invalid timestamp format in annotation: %s", timestamp)
			}
		})
	}
}

// Helper functions for tests

func getTestApp() *v1alpha1.App {
	return &v1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "example",
		},
	}
}

func getTestCredentialsSecret() *corev1.Secret {
	credentialsYAML := `- name: test
  owner: giantswarm
  credentials:
    client-id: original-client
    client-secret: original-secret
`
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CredentialsSecretName,
			Namespace: "example",
		},
		Data: map[string][]byte{
			"credentials": []byte(credentialsYAML),
		},
	}
}

func getSecretWithoutCredentialsData() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CredentialsSecretName,
			Namespace: "example",
		},
		Data: map[string][]byte{
			"other-data": []byte("some data"),
		},
	}
}
