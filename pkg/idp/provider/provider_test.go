package provider

import (
	"giantswarm/dex-operator/pkg/key"
	"reflect"
	"strconv"
	"testing"
)

func TestReadCredentials(t *testing.T) {
	testCases := []struct {
		name        string
		location    string
		credentials []ProviderCredential
		expectError error
	}{
		{
			name:     "case 0",
			location: "test-data/credentials",
			credentials: []ProviderCredential{
				{Name: "mock", Owner: key.OwnerGiantswarm, Credentials: map[string]string{"hello": "hi"}},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			credentials, err := ReadCredentials(tc.location)
			if err != tc.expectError {
				if err != nil {
					t.Fatal(err)
				} else {
					t.Fatalf("Expected error %v", tc.expectError)
				}
			}
			if !reflect.DeepEqual(credentials, tc.credentials) {
				t.Fatalf("Expected %v to be equal to %v", credentials, tc.credentials)
			}
		})
	}
}
