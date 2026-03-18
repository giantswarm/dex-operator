package azure

import (
	"strconv"
	"testing"
	"time"

	"github.com/giantswarm/dex-operator/pkg/idp/provider"

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

func TestComputeClaimsUpdatePatch(t *testing.T) {
	testCases := []struct {
		name         string
		claims       models.OptionalClaimsable
		updateNeeded bool
	}{
		{
			name:         "case 0",
			claims:       getClaimsRequestBody(),
			updateNeeded: false,
		},
		{
			name:         "case 1",
			claims:       nil,
			updateNeeded: true,
		},
		{
			name:         "case 2",
			claims:       getTestClaimGroups(),
			updateNeeded: true,
		},
		{
			name:         "case 3",
			claims:       getTestClaimEmail(),
			updateNeeded: true,
		},
		{
			name:         "case 4",
			claims:       getTestClaimEmailGroups(),
			updateNeeded: true,
		},
		{
			name:         "case 4",
			claims:       getTestClaimEmailGroupsComplete(),
			updateNeeded: false,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			app := models.NewApplication()
			app.SetOptionalClaims(tc.claims)
			updateNeeded, _ := computeClaimsUpdatePatch(app)
			if updateNeeded != tc.updateNeeded {
				t.Fatalf("Expected %v, got %v", updateNeeded, tc.updateNeeded)
			}
		})
	}
}

func getTestClaimGroups() models.OptionalClaimsable {
	claims := models.NewOptionalClaims()
	claim := getClaim()
	claims.SetSaml2Token([]models.OptionalClaimable{claim})

	return claims
}

func getTestClaimEmail() models.OptionalClaimsable {
	claims := models.NewOptionalClaims()
	claim := getClaimFromName("email")
	claims.SetAccessToken([]models.OptionalClaimable{claim})

	return claims
}

func getTestClaimEmailGroups() models.OptionalClaimsable {
	claims := models.NewOptionalClaims()
	claim := getClaim()
	emailClaim := getClaimFromName("email")
	claims.SetIdToken([]models.OptionalClaimable{claim, emailClaim})

	return claims
}

func getTestClaimEmailGroupsComplete() models.OptionalClaimsable {
	claims := models.NewOptionalClaims()
	claim := getClaim()
	emailClaim := getClaimFromName("email")
	claims.SetIdToken([]models.OptionalClaimable{claim, emailClaim})
	claims.SetAccessToken([]models.OptionalClaimable{claim})
	claims.SetSaml2Token([]models.OptionalClaimable{claim, emailClaim})

	return claims
}

func TestNewConfig(t *testing.T) {
	testCases := []struct {
		name        string
		credentials provider.ProviderCredential
		log         logr.Logger
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
			s.SetEndDateTime(&testCases[i].expirationDate)
			if secretExpired(s) != tc.expired {
				t.Fatalf("Expected %v, got %v", tc.expired, secretExpired(s))
			}
		})
	}
}

func TestComputeURIUpdatePatch(t *testing.T) {
	testCases := []struct {
		name         string
		URIs         []string
		updateNeeded bool
	}{
		{
			name:         "case 0",
			URIs:         nil,
			updateNeeded: true,
		},
		{
			name:         "case 1",
			URIs:         []string{"hi.io"},
			updateNeeded: true,
		},
		{
			name:         "case 2",
			URIs:         []string{"hello.io"},
			updateNeeded: false,
		},
		{
			name:         "case 3",
			URIs:         []string{"hi.io", "hello.io"},
			updateNeeded: false,
		},
		{
			name:         "case 4",
			URIs:         []string{},
			updateNeeded: true,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			app := models.NewApplication()
			app.SetWeb(getRedirectURIsRequestBody(tc.URIs))
			updateNeeded, _ := computeRedirectURIUpdatePatch(app, provider.GetTestConfig())
			if updateNeeded != tc.updateNeeded {
				t.Fatalf("Expected %v, got %v", updateNeeded, tc.updateNeeded)
			}
		})
	}
}
