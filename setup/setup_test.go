package setup

import (
	"giantswarm/dex-operator/pkg/idp/provider/github"
	"giantswarm/dex-operator/pkg/idp/provider/mockprovider"
	"strconv"
	"testing"
)

func TestCredentialSetup(t *testing.T) {
	testCases := []struct {
		name        string
		setup       Setup
		expectError bool
	}{
		{
			name: "case 0",
			setup: Setup{
				Installation:   "test",
				CredentialFile: "test-data/credentials",
				Provider:       IncludeAll,
				Action:         CleanAction,
				AppName:        "test",
			},
		},
		{
			name: "case 1",
			setup: Setup{
				Installation:   "test",
				CredentialFile: "test-data/credentials",
				Provider:       IncludeAll,
				Action:         CreateAction,
				AppName:        "test",
			},
		},
		{
			name: "case 2",
			setup: Setup{
				Installation:   "test",
				CredentialFile: "test-data/credentials",
				Provider:       mockprovider.ProviderName,
				Action:         CreateAction,
				AppName:        "test",
			},
		},
		{
			name: "case 3",
			setup: Setup{
				Installation:   "test",
				CredentialFile: "test-data/credentials",
				Provider:       mockprovider.ProviderName,
				Action:         CleanAction,
				AppName:        "test",
			},
		},
		{
			name: "case 4",
			setup: Setup{
				Installation:   "test",
				CredentialFile: "test-data/credentials",
				Provider:       github.ProviderName,
				Action:         CleanAction,
				AppName:        "test",
			},
		},
		{
			name: "case 5",
			setup: Setup{
				Installation:   "test",
				CredentialFile: "test-data/credentials",
				Provider:       github.ProviderName,
				Action:         "somethingelse",
				AppName:        "test",
			},
			expectError: true,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			err := CredentialSetup(tc.setup)
			if err != nil && !tc.expectError {
				t.Fatal(err)
			}
			if err == nil && tc.expectError {
				t.Fatalf("Expected an error, got success.")
			}
		})
	}
}
