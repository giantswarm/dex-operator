package github

import (
	"giantswarm/dex-operator/pkg/idp/provider"
	"strconv"
	"testing"

	"github.com/go-logr/logr"
)

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
					OrganizationKey:   "org",
					TeamKey:           "team",
					AppIDKey:          "123",
					InstallationIDKey: "1200",
					PrivateKeyKey:     "abc",
					AppSecretKey:      "def",
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
					OrganizationKey:   "org",
					TeamKey:           "team",
					AppIDKey:          "abc",
					InstallationIDKey: "1200",
					PrivateKeyKey:     "abc",
					AppSecretKey:      "def",
				},
			},
			log:         provider.GetTestLogger(),
			expectError: true,
		},
		{
			name: "case 4",
			credentials: provider.ProviderCredential{
				Name:  "name",
				Owner: "test",
				Credentials: map[string]string{
					OrganizationKey:   "org",
					TeamKey:           "team",
					AppIDKey:          "123",
					InstallationIDKey: "xyz",
					PrivateKeyKey:     "abc",
					AppSecretKey:      "def",
				},
			},
			log:         provider.GetTestLogger(),
			expectError: true,
		},
		{
			name: "case 2",
			credentials: provider.ProviderCredential{
				Name:  "name",
				Owner: "test",
				Credentials: map[string]string{
					OrganizationKey:   "org",
					TeamKey:           "team",
					AppIDKey:          "123",
					InstallationIDKey: "1200",
					PrivateKeyKey:     "abc",
					AppSecretKey:      "def",
				},
			},
			expectError: true,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := newGithubConfig(tc.credentials, tc.log)
			if err != nil && !tc.expectError {
				t.Fatal(err)
			}
			if err == nil && tc.expectError {
				t.Fatalf("Expected an error, got success.")
			}
		})
	}
}
