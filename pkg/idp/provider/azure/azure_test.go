package azure

import (
	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/key"
	"strconv"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	ctrl "sigs.k8s.io/controller-runtime"
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
			s := GetSecretCreateRequestBody(tc.config.Name, key.SecretValidityMonths)
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
			a := Azure{
				Log: getTestLogger(),
			}
			updateNeeded, _ := a.computeAppUpdatePatch(getTestConfig(), tc.app, models.NewApplication())
			if updateNeeded != tc.updateNeeded {
				t.Fatalf("Expected %v, got %v", updateNeeded, tc.updateNeeded)
			}
		})
	}
}

func TestSecretExpired(t *testing.T) {
	testCases := []struct {
		name           string
		expirationDate time.Time
		expired        bool
	}{
		{
			name:           "case 0",
			expirationDate: time.Now(),
			expired:        true,
		},
		{
			name:           "case 1",
			expirationDate: time.Now().Add(7 * 24 * time.Hour),
			expired:        true,
		},
		{
			name:           "case 2",
			expirationDate: time.Now().Add(14 * 24 * time.Hour),
			expired:        false,
		},
		{
			name:           "case 3",
			expirationDate: time.Now().Add(-1 * 24 * time.Hour),
			expired:        true,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			s := models.NewPasswordCredential()
			s.SetEndDateTime(&tc.expirationDate)
			if secretExpired(s) != tc.expired {
				t.Fatalf("Expected %v, got %v", tc.expired, secretExpired(s))
			}
		})
	}
}

func getTestConfig() provider.AppConfig {
	return provider.AppConfig{RedirectURI: "hello.io", Name: "test"}
}

func getTestLogger() *logr.Logger {
	l := ctrl.Log.WithName("test")
	return &l
}
