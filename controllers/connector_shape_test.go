package controllers

import (
	"context"
	"encoding/json"
	"testing"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/dex-operator/pkg/key"
)

const (
	managedConnectorID = "giantswarm-mock"
	userConnectorMap   = `
oidc:
  giantswarm:
    connectors:
      giantswarm-gazelle-oidc:
        connectorType: oidc
`
	userConnectorMapWithManagedID = `
oidc:
  giantswarm:
    connectors:
      giantswarm-mock:
        connectorConfig: "issuer: https://elsewhere"
`
)

// The shape the operator wrote under oidc.giantswarm.connectors: "map",
// "list", or "" when the secret is gone.
func writtenShape(t *testing.T, c client.Client) string {
	t.Helper()
	secret := &corev1.Secret{}
	err := c.Get(context.Background(), types.NamespacedName{Name: key.GetDexConfigName(siblingName), Namespace: siblingNamespace}, secret)
	if apierrors.IsNotFound(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	var written struct {
		Oidc struct {
			Giantswarm struct {
				Connectors json.RawMessage `json:"connectors"`
			} `json:"giantswarm"`
		} `json:"oidc"`
	}
	if err := json.Unmarshal(secret.Data["default"], &written); err != nil {
		t.Fatal(err)
	}
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(written.Oidc.Giantswarm.Connectors, &asMap); err == nil {
		if _, ok := asMap[managedConnectorID]; !ok {
			t.Fatalf("map lacks the managed connector %q: %s", managedConnectorID, written.Oidc.Giantswarm.Connectors)
		}
		return "map"
	}
	var asList []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(written.Oidc.Giantswarm.Connectors, &asList); err != nil {
		t.Fatalf("connectors are neither a map nor a list: %s", written.Oidc.Giantswarm.Connectors)
	}
	if len(asList) != 1 || asList[0].ID != managedConnectorID {
		t.Fatalf("list lacks the managed connector %q: %s", managedConnectorID, written.Oidc.Giantswarm.Connectors)
	}
	return "list"
}

func userValues(values string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "dex-app-user-values", Namespace: siblingNamespace},
		Data:       map[string]string{key.ValuesConfigMapKey: values},
	}
}

func TestHelmReleaseConnectorShape(t *testing.T) {
	testCases := []struct {
		name         string
		chartVersion string
		userValues   string
		want         string
	}{
		{name: "map-capable chart gets a map", chartVersion: "3.5.0", want: "map"},
		{name: "list-only chart gets a list", chartVersion: "3.4.0", want: "list"},
		{name: "not installed yet gets a list", want: "list"},
		{name: "user map of other ids merges with the map", chartVersion: "3.5.0", userValues: userConnectorMap, want: "map"},
		{name: "a clash keeps the secret up to date", chartVersion: "3.5.0", userValues: userConnectorMapWithManagedID, want: "map"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := siblingScheme(t)
			hr := siblingHelmRelease(false)
			if tc.chartVersion != "" {
				hr.Status.History = helmv2.Snapshots{{Version: 1, ChartVersion: tc.chartVersion}}
			}
			objects := []client.Object{hr, siblingSecret(t, s, hr)}
			if tc.userValues != "" {
				hr.Spec.ValuesFrom = append(hr.Spec.ValuesFrom, helmv2.ValuesReference{Kind: "ConfigMap", Name: "dex-app-user-values", ValuesKey: key.ValuesConfigMapKey})
				objects = append(objects, userValues(tc.userValues))
			}
			c := fake.NewClientBuilder().WithScheme(s).WithObjects(objects...).WithInterceptorFuncs(helmReleaseWriteGuard(t)).Build()

			reconcileSibling(t, c, s, hr)

			if got := writtenShape(t, c); got != tc.want {
				t.Fatalf("written shape = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAppConnectorShape(t *testing.T) {
	testCases := []struct {
		name            string
		specVersion     string
		deployedVersion string
		userValues      string
		want            string
	}{
		{name: "map-capable chart gets a map", specVersion: "3.5.0", deployedVersion: "3.5.0", want: "map"},
		{name: "list-only chart gets a list", specVersion: "2.2.2", deployedVersion: "2.2.2", want: "list"},
		{name: "downgrade gets a list", specVersion: "3.4.0", deployedVersion: "3.5.0", want: "list"},
		{name: "user map of other ids merges with the map", specVersion: "3.5.0", deployedVersion: "3.5.0", userValues: userConnectorMap, want: "map"},
		{name: "user map on a list-only chart backs off", specVersion: "2.2.2", deployedVersion: "2.2.2", userValues: userConnectorMap},
		{name: "user map entry with a managed id backs off", specVersion: "3.5.0", deployedVersion: "3.5.0", userValues: userConnectorMapWithManagedID},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := siblingScheme(t)
			app := siblingApp(false)
			app.Spec.Version = tc.specVersion
			app.Status.Version = tc.deployedVersion
			objects := []client.Object{app, siblingSecret(t, s, app)}
			if tc.userValues != "" {
				app.Spec.UserConfig.ConfigMap = v1alpha1.AppSpecUserConfigConfigMap{Name: "dex-app-user-values", Namespace: siblingNamespace}
				objects = append(objects, userValues(tc.userValues))
			}
			c := fake.NewClientBuilder().WithScheme(s).WithObjects(objects...).Build()

			reconcileSibling(t, c, s, app)

			if got := writtenShape(t, c); got != tc.want {
				t.Fatalf("written shape = %q, want %q", got, tc.want)
			}
		})
	}
}
