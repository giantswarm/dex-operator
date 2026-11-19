package dextarget

import (
	"context"
	"errors"
	"reflect"
	"testing"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/giantswarm/dex-operator/pkg/key"
)

func TestDexTarget(t *testing.T) {
	// Placeholder test to ensure package is included in coverage
}

const (
	testValuesKey        = "values.yaml"
	testUserValues       = "user-values"
	kindSecret           = "Secret"
	testNamespace        = "example"
	giantswarmConnectors = `
oidc:
  giantswarm:
    connectors:
    - id: giantswarm-github
`
	customerConnectors = `
oidc:
  customer:
    connectors:
    - id: customer-entra
`
)

func TestUserConnectors(t *testing.T) {
	testCases := []struct {
		name        string
		values      string
		expected    []UserConnectors
		expectError bool
	}{
		{
			name:     "giantswarm connectors only",
			values:   giantswarmConnectors,
			expected: []UserConnectors{list(key.OwnerGiantswarm)},
		},
		{
			name:     "customer connectors only",
			values:   customerConnectors,
			expected: []UserConnectors{list(key.OwnerCustomer)},
		},
		{
			name: "both owners",
			values: `
oidc:
  customer:
    connectors:
    - id: customer-entra
  giantswarm:
    connectors:
    - id: giantswarm-github
`,
			expected: []UserConnectors{list(key.OwnerGiantswarm), list(key.OwnerCustomer)},
		},
		{
			name: "empty connectors list",
			values: `
oidc:
  giantswarm:
    connectors: []
`,
			expected: []UserConnectors{list(key.OwnerGiantswarm)},
		},
		{
			name: "null connectors",
			values: `
oidc:
  customer:
    connectors: null
`,
			expected: []UserConnectors{list(key.OwnerCustomer)},
		},
		{
			name: "giantswarm map sets entries by id",
			values: `oidc:
  giantswarm:
    connectors:
      federation:
        connectorType: oidc
`,
			expected: []UserConnectors{entries(key.OwnerGiantswarm, "federation")},
		},
		{
			name: "connectors set to a map",
			values: `
oidc:
  customer:
    connectors: {}
  giantswarm:
    connectors:
    - id: giantswarm-github
`,
			expected: []UserConnectors{list(key.OwnerGiantswarm), entries(key.OwnerCustomer)},
		},
		{
			name: "duplicate top-level key",
			values: `
ingress:
  enabled: true
ingress:
  enabled: false
oidc:
  giantswarm:
    connectors:
    - id: giantswarm-github
`,
			expected: []UserConnectors{list(key.OwnerGiantswarm)},
		},
		{
			name:   "no oidc key",
			values: "ingress:\n  enabled: true\n",
		},
		{
			name: "connectors key elsewhere",
			values: `
connectors:
- id: top-level
oidc:
  giantswarm:
    enabled: true
other:
  connectors:
  - id: nested
`,
		},
		{
			name: "other keys under oidc",
			values: `
oidc:
  expiry: 24h
  staticClients:
  - id: client
  giantswarm:
    connectors:
    - id: giantswarm-github
`,
			expected: []UserConnectors{list(key.OwnerGiantswarm)},
		},
		{
			name: "other owner with an unexpected type",
			values: `
oidc:
  customer: false
  giantswarm:
    connectors:
    - id: giantswarm-github
`,
			expected: []UserConnectors{list(key.OwnerGiantswarm)},
		},
		{
			name: "oidc with an unexpected type",
			values: `
oidc: ""
`,
		},
		{
			name:        "invalid yaml",
			values:      "oidc: [giantswarm",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			owners, err := userConnectors(tc.values)
			if err != nil && !tc.expectError {
				t.Fatal(err)
			}
			if err == nil && tc.expectError {
				t.Fatalf("Expected an error, got success.")
			}
			if !reflect.DeepEqual(owners, tc.expected) {
				t.Fatalf("Expected %v, got %v", tc.expected, owners)
			}
		})
	}
}

func TestHelmReleaseUserConnectors(t *testing.T) {
	testCases := []struct {
		name        string
		valuesFrom  []helmv2.ValuesReference
		objects     []client.Object
		interceptor interceptor.Funcs
		expected    []UserConnectors
		expectError bool
	}{
		{
			name:       "customer connectors in a secret",
			valuesFrom: []helmv2.ValuesReference{{Kind: kindSecret, Name: testUserValues}},
			objects:    []client.Object{getSecret(testUserValues, testValuesKey, customerConnectors)},
			expected:   []UserConnectors{list(key.OwnerCustomer)},
		},
		{
			name:       "own dex config secret is skipped",
			valuesFrom: []helmv2.ValuesReference{{Kind: kindSecret, Name: key.GetDexConfigName("test"), ValuesKey: "default"}},
			objects:    []client.Object{getSecret(key.GetDexConfigName("test"), "default", giantswarmConnectors)},
		},
		{
			name:       "dex config secret of another target is not skipped",
			valuesFrom: []helmv2.ValuesReference{{Kind: kindSecret, Name: key.GetDexConfigName("other"), ValuesKey: "default"}},
			objects:    []client.Object{getSecret(key.GetDexConfigName("other"), "default", giantswarmConnectors)},
			expected:   []UserConnectors{list(key.OwnerGiantswarm)},
		},
		{
			name:       "missing secret is skipped",
			valuesFrom: []helmv2.ValuesReference{{Kind: kindSecret, Name: testUserValues}},
		},
		{
			name:       "other get error is returned",
			valuesFrom: []helmv2.ValuesReference{{Kind: kindSecret, Name: testUserValues}},
			objects:    []client.Object{getSecret(testUserValues, testValuesKey, customerConnectors)},
			interceptor: interceptor.Funcs{
				Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
					return errors.New("connection refused")
				},
			},
			expectError: true,
		},
		{
			name:       "custom values key",
			valuesFrom: []helmv2.ValuesReference{{Kind: "ConfigMap", Name: testUserValues, ValuesKey: "custom"}},
			objects:    []client.Object{getConfigMap(testUserValues, map[string]string{testValuesKey: customerConnectors, "custom": giantswarmConnectors})},
			expected:   []UserConnectors{list(key.OwnerGiantswarm)},
		},
		{
			name: "invalid yaml is skipped",
			valuesFrom: []helmv2.ValuesReference{
				{Kind: kindSecret, Name: "broken", ValuesKey: "values"},
				{Kind: kindSecret, Name: testUserValues},
			},
			objects: []client.Object{
				getSecret("broken", "values", "oidc: [giantswarm"),
				getSecret(testUserValues, testValuesKey, customerConnectors),
			},
			expected: []UserConnectors{list(key.OwnerCustomer)},
		},
		{
			name:       "target path at a map entry sets that entry",
			valuesFrom: []helmv2.ValuesReference{{Kind: kindSecret, Name: "connector-config", TargetPath: "oidc.giantswarm.connectors.federation.connectorConfig"}},
			expected:   []UserConnectors{entries(key.OwnerGiantswarm, "federation")},
		},
		{
			name:       "giantswarm connector map in user config",
			valuesFrom: []helmv2.ValuesReference{{Kind: kindConfigMap, Name: testUserValues}},
			objects:    []client.Object{getConfigMap(testUserValues, map[string]string{testValuesKey: "oidc:\n  giantswarm:\n    connectors:\n      federation:\n        connectorType: oidc\n"})},
			expected:   []UserConnectors{entries(key.OwnerGiantswarm, "federation")},
		},
		{
			name:       "customer target path at a map entry sets that entry",
			valuesFrom: []helmv2.ValuesReference{{Kind: kindSecret, Name: "connector-config", TargetPath: "oidc.customer.connectors.federation.connectorConfig"}},
			expected:   []UserConnectors{entries(key.OwnerCustomer, "federation")},
		},
		{
			name:       "customer connector map in user config",
			valuesFrom: []helmv2.ValuesReference{{Kind: kindConfigMap, Name: testUserValues}},
			objects:    []client.Object{getConfigMap(testUserValues, map[string]string{testValuesKey: "oidc:\n  customer:\n    connectors:\n      federation:\n        connectorType: oidc\n"})},
			expected:   []UserConnectors{entries(key.OwnerCustomer, "federation")},
		},
		{
			name:       "target path under owner connectors",
			valuesFrom: []helmv2.ValuesReference{{Kind: kindSecret, Name: "connector-id", TargetPath: "oidc.giantswarm.connectors[0].id"}},
			expected:   []UserConnectors{list(key.OwnerGiantswarm)},
		},
		{
			name:       "other target path is skipped",
			valuesFrom: []helmv2.ValuesReference{{Kind: kindSecret, Name: testUserValues, TargetPath: "some.other.path"}},
			objects:    []client.Object{getSecret(testUserValues, testValuesKey, customerConnectors)},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithObjects(tc.objects...).WithInterceptorFuncs(tc.interceptor).Build()
			target := NewHelmReleaseTarget(&helmv2.HelmRelease{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace},
				Spec:       helmv2.HelmReleaseSpec{ValuesFrom: tc.valuesFrom},
			})

			owners, err := target.UserConnectors(context.Background(), c)
			if err != nil && !tc.expectError {
				t.Fatal(err)
			}
			if err == nil && tc.expectError {
				t.Fatalf("Expected an error, got success.")
			}
			if !reflect.DeepEqual(owners, tc.expected) {
				t.Fatalf("Expected %v, got %v", tc.expected, owners)
			}
		})
	}
}

func TestAppUserConnectors(t *testing.T) {
	testCases := []struct {
		name     string
		values   string
		expected []UserConnectors
	}{
		{
			name:     "customer connectors in user config",
			values:   customerConnectors,
			expected: []UserConnectors{list(key.OwnerCustomer)},
		},
		{
			name:   "no connectors in user config",
			values: "ingress:\n  enabled: true\n",
		},
		{
			name:   "invalid yaml is skipped",
			values: "oidc: [giantswarm",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithObjects(getConfigMap(testUserValues, map[string]string{key.ValuesConfigMapKey: tc.values})).Build()
			app := &v1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace},
			}
			app.Spec.UserConfig.ConfigMap = v1alpha1.AppSpecUserConfigConfigMap{Name: testUserValues, Namespace: testNamespace}

			owners, err := NewAppTarget(app).UserConnectors(context.Background(), c)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(owners, tc.expected) {
				t.Fatalf("Expected %v, got %v", tc.expected, owners)
			}
		})
	}
}

func list(owner string) UserConnectors {
	return UserConnectors{Owner: owner}
}

func entries(owner string, ids ...string) UserConnectors {
	return UserConnectors{Owner: owner, Map: true, IDs: ids}
}

func getSecret(name, valuesKey, values string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Data:       map[string][]byte{valuesKey: []byte(values)},
	}
}

func getConfigMap(name string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Data:       data,
	}
}

// dex-operator never writes spec.valuesFrom on a HelmRelease, with or without
// the Flux labels: the manifest has to reference the secret.
func TestHelmReleaseTargetNeverWritesSecretConfig(t *testing.T) {
	for name, labels := range map[string]map[string]string{
		"Flux labels":    {"kustomize.toolkit.fluxcd.io/name": "collections", "kustomize.toolkit.fluxcd.io/namespace": "flux-giantswarm"},
		"no Flux labels": nil,
	} {
		t.Run(name, func(t *testing.T) {
			hr := &helmv2.HelmRelease{ObjectMeta: metav1.ObjectMeta{Name: "dex-app", Namespace: "giantswarm", Labels: labels}}
			c := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
				Update: func(context.Context, client.WithWatch, client.Object, ...client.UpdateOption) error {
					t.Error("unexpected Update")
					return errors.New("update not allowed")
				},
			}).Build()
			target := NewHelmReleaseTarget(hr)

			if target.ManagesSecretConfig() {
				t.Error("expected ManagesSecretConfig to be false")
			}
			if err := target.AddSecretConfig("dex-app-default-dex-config", "giantswarm"); err != nil {
				t.Fatal(err)
			}
			if changed, err := target.AttachSecretConfig(context.Background(), c); changed || err != nil {
				t.Errorf("expected no write, got changed=%v err=%v", changed, err)
			}
			if len(hr.Spec.ValuesFrom) != 0 {
				t.Errorf("expected spec.valuesFrom untouched, got %v", hr.Spec.ValuesFrom)
			}
		})
	}
}

func TestUserConnectorsMapIDs(t *testing.T) {
	found, err := userConnectors("oidc:\n  customer:\n    connectors:\n      customer-oidc: {}\n      customer-alba-oidc: {}\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []UserConnectors{entries(key.OwnerCustomer, "customer-alba-oidc", "customer-oidc")}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("Expected %v, got %v", want, found)
	}
}

func TestTargetPathConnectors(t *testing.T) {
	testCases := []struct {
		targetPath string
		expected   UserConnectors
		found      bool
	}{
		{"oidc.giantswarm.connectors", list(key.OwnerGiantswarm), true},
		{"oidc.customer.connectors[1].connectorConfig", list(key.OwnerCustomer), true},
		{"oidc.customer.connectors.customer-alba-oidc", entries(key.OwnerCustomer, "customer-alba-oidc"), true},
		{"oidc.giantswarm.connectors.giantswarm-github.connectorConfig", entries(key.OwnerGiantswarm, "giantswarm-github"), true},
		{"oidc.giantswarm.connectorsextra", UserConnectors{}, false},
		{"oidc.extraStaticClients", UserConnectors{}, false},
	}
	for _, tc := range testCases {
		t.Run(tc.targetPath, func(t *testing.T) {
			got, found := targetPathConnectors(tc.targetPath)
			if found != tc.found || !reflect.DeepEqual(got, tc.expected) {
				t.Fatalf("Expected %v %v, got %v %v", tc.expected, tc.found, got, found)
			}
		})
	}
}

func TestHelmReleaseAcceptsConnectorMaps(t *testing.T) {
	testCases := []struct {
		name     string
		history  helmv2.Snapshots
		expected bool
	}{
		{name: "not installed yet"},
		{name: "map-capable chart", history: helmv2.Snapshots{{ChartVersion: "3.5.0"}}, expected: true},
		{name: "later chart with build metadata", history: helmv2.Snapshots{{ChartVersion: "3.6.1+4734369129eb"}}, expected: true},
		{name: "list-only chart", history: helmv2.Snapshots{{ChartVersion: "3.4.0"}}},
		{name: "pre-release of the first map-capable chart", history: helmv2.Snapshots{{ChartVersion: "3.5.0-rc.8"}}},
		{name: "latest release decides", history: helmv2.Snapshots{{Version: 1, ChartVersion: "3.5.0"}, {Version: 2, ChartVersion: "3.4.0"}}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hr := &helmv2.HelmRelease{}
			hr.Status.History = tc.history
			if got := NewHelmReleaseTarget(hr).AcceptsConnectorMaps(); got != tc.expected {
				t.Fatalf("Expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestAppAcceptsConnectorMaps(t *testing.T) {
	testCases := []struct {
		name            string
		specVersion     string
		deployedVersion string
		expected        bool
	}{
		{name: "map-capable chart deployed", specVersion: "3.5.0", deployedVersion: "3.5.0", expected: true},
		{name: "list-only chart deployed", specVersion: "2.2.2", deployedVersion: "2.2.2"},
		{name: "not deployed yet", specVersion: "3.5.0"},
		{name: "upgrade not deployed yet", specVersion: "3.5.0", deployedVersion: "3.4.0"},
		{name: "downgrade requested", specVersion: "3.4.0", deployedVersion: "3.5.0"},
		{name: "unparsable version", specVersion: "main", deployedVersion: "main"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := &v1alpha1.App{}
			app.Spec.Version = tc.specVersion
			app.Status.Version = tc.deployedVersion
			if got := NewAppTarget(app).AcceptsConnectorMaps(); got != tc.expected {
				t.Fatalf("Expected %v, got %v", tc.expected, got)
			}
		})
	}
}
