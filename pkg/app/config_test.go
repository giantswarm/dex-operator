package app

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/giantswarm/dex-operator/pkg/key"
	"github.com/giantswarm/dex-operator/pkg/tests"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetAppConfig(t *testing.T) {
	testCases := []struct {
		name                           string
		managementClusterName          string
		managementClusterBaseDomain    string
		managementClusterIssuerAddress string
		app                            *v1alpha1.App
		clusterValuesConfigMap         *corev1.ConfigMap
		expectedAppConfig              Config
	}{
		{
			name:                           "case 0: Get issuer URL from cluster config values",
			managementClusterName:          "testcluster",
			managementClusterBaseDomain:    "base.domain.io",
			managementClusterIssuerAddress: "issuer.cluster.base.domain.io",
			app:                            tests.GetExampleApp(),
			clusterValuesConfigMap:         getClusterValuesConfigMap("baseDomain: wc.cluster.domain.io"),
			expectedAppConfig: Config{
				Name:                 "testcluster-example-test",
				IssuerURI:            "https://dex.wc.cluster.domain.io",
				RedirectURI:          "https://dex.wc.cluster.domain.io/callback",
				IdentifierURI:        "https://dex.giantswarm.io/testcluster-example-test",
				SecretValidityMonths: key.SecretValidityMonths,
			},
		},
		{
			name:                           "case 1: Get issuer URL from management cluster issuer URL property",
			managementClusterName:          "testcluster",
			managementClusterBaseDomain:    "base.domain.io",
			managementClusterIssuerAddress: "issuer.cluster.domain.io",
			app:                            tests.GetExampleApp(),
			expectedAppConfig: Config{
				Name:                 "testcluster-example-test",
				IssuerURI:            "https://issuer.cluster.domain.io",
				RedirectURI:          "https://issuer.cluster.domain.io/callback",
				IdentifierURI:        "https://dex.giantswarm.io/testcluster-example-test",
				SecretValidityMonths: key.SecretValidityMonths,
			},
		},
		{
			name:                        "case 2: Get issuer URL from management cluster base domain",
			managementClusterName:       "testcluster",
			managementClusterBaseDomain: "base.domain.io",
			app:                         tests.GetExampleApp(),
			expectedAppConfig: Config{
				Name:                 "testcluster-example-test",
				IssuerURI:            "https://dex.g8s.base.domain.io",
				RedirectURI:          "https://dex.g8s.base.domain.io/callback",
				IdentifierURI:        "https://dex.giantswarm.io/testcluster-example-test",
				SecretValidityMonths: key.SecretValidityMonths,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			ctx := context.Background()

			fakeClientBuilder := fake.NewClientBuilder()
			if tc.clusterValuesConfigMap != nil {
				tc.app.Spec = v1alpha1.AppSpec{
					Config: v1alpha1.AppSpecConfig{
						ConfigMap: v1alpha1.AppSpecConfigConfigMap{
							Name:      tc.clusterValuesConfigMap.Name,
							Namespace: tc.clusterValuesConfigMap.Namespace,
						},
					},
				}
				fakeClientBuilder.WithObjects(tc.clusterValuesConfigMap)
			}

			appConfig, err := GetConfig(ctx, tc.app, fakeClientBuilder.Build(), ManagementClusterProps{
				Name:          tc.managementClusterName,
				BaseDomain:    tc.managementClusterBaseDomain,
				IssuerAddress: tc.managementClusterIssuerAddress,
			})
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(appConfig, tc.expectedAppConfig) {
				t.Fatalf("Expacted %v, got %v", tc.expectedAppConfig, appConfig)
			}
		})
	}
}

func getClusterValuesConfigMap(clusterValues string) *corev1.ConfigMap {
	name := fmt.Sprintf("test-%s", key.ClusterValuesConfigmapSuffix)
	return tests.GetClusterValuesConfigMap(name, "example", clusterValues)
}
