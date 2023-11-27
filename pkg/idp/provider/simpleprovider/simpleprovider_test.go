package simpleprovider

import (
	"context"
	"reflect"
	"strconv"
	"testing"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/idp/provider"

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
			name:        "case 0 - no log",
			expectError: true,
		},
		{
			name:        "case 1 - invalid credentials",
			credentials: provider.GetTestCredential(),
			log:         provider.GetTestLogger(),
			expectError: true,
		},
		{
			name: "case 2 - valid credentials",
			credentials: provider.ProviderCredential{
				Name:  "name",
				Owner: "test",
				Credentials: map[string]string{
					connectorTypeKey:   "type",
					connectorConfigKey: "config",
				},
			},
			log:         provider.GetTestLogger(),
			expectError: false,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, err := newSimpleConfig(tc.credentials, tc.log)
			if err != nil && !tc.expectError {
				t.Fatal(err)
			}
			if err == nil && tc.expectError {
				t.Fatalf("Expected an error, got success.")
			}
		})
	}
}

func TestCreateApp(t *testing.T) {
	testCases := []struct {
		name            string
		connectorType   string
		connectorConfig string
		description     string
		appConfig       provider.AppConfig

		expectedConnector dex.Connector
		expectError       bool
	}{
		{
			name:            "case 0 - microsoft connector - uri given",
			connectorType:   "microsoft",
			connectorConfig: "clientID: 123\nclientSecret: abc\ntenant: 123\nredirectURI: hi.io",
			appConfig:       provider.GetTestConfig(),

			expectedConnector: dex.Connector{
				Type:   "microsoft",
				ID:     "test-simple-microsoft",
				Name:   "Simple Provider for test",
				Config: "clientID: 123\nclientSecret: abc\ntenant: 123\nredirectURI: hello.io",
			},
		},
		{
			name:            "case 1 - microsoft connector - no uri given",
			connectorType:   "microsoft",
			connectorConfig: "clientID: 123\nclientSecret: abc\ntenant: 123",
			appConfig:       provider.GetTestConfig(),

			expectedConnector: dex.Connector{
				Type:   "microsoft",
				ID:     "test-simple-microsoft",
				Name:   "Simple Provider for test",
				Config: "clientID: 123\nclientSecret: abc\ntenant: 123\nredirectURI: hello.io",
			},
		},
		{
			name:            "case 2 - microsoft connector - uri given at the beginning",
			connectorType:   "microsoft",
			connectorConfig: "redirectURI: hi.x.y.io/a/b-c/d\nclientID: 123\nclientSecret: abc\ntenant: 123",
			appConfig:       provider.GetTestConfig(),

			expectedConnector: dex.Connector{
				Type:   "microsoft",
				ID:     "test-simple-microsoft",
				Name:   "Simple Provider for test",
				Config: "redirectURI: hello.io\nclientID: 123\nclientSecret: abc\ntenant: 123",
			},
		},
		{
			name:            "case 3 - microsoft connector - uri given in the middle",
			connectorType:   "microsoft",
			connectorConfig: "clientID: 123\nredirectURI: http://xyz.hi.io\nclientSecret: abc\ntenant: 123",
			appConfig:       provider.GetTestConfig(),

			expectedConnector: dex.Connector{
				Type:   "microsoft",
				ID:     "test-simple-microsoft",
				Name:   "Simple Provider for test",
				Config: "clientID: 123\nredirectURI: hello.io\nclientSecret: abc\ntenant: 123",
			},
		},
		{
			name:            "case 4 - github connector",
			connectorType:   "github",
			connectorConfig: "clientID: 123\nclientSecret: abc\norgs:\n- name: giantswarm\n- name: giantswarm2\nredirectURI: hi.io",
			appConfig:       provider.GetTestConfig(),

			expectedConnector: dex.Connector{
				Type:   "github",
				ID:     "test-simple-github",
				Name:   "Simple Provider for test",
				Config: "clientID: 123\nclientSecret: abc\norgs:\n- name: giantswarm\n- name: giantswarm2\nredirectURI: hello.io",
			},
		},
		{
			name:            "case 5 - oidc connector",
			connectorType:   "oidc",
			connectorConfig: "issuer: https://dex.example.com\nclientID: 123\nclientSecret: abc\nredirectURI: hi.io",
			appConfig:       provider.GetTestConfig(),

			expectedConnector: dex.Connector{
				Type:   "oidc",
				ID:     "test-simple-oidc",
				Name:   "Simple Provider for test",
				Config: "issuer: https://dex.example.com\nclientID: 123\nclientSecret: abc\nredirectURI: hello.io",
			},
		},
		{
			name:            "case 6 - ldap connector",
			connectorType:   "ldap",
			connectorConfig: "host: ldap.example.com\nport: 389\ninsecureNoSSL: true\nbindDN: cn=admin,dc=example,dc=com\nbindPW: password\nuserSearch:\n  baseDN: ou=users,dc=example,dc=com\n  filter: (uid=%s)\n  username: uid\n  idAttr: uid\n  emailAttr: mail\n  nameAttr: cn\n  preferredUsernameAttr: uid\n  groups:\n    query: (|(memberUid=%s)(uniqueMember=%s)(member=%s))\n    userAttr: dn\n    groupAttr: cn\n    nameAttr: cn\n    adminAttr: cn\n    idAttr: dn\n    emailAttr: mail\n    userScope: sub\n    groupScope: sub\n    groupSearch:\n      baseDN: ou=groups,dc=example,dc=com\n      filter: (objectClass=groupOfNames)\n      username: cn\n      idAttr: cn\n      emailAttr: mail\n      nameAttr: cn\n      preferredUsernameAttr: cn\n      userAttr: member\n      groupAttr: member\n      adminAttr: member\n      userScope: base\n      groupScope: base\n      groupSearch:\n        baseDN: ou=groups,dc=example,dc=com\n        filter: (objectClass=groupOfNames)\n        username: cn\n        idAttr: cn\n        emailAttr: mail\n        nameAttr: cn\n        preferredUsernameAttr: cn\n        userAttr: member\n        groupAttr: member\n        adminAttr: member\n        userScope: base\n        groupScope: base",
			appConfig:       provider.GetTestConfig(),

			expectedConnector: dex.Connector{
				Type:   "ldap",
				ID:     "test-simple-ldap",
				Name:   "Simple Provider for test",
				Config: "host: ldap.example.com\nport: 389\ninsecureNoSSL: true\nbindDN: cn=admin,dc=example,dc=com\nbindPW: password\nuserSearch:\n  baseDN: ou=users,dc=example,dc=com\n  filter: (uid=%s)\n  username: uid\n  idAttr: uid\n  emailAttr: mail\n  nameAttr: cn\n  preferredUsernameAttr: uid\n  groups:\n    query: (|(memberUid=%s)(uniqueMember=%s)(member=%s))\n    userAttr: dn\n    groupAttr: cn\n    nameAttr: cn\n    adminAttr: cn\n    idAttr: dn\n    emailAttr: mail\n    userScope: sub\n    groupScope: sub\n    groupSearch:\n      baseDN: ou=groups,dc=example,dc=com\n      filter: (objectClass=groupOfNames)\n      username: cn\n      idAttr: cn\n      emailAttr: mail\n      nameAttr: cn\n      preferredUsernameAttr: cn\n      userAttr: member\n      groupAttr: member\n      adminAttr: member\n      userScope: base\n      groupScope: base\n      groupSearch:\n        baseDN: ou=groups,dc=example,dc=com\n        filter: (objectClass=groupOfNames)\n        username: cn\n        idAttr: cn\n        emailAttr: mail\n        nameAttr: cn\n        preferredUsernameAttr: cn\n        userAttr: member\n        groupAttr: member\n        adminAttr: member\n        userScope: base\n        groupScope: base",
			},
		},
		{
			name:            "case 7 - oidc connector - invalid config",
			connectorType:   "oidc",
			connectorConfig: "invalid",
			appConfig:       provider.GetTestConfig(),

			expectError: true,
		},
		{
			name:            "case 8 - github connector - invalid config",
			connectorType:   "github",
			connectorConfig: "clientID: 123\nclientSecret: abc\norgs:\n- giantswarm\n- giantswarm2\nredirectURI: hi.io",
			appConfig:       provider.GetTestConfig(),

			expectError: true,
		},
		{
			name:            "case 9 - microsoft connector - invalid config",
			connectorType:   "microsoft",
			connectorConfig: "clientID: 123\n  clientSecret: abc\ntenant: 123\nredirectURI: hi.io",
			appConfig:       provider.GetTestConfig(),

			expectError: true,
		},
		{
			name:            "case 10 - invalid connector type",
			connectorType:   "invalid",
			connectorConfig: "clientID: 123\nclientSecret: abc\norgs:\n- giantswarm\n- giantswarm2\nredirectURI: hi.io",
			appConfig:       provider.GetTestConfig(),

			expectError: true,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			credential := provider.ProviderCredential{
				Name:  "name",
				Owner: "test",
				Credentials: map[string]string{
					connectorTypeKey:   tc.connectorType,
					connectorConfigKey: tc.connectorConfig,
				},
			}
			simple, err := New(credential, provider.GetTestLogger())
			if err != nil {
				t.Fatal(err)
			}
			app, err := simple.CreateOrUpdateApp(tc.appConfig, context.Background(), dex.Connector{})
			if err != nil && !tc.expectError {
				t.Fatal(err)
			}
			if err == nil && tc.expectError {
				t.Fatalf("Expected an error, got success.")
			}
			if !reflect.DeepEqual(app.Connector, tc.expectedConnector) {
				t.Fatalf("Expected %v to be equal to %v", app.Connector.Config, tc.expectedConnector.Config)
			}
		})
	}
}
