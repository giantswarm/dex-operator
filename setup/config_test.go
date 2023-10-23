package setup

import (
	"reflect"
	"strconv"
	"testing"
)

func TestGetCredentialsFromBase64Vars(t *testing.T) {
	testCases := []struct {
		name        string
		file        []byte
		expected    Config
		expectedErr bool
	}{
		{
			name:     "case 0",
			file:     getValidBase64VarsFile(),
			expected: getValidResultConfig(),
		},
		{
			name:        "case 1",
			file:        getInvalidBase64VarsFile(),
			expectedErr: true,
		},
		{
			name:     "case 2",
			file:     getValidBase64VarsFileEmptyLines(),
			expected: getValidResultConfig(),
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			result, err := getCredentialsFromBase64Vars(tc.file)
			if err != nil && !tc.expectedErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && tc.expectedErr {
				t.Fatalf("expected error but got none")
			}
			if !reflect.DeepEqual(result, tc.expected) {
				t.Fatalf("expected result %#v got %#v", tc.expected, result)
			}
		})
	}
}

func TestGetBase64VarsFromConfig(t *testing.T) {
	testCases := []struct {
		name     string
		config   Config
		expected []byte
	}{
		{
			name:     "case 0",
			config:   getValidResultConfig(),
			expected: getValidBase64VarsFile(),
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			result := getBase64VarsFromConfig(tc.config)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Fatalf("expected result %#v got %#v", string(tc.expected), string(result))
			}
		})
	}
}

func getValidBase64VarsFile() []byte {
	file := "dex_operator_ad_credential_b64='dGVzdA=='\n" +
		"dex_operator_github_credential_b64='ZXhhbXBsZQ=='"
	return []byte(file)
}

func getValidBase64VarsFileEmptyLines() []byte {
	file := "\n\ndex_operator_ad_credential_b64='dGVzdA=='\n\n" +
		"dex_operator_github_credential_b64='ZXhhbXBsZQ=='\n\n"
	return []byte(file)
}

func getInvalidBase64VarsFile() []byte {
	file := "hello hello\n" +
		"some other stuff\n" +
		"1234\n"
	return []byte(file)
}

func getValidResultConfig() Config {
	return Config{
		Oidc: Oidc{
			Giantswarm: OidcOwner{
				Providers: []OidcOwnerProvider{
					{
						Name:        "ad",
						Credentials: "test",
					},
					{
						Name:        "github",
						Credentials: "example",
					},
				},
			},
		},
	}
}
