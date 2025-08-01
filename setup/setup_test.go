package setup

import (
	"os"
	"path"
	"reflect"
	"strconv"
	"testing"

	"github.com/giantswarm/dex-operator/pkg/idp/provider/github"
	"github.com/giantswarm/dex-operator/pkg/idp/provider/mockprovider"
	"github.com/giantswarm/dex-operator/pkg/idp/provider/simpleprovider"
)

func TestSetupConfig(t *testing.T) {
	testCases := []struct {
		name        string
		setup       SetupConfig
		expectError bool
	}{
		{
			name: "case 0",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials",
				Provider:              IncludeAll,
				Action:                CleanAction,
			},
		},
		{
			name: "case 1",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials_notexist",
				Provider:              IncludeAll,
				Action:                UpdateAction,
			},
			expectError: true,
		},
		{
			name: "case 2",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials_repeat",
				Provider:              IncludeAll,
				Action:                UpdateAction,
			},
			expectError: true,
		},
		{
			name: "case 3",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials_customer",
				Provider:              IncludeAll,
				Action:                UpdateAction,
			},
		},
		{
			name: "case 4",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials",
				Provider:              mockprovider.ProviderName,
				Action:                CleanAction,
			},
		},
		{
			name: "case 5",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials",
				Provider:              github.ProviderName,
				Action:                CleanAction,
			},
		},
		{
			name: "case 6",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials_both",
				Provider:              github.ProviderName,
				Action:                CleanAction,
			},
		},
		{
			name: "case 7",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials_empty",
				Provider:              github.ProviderName,
				Action:                CleanAction,
			},
		},
		{
			name: "case 8",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials_b64",
				Provider:              IncludeAll,
				Action:                UpdateAction,
				Base64Vars:            true,
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := New(tc.setup)
			if err != nil && !tc.expectError {
				t.Fatal(err)
			}
			if err == nil && tc.expectError {
				t.Fatalf("Expected an error, got success.")
			}
		})
	}
}

func TestRun(t *testing.T) {
	testCases := []struct {
		name         string
		setup        SetupConfig
		expectConfig Config
		expectError  bool
	}{
		{
			name: "case 0",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials",
				Provider:              IncludeAll,
				Action:                UpdateAction,
			},
			expectConfig: getDefaultConfig(),
		},
		{
			name: "case 1",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials_customer",
				Provider:              IncludeAll,
				Action:                UpdateAction,
			},
			expectConfig: Config{
				Oidc: Oidc{
					Customer: getOldprovider(),
				},
			},
		},
		{
			name: "case 2",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials",
				Provider:              mockprovider.ProviderName,
				Action:                UpdateAction,
			},
			expectConfig: getDefaultConfig(),
		},
		{
			name: "case 3",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials",
				Provider:              github.ProviderName,
				Action:                UpdateAction,
			},
			expectConfig: Config{
				Oidc: Oidc{
					Giantswarm: getOldprovider(),
				},
			},
		},
		{
			name: "case 4",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials_both",
				Provider:              mockprovider.ProviderName,
				Action:                UpdateAction,
			},
			expectConfig: Config{
				Oidc: Oidc{
					Customer:   getOldprovider(),
					Giantswarm: getNewprovider(),
				},
			},
		},
		{
			name: "case 5",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials_empty",
				Provider:              mockprovider.ProviderName,
				Action:                UpdateAction,
			},
			expectConfig: Config{},
		},
		{
			name: "case 6",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials",
				Provider:              IncludeAll,
				Action:                CleanAction,
			},
			expectConfig: Config{
				Oidc: Oidc{
					Giantswarm: getOldprovider(),
				},
			},
		},
		{
			name: "case 7",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials",
				Provider:              IncludeAll,
				Action:                "something",
			},
			expectError: true,
			expectConfig: Config{
				Oidc: Oidc{
					Giantswarm: getOldprovider(),
				},
			},
		},
		{
			name: "case 8",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials_both",
				Provider:              IncludeAll,
				Action:                CreateAction,
			},
			expectConfig: Config{
				Oidc: Oidc{
					Giantswarm: getNewprovider(),
				},
			},
		},
		{
			name: "case 9",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials",
				Provider:              IncludeAll,
				Action:                DeleteAction,
			},
			expectConfig: Config{
				Oidc: Oidc{
					Giantswarm: getOldprovider(),
				},
			},
		},
		{
			name: "case 10",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials_b64",
				Provider:              IncludeAll,
				Action:                UpdateAction,
				Base64Vars:            true,
			},
			expectConfig: getDefaultConfig(),
		},
		{
			name: "case 11",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials_simple",
				Provider:              IncludeAll,
				Action:                UpdateAction,
			},
			expectConfig: Config{
				Oidc: Oidc{
					Giantswarm: OidcOwner{
						[]OidcOwnerProvider{
							{
								Name:        mockprovider.ProviderName,
								Credentials: getNewCredential(),
							},
							{
								Name:        simpleprovider.ProviderName,
								Credentials: getOldCredential(),
							},
						},
					},
				},
			},
		},
		{
			name: "case 12",
			setup: SetupConfig{
				ManagementClusterName: "test",
				CredentialFile:        "test-data/credentials_simple",
				Provider:              IncludeAll,
				Action:                CreateAction,
			},
			expectConfig: getDefaultConfig(),
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			dir, err := os.MkdirTemp("", "dex-operator-test")
			if err != nil {
				t.Fatal(err)
			}
			tc.setup.OutputFile = path.Join(dir, tc.name)
			setup, err := New(tc.setup)
			if err != nil {
				t.Fatal(err)
			}
			err = setup.Run()
			if err != nil && !tc.expectError {
				t.Fatal(err)
			}
			if err == nil && tc.expectError {
				t.Fatalf("Expected an error, got success.")
			}
			if !reflect.DeepEqual(setup.config, tc.expectConfig) {
				t.Fatalf("Expected Configs to match.")
			}
		})
	}
}

func getDefaultConfig() Config {
	return Config{
		Oidc: Oidc{
			Giantswarm: getNewprovider(),
		},
	}
}

func getOldprovider() OidcOwner {
	return OidcOwner{
		[]OidcOwnerProvider{
			{
				Name:        mockprovider.ProviderName,
				Credentials: getOldCredential(),
			},
		},
	}
}

func getNewprovider() OidcOwner {
	return OidcOwner{
		[]OidcOwnerProvider{
			{
				Name:        mockprovider.ProviderName,
				Credentials: getNewCredential(),
			},
		},
	}
}

func getOldCredential() string {
	return `client-id: xyz
client-secret: test
`
}

func getNewCredential() string {
	return `cert: |-
  -----BEGIN MOCK CERT-----
  mock
  cert
  hello
  -----END MOCK CERT-----
client-id: abc
client-secret: test
`
}
