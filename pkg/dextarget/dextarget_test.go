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

func TestConnectorOwners(t *testing.T) {
	testCases := []struct {
		name           string
		values         string
		expectedOwners []string
		expectError    bool
	}{
		{
			name:           "giantswarm connectors only",
			values:         giantswarmConnectors,
			expectedOwners: []string{key.OwnerGiantswarm},
		},
		{
			name:           "customer connectors only",
			values:         customerConnectors,
			expectedOwners: []string{key.OwnerCustomer},
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
			expectedOwners: []string{key.OwnerGiantswarm, key.OwnerCustomer},
		},
		{
			name: "empty connectors list",
			values: `
oidc:
  giantswarm:
    connectors: []
`,
			expectedOwners: []string{key.OwnerGiantswarm},
		},
		{
			name: "null connectors",
			values: `
oidc:
  customer:
    connectors: null
`,
			expectedOwners: []string{key.OwnerCustomer},
		},
		{
			name: "giantswarm maps merge with managed connectors",
			values: `oidc:
  giantswarm:
    connectors:
      federation:
        connectorType: oidc
`,
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
			expectedOwners: []string{key.OwnerGiantswarm, key.OwnerCustomer},
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
			expectedOwners: []string{key.OwnerGiantswarm},
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
			expectedOwners: []string{key.OwnerGiantswarm},
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
			expectedOwners: []string{key.OwnerGiantswarm},
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
			owners, err := connectorOwners(tc.values)
			if err != nil && !tc.expectError {
				t.Fatal(err)
			}
			if err == nil && tc.expectError {
				t.Fatalf("Expected an error, got success.")
			}
			if !reflect.DeepEqual(owners, tc.expectedOwners) {
				t.Fatalf("Expected %v, got %v", tc.expectedOwners, owners)
			}
		})
	}
}

func TestHelmReleaseUserConnectorOwners(t *testing.T) {
	testCases := []struct {
		name           string
		valuesFrom     []helmv2.ValuesReference
		objects        []client.Object
		interceptor    interceptor.Funcs
		expectedOwners []string
		expectError    bool
	}{
		{
			name:           "customer connectors in a secret",
			valuesFrom:     []helmv2.ValuesReference{{Kind: kindSecret, Name: testUserValues}},
			objects:        []client.Object{getSecret(testUserValues, "values.yaml", customerConnectors)},
			expectedOwners: []string{key.OwnerCustomer},
		},
		{
			name:       "own dex config secret is skipped",
			valuesFrom: []helmv2.ValuesReference{{Kind: kindSecret, Name: key.GetDexConfigName("test"), ValuesKey: "default"}},
			objects:    []client.Object{getSecret(key.GetDexConfigName("test"), "default", giantswarmConnectors)},
		},
		{
			name:           "dex config secret of another target is not skipped",
			valuesFrom:     []helmv2.ValuesReference{{Kind: kindSecret, Name: key.GetDexConfigName("other"), ValuesKey: "default"}},
			objects:        []client.Object{getSecret(key.GetDexConfigName("other"), "default", giantswarmConnectors)},
			expectedOwners: []string{key.OwnerGiantswarm},
		},
		{
			name:       "missing secret is skipped",
			valuesFrom: []helmv2.ValuesReference{{Kind: kindSecret, Name: testUserValues}},
		},
		{
			name:       "other get error is returned",
			valuesFrom: []helmv2.ValuesReference{{Kind: kindSecret, Name: testUserValues}},
			objects:    []client.Object{getSecret(testUserValues, "values.yaml", customerConnectors)},
			interceptor: interceptor.Funcs{
				Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
					return errors.New("connection refused")
				},
			},
			expectError: true,
		},
		{
			name:           "custom values key",
			valuesFrom:     []helmv2.ValuesReference{{Kind: "ConfigMap", Name: testUserValues, ValuesKey: "custom"}},
			objects:        []client.Object{getConfigMap(testUserValues, map[string]string{"values.yaml": customerConnectors, "custom": giantswarmConnectors})},
			expectedOwners: []string{key.OwnerGiantswarm},
		},
		{
			name: "invalid yaml is skipped",
			valuesFrom: []helmv2.ValuesReference{
				{Kind: kindSecret, Name: "broken", ValuesKey: "values"},
				{Kind: kindSecret, Name: testUserValues},
			},
			objects: []client.Object{
				getSecret("broken", "values", "oidc: [giantswarm"),
				getSecret(testUserValues, "values.yaml", customerConnectors),
			},
			expectedOwners: []string{key.OwnerCustomer},
		},
		{
			name:       "target path at a map entry can coexist",
			valuesFrom: []helmv2.ValuesReference{{Kind: kindSecret, Name: "connector-config", TargetPath: "oidc.giantswarm.connectors.federation.connectorConfig"}},
		},
		{
			name:       "giantswarm connector map in user config can coexist",
			valuesFrom: []helmv2.ValuesReference{{Kind: kindConfigMap, Name: testUserValues}},
			objects:    []client.Object{getConfigMap(testUserValues, map[string]string{"values.yaml": "oidc:\n  giantswarm:\n    connectors:\n      federation:\n        connectorType: oidc\n"})},
		},
		{
			name:           "target path under owner connectors",
			valuesFrom:     []helmv2.ValuesReference{{Kind: kindSecret, Name: "connector-id", TargetPath: "oidc.giantswarm.connectors[0].id"}},
			expectedOwners: []string{key.OwnerGiantswarm},
		},
		{
			name:       "other target path is skipped",
			valuesFrom: []helmv2.ValuesReference{{Kind: kindSecret, Name: testUserValues, TargetPath: "some.other.path"}},
			objects:    []client.Object{getSecret(testUserValues, "values.yaml", customerConnectors)},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithObjects(tc.objects...).WithInterceptorFuncs(tc.interceptor).Build()
			target := NewHelmReleaseTarget(&helmv2.HelmRelease{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace},
				Spec:       helmv2.HelmReleaseSpec{ValuesFrom: tc.valuesFrom},
			})

			owners, err := target.UserConnectorOwners(context.Background(), c)
			if err != nil && !tc.expectError {
				t.Fatal(err)
			}
			if err == nil && tc.expectError {
				t.Fatalf("Expected an error, got success.")
			}
			if !reflect.DeepEqual(owners, tc.expectedOwners) {
				t.Fatalf("Expected %v, got %v", tc.expectedOwners, owners)
			}
		})
	}
}

func TestAppUserConnectorOwners(t *testing.T) {
	testCases := []struct {
		name           string
		values         string
		expectedOwners []string
	}{
		{
			name:           "customer connectors in user config",
			values:         customerConnectors,
			expectedOwners: []string{key.OwnerCustomer},
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

			owners, err := NewAppTarget(app).UserConnectorOwners(context.Background(), c)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(owners, tc.expectedOwners) {
				t.Fatalf("Expected %v, got %v", tc.expectedOwners, owners)
			}
		})
	}
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
