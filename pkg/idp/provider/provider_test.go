package provider

import (
	"reflect"
	"strconv"
	"testing"
)

func TestReadCredentials(t *testing.T) {
	testCases := []struct {
		name        string
		location    string
		credentials []ProviderCredential
	}{
		{
			name:     "case 0",
			location: "test-data/credentials",
			credentials: []ProviderCredential{
				{Name: "mock", Owner: "giantswarm", Credentials: map[string]string{"hello": "hi"}},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			credentials, err := ReadCredentials(tc.location)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(credentials, tc.credentials) {
				t.Fatalf("Expected %v to be equal to %v", credentials, tc.credentials)
			}
		})
	}
}
