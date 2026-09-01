package github

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/giantswarm/dex-operator/pkg/idp/provider"
	"github.com/giantswarm/dex-operator/pkg/yaml"

	githubconnector "github.com/dexidp/dex/connector/github"
	"github.com/go-logr/logr"
)

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
					OrganizationKey: "org",
					TeamKey:         "team",
					AppIDKey:        "123",
					PrivateKeyKey:   "abc",
					ClientSecretKey: "def",
					ClientIDKey:     "456",
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
					OrganizationKey: "org",
					TeamKey:         "team",
					AppIDKey:        "abc",
					PrivateKeyKey:   "abc",
					ClientSecretKey: "def",
					ClientIDKey:     "456",
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
					OrganizationKey: "org",
					TeamKey:         "team",
					AppIDKey:        "123",
					PrivateKeyKey:   "abc",
					ClientSecretKey: "def",
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
					OrganizationKey: "org",
					TeamKey:         "team",
					AppIDKey:        "123",
					PrivateKeyKey:   "abc",
					ClientSecretKey: "def",
					ClientIDKey:     "456",
				},
			},
			expectError: true,
		},
		{
			name: "case 6: teams list without single team",
			credentials: provider.ProviderCredential{
				Name:  "name",
				Owner: "test",
				Credentials: map[string]string{
					OrganizationKey: "org",
					TeamsKey:        "team-a,team-b",
					AppIDKey:        "123",
					PrivateKeyKey:   "abc",
					ClientSecretKey: "def",
					ClientIDKey:     "456",
				},
			},
			log:         provider.GetTestLogger(),
			expectError: false,
		},
		{
			name: "case 7: neither team nor teams",
			credentials: provider.ProviderCredential{
				Name:  "name",
				Owner: "test",
				Credentials: map[string]string{
					OrganizationKey: "org",
					AppIDKey:        "123",
					PrivateKeyKey:   "abc",
					ClientSecretKey: "def",
					ClientIDKey:     "456",
				},
			},
			log:         provider.GetTestLogger(),
			expectError: true,
		},
		{
			name: "case 8: teams list of only separators",
			credentials: provider.ProviderCredential{
				Name:  "name",
				Owner: "test",
				Credentials: map[string]string{
					OrganizationKey: "org",
					TeamsKey:        " , ,",
					AppIDKey:        "123",
					PrivateKeyKey:   "abc",
					ClientSecretKey: "def",
					ClientIDKey:     "456",
				},
			},
			log:         provider.GetTestLogger(),
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

func TestPreferredEmailDomain(t *testing.T) {
	credentials := provider.ProviderCredential{
		Name:  "name",
		Owner: "test",
		Credentials: map[string]string{
			OrganizationKey:         "org",
			TeamKey:                 "team",
			AppIDKey:                "123",
			PrivateKeyKey:           "abc",
			ClientSecretKey:         "def",
			ClientIDKey:             "456",
			PreferredEmailDomainKey: "example.com",
		},
	}
	c, err := newGithubConfig(credentials, provider.GetTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	if c.PreferredEmailDomain != "example.com" {
		t.Fatalf("Expected preferred email domain example.com, got %q.", c.PreferredEmailDomain)
	}
}

func TestConnectorConfigRendersPreferredEmailDomain(t *testing.T) {
	testCases := []struct {
		name                 string
		preferredEmailDomain string
		expected             string
	}{
		{
			name:                 "set",
			preferredEmailDomain: "example.com",
			expected:             "preferredEmailDomain: example.com",
		},
		{
			name:                 "unset",
			preferredEmailDomain: "",
			expected:             "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cc := &connectorConfig{
				Config: githubconnector.Config{
					ClientID:      "id",
					ClientSecret:  "secret",
					Orgs:          []githubconnector.Org{{Name: "org", Teams: []string{"team"}}},
					RedirectURI:   "https://example.com/callback",
					TeamNameField: TeamNameFieldSlug,
				},
				PreferredEmailDomain: tc.preferredEmailDomain,
			}
			data, err := yaml.MarshalWithJsonAnnotations(cc)
			if err != nil {
				t.Fatal(err)
			}
			rendered := string(data)
			if tc.expected != "" && !strings.Contains(rendered, tc.expected) {
				t.Fatalf("Expected rendered config to contain %q, got:\n%s", tc.expected, rendered)
			}
			if tc.expected == "" && strings.Contains(rendered, "preferredEmailDomain") {
				t.Fatalf("Expected rendered config to omit preferredEmailDomain, got:\n%s", rendered)
			}
			if !strings.Contains(rendered, "clientID: id") {
				t.Fatalf("Expected embedded connector fields to render inline, got:\n%s", rendered)
			}
		})
	}
}

const (
	testTeamA = "team-a"
	testTeamB = "team-b"
)

func TestTeamsParsing(t *testing.T) {
	testCases := []struct {
		name          string
		credentials   map[string]string
		expectedTeams []string
		expectError   bool
	}{
		{
			name:          "single team key",
			credentials:   map[string]string{TeamKey: testTeamA},
			expectedTeams: []string{testTeamA},
		},
		{
			name:          "teams list",
			credentials:   map[string]string{TeamsKey: "team-a,team-b"},
			expectedTeams: []string{testTeamA, testTeamB},
		},
		{
			name:          "teams list with spaces and empty entries",
			credentials:   map[string]string{TeamsKey: " team-a , ,team-b, "},
			expectedTeams: []string{testTeamA, testTeamB},
		},
		{
			name:          "both keys with team included in teams",
			credentials:   map[string]string{TeamKey: testTeamA, TeamsKey: "team-a,team-b"},
			expectedTeams: []string{testTeamA, testTeamB},
		},
		{
			name:        "both keys with team missing from teams",
			credentials: map[string]string{TeamKey: testTeamA, TeamsKey: "team-b,team-c"},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			credentials := map[string]string{
				OrganizationKey: "org",
				AppIDKey:        "123",
				PrivateKeyKey:   "abc",
				ClientSecretKey: "def",
				ClientIDKey:     "456",
			}
			for k, v := range tc.credentials {
				credentials[k] = v
			}
			c, err := newGithubConfig(provider.ProviderCredential{
				Name:        "name",
				Owner:       "test",
				Credentials: credentials,
			}, provider.GetTestLogger())
			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected an error, got success with teams %v.", c.Teams)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(c.Teams, tc.expectedTeams) {
				t.Fatalf("Expected teams %v, got %v.", tc.expectedTeams, c.Teams)
			}
		})
	}
}
